# CLAUDE.md

Guidance for working in `terraform-provider-hoop`, the Terraform provider for the Hoop Gateway API. It is built on the [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework) (protocol v6). It does not use SDKv2. Registry address: `hoophq/hoop`.

## Commands

```sh
make build        # go build ./...
make test         # unit tests: go test -v -cover -timeout=10m -parallel=10 ./...
make lint         # golangci-lint (config in .golangci.yml)
make fmt          # gofmt -s -w
make generate     # terraform fmt on examples/, then tfplugindocs regenerates docs/
make install      # go install into $GOPATH/bin

go test ./internal/provider -run TestSidecarResourceIgnoresZeroValues -v   # one test
./scripts/terraform -chdir=dev/main plan             # run a local build against a live gateway
```

The tests start a real Terraform binary. Keep `terraform` (CI pins 1.9.x) on `PATH`. If it is missing, `terraform-plugin-testing` downloads one, and parallel tests race on that download ("text file busy").

Before you call a change done, run `make fmt lint test generate`, then check `git status` for doc changes that need to be committed.

## Layout

```
main.go                      provider server entrypoint (providerserver, protocol v6)
internal/hoop/               Hoop Gateway HTTP client. No Terraform imports.
  client.go, auth.go         the only files allowed to build requests or set headers
  errors.go                  APIError, IsNotFound, IsLicenseRefusal, operator hints
  <domain>.go                one file per API area: API structs + Client methods
internal/provider/           Terraform Plugin Framework layer
  provider.go                provider schema, Configure, Resources()/DataSources() registry
  <name>_resource.go         one resource per file
  <name>_data_source.go      one data source per file
  validators.go              shared schema validators
  sidecar_common.go          shared helpers for sidecar binding (sidecar_targets, JSON specs)
  *_test.go                  unit tests against an in-process fake gateway
examples/                    HCL picked up by tfplugindocs (see Documentation)
docs/                        GENERATED registry docs, except docs/templates/
docs/templates/guides/       hand-written guides (source for docs/guides/)
tools/                       go:generate entrypoints (tfplugindocs, copywrite)
dev/                         local sandbox: gitignored, never commit
```

## Architecture rules

### Two layers, one direction

- `internal/hoop` speaks HTTP and JSON. It knows nothing about Terraform.
- `internal/provider` maps Terraform schema and state to `hoop` types. Each resource model has `fromApi`/`toApi`-style helpers, and the CRUD methods stay thin.
- Every request goes through `(*Client).newRequest` and `(*Client).send` / `sendDiscard`. `TestRequestChokepoint` fails the build if any other file in `internal/hoop` builds a request, sets a header, or touches the HTTP client. A new endpoint is a method that calls those two functions.
- Non-2xx responses become `*hoop.APIError`. Use `hoop.IsNotFound(err)` to recognize 404. Never compare status codes in the provider layer.

### Resources

- **One resource manages one API object.** It matches the gateway's names and shapes unless that clearly hurts usability. Higher-level composition belongs in user modules, not the provider. Follow [HashiCorp's provider design principles](https://developer.hashicorp.com/terraform/plugin/best-practices/hashicorp-provider-design-principles).
- **Skeleton.** Copy the structure of an existing resource. `feature_flag_resource.go` is the smallest complete example.
  - Interface assertions: `var _ resource.Resource = &xResource{}`, plus each `ResourceWith*` it implements.
  - A `NewXResource()` constructor, registered in `provider.go` → `Resources()`.
  - A model struct with `tfsdk:"..."` tags that uses `types.*` values. Plain Go types cannot represent null or unknown.
  - `Metadata` sets `req.ProviderTypeName + "_<name>"`.
  - `Configure` asserts `*hoop.Client` and returns early when `ProviderData == nil`.
- **Importable.** Every resource implements `ImportState`. Use `resource.ImportStatePassthroughID` for single-key IDs. Composite IDs are parsed and validated, and a bad format is an error. Add `examples/resources/<type>/import.sh`.
- **Read handles drift.** On `hoop.IsNotFound`, call `resp.State.RemoveResource(ctx)` and return, so the next plan recreates the object. A license refusal (`IsLicenseRefusal`) is not "gone".
- **Delete is idempotent.** Treat a 404 on delete as success.
- **Diagnostics.** Append with `resp.Diagnostics.Append(...)` and check `HasError()` before continuing.
  - Summaries are short Title Case phrases with no values, such as `"Error Reading Feature Flag"`.
  - Details are specific and actionable. Include the object's name and the error.
  - Use `AddAttributeError` / `AddAttributeWarning` with a `path` when one attribute is at fault.
  - Use warnings for things the operator should know but that are not fatal, such as unbinding or an owner switch.

### Schema and state consistency

These rules prevent "Provider produced inconsistent result" errors and perpetual diffs, which are the most common provider bugs.

- **Naming.** Attribute names use lowercase `snake_case`.
  - Singular nouns for single values. Plural nouns for lists, sets, and maps.
  - Booleans are positive: `true` turns something on (`enabled`, not `disabled`).
  - Resource and data source names are `hoop_<noun>`.
- **Required / Optional / Computed.**
  - `Required`: the gateway needs the value.
  - `Optional`: the user may omit it.
  - `Computed`: the gateway owns it.
  - `Optional + Computed`: the gateway fills in a default when the user omits it.
  - Never return a value that differs from what the user configured. Normalize on the way in, or use a semantic-equality type such as `jsontypes.Normalized` for JSON.
- **Null stays null.** An optional attribute the user left out must read back as null, not `""`, `[]`, or `{}`. Use the helpers in `sidecar_common.go`, such as `fromApiString` and `fromApiStringList`, which take the prior value for this reason.
- **Declared vs. not declared.** Attributes that the UI may also manage, such as `sidecar_targets` or rule specs, follow one convention:
  - Not declared means the attribute is left out of the request, so changes made outside Terraform survive.
  - Declared means Terraform owns the value and sends it on every write.
  - Removing a declared value sends the empty value that clears it.
  - New attributes of this kind reuse the `toApiOwned*` / `fromApiOwned*` helpers. Do not invent a new convention.
- **Order.** Lists keep the order the user wrote. If the API reorders, realign to prior state the way `reorderTargets` does, or use a set.
- **Plan modifiers.**
  - `RequiresReplace()` goes on attributes the API cannot update in place, such as names used as identifiers.
  - `UseStateForUnknown()` goes on stable computed attributes, such as `id` and created timestamps, so plans do not show "(known after apply)" noise.
  - Use `Default` (static defaults) instead of writing defaults in CRUD code.
- **Catch errors at plan time.** Field checks go in `Validators`; reuse `validators.go` and `terraform-plugin-framework-validators`. Cross-field checks go in `ValidateConfig`. Checks that need the API, such as the license preflight or bound-listener guards, go in `ModifyPlan`.
  - Skip validation when a value is unknown. It will be validated once it is known.
  - In `ModifyPlan`, `req.State.Raw.IsNull()` means create and `req.Plan.Raw.IsNull()` means destroy.
- **Secrets.**
  - Mark credentials and tokens `Sensitive: true`.
  - Never log them. Log with `tflog` using structured fields. Only `client.RedactedAPIURL()` may be logged for the URL.
  - For new secret inputs that do not need to persist in state, consider a write-only `_wo` attribute.
- **Descriptions are the docs.** Every schema, attribute, and nested attribute gets a `Description` written for practitioners. Say what the attribute does, what values it accepts, and what happens when it is omitted or removed. Use Markdown backticks for values.

### Compatibility and versioning

- The provider follows SemVer. Released state must keep working: renaming or removing an attribute or resource, or changing its type or meaning, is a breaking change.
- Deprecate before removing. Set `DeprecationMessage` and point to the replacement, as `connection_resource.go` does for redact types. Keep the old path working for at least one release.
- If the stored state shape changes, bump the schema `Version` and implement `ResourceWithUpgradeState`.
- A breaking or behavior-changing release gets a guide in `docs/templates/guides/upgrade-guide-<version>.md`.
- Releases are tags `v*`. GoReleaser builds and signs them (`scripts/publish-release.sh`, `.github/workflows/release.yml`). Do not tag unless asked.

## Testing

- Tests are **unit tests run through the real Terraform CLI** (`resource.UnitTest` / `IsUnitTest: true`). The provider runs against an in-process fake gateway, injected as the `hoop.HttpClient` passed to `New(version, httpClient)`. No live gateway or `TF_ACC` is needed. CI runs `make test` on every PR.
- The fake gateway building blocks live in `provider_test.go` and `sidecar_fake_test.go`.
  - `clientFunc` turns a function into an HTTP client.
  - `httpTestOk` / `httpTestErr` build responses.
  - `requestRecorder` and `checkLastBody` assert the exact JSON sent.
  - The fakes model real gateway semantics: a missing key versus `[]` versus `null`, and 404s.
  - Assert on request bodies when the wire format matters.
- Every resource should cover:
  1. Create, then check the attributes (`resource.TestCheckResourceAttr`).
  2. Update in place, and a step that forces replacement where it applies.
  3. Import, with `ImportState: true` and `ImportStateVerify: true` (plus `ImportStateVerifyIdentifierAttribute` for non-`id` keys).
  4. Bad configuration rejected at plan, with `ExpectError` and a regexp.
  5. Out-of-band deletion, where the fake returns 404 on read and the plan recreates the object.
  6. No diff after apply. The framework enforces this; do not hide a diff with `ExpectNonEmptyPlan` unless the diff is intended.
- Use `t.Parallel()`. Each test gets its own fake gateway and must share no state.
- Pure helpers, such as plan logic and JSON normalization, get plain table-driven Go tests (see `sidecar_plan_test.go`).

## Documentation

Docs are generated by `tfplugindocs` (`make generate`) from:

- schema `Description`s
- `examples/provider/provider.tf`
- `examples/resources/<type>/resource.tf` and `import.sh`
- `examples/data-sources/<type>/data-source.tf`
- `docs/templates/guides/*.md`

Rules:

- **Never hand-edit `docs/resources/`, `docs/data-sources/`, `docs/index.md` or `docs/guides/`.** Change the source and regenerate. Commit the regenerated docs with the code change.
- Every new resource needs an `examples/resources/<type>/` directory, and the example must be valid HCL that `terraform fmt` accepts.
- Add new resources to the "Supported Resources" list in `readme.md`.

## Code style

- Every `.go`, `.tf`, `.sh`, and `.yml` file starts with the `# Copyright (c) HashiCorp, Inc.` header. Use `//` in Go.
- Comments explain *why*: gateway quirks, ordering constraints, past bugs. They are full sentences ending with a period, because the `godot` linter checks this. Keep the density of comments you see around your change.
- Wrap API errors as `fmt.Errorf("failed <doing x>, reason=%v", err)`, the format used in `internal/hoop`.
- `forcetypeassert` is enabled, so use the `v, ok :=` form.
- Commit messages are one imperative sentence that says what changed and why it matters to the operator, for example: `Require listener_name on sidecar_targets: the gateway binds per listener and refuses a nameless target`.

## Safety

- Never commit credentials. `.api-key`, `dev/`, `*.tfstate`, and `.terraformrc` hold local secrets and state.
- Sidecar and sidecar service account resources need an Enterprise license on the organization. Keep the plan-time preflight (`refuseWithoutEnterpriseLicense`) in any new resource behind that license.
