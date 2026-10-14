---
page_title: "Terraform Hoop Provider 0.0.22 Upgrade Guide"
subcategory: ""
---

Version 0.0.22 adds the resources that manage a sidecar fleet from Terraform, and extends `hoop_datamasking_rules` so a masking rule can reach sidecars. Existing configurations need no change.

## New resources

| Resource | Gateway API | Needs Enterprise license |
|---|---|---|
| `hoop_sidecar` | `/api/sidecars` | yes |
| `hoop_sidecar_service_account` | `/api/sidecar-service-accounts` | yes |
| `hoop_guardrail` | `/api/guardrails` | no |
| `hoop_ai_session_analyzer_rule` | `/api/ai/session-analyzer/rules` | no |
| `data.hoop_sidecar` | `GET /api/sidecars/{name}` | yes |

A gateway without an Enterprise license answers the licensed routes with `403` and this message:

```
Error: status=403, payload={"message": "this resource requires a valid enterprise license, ..."}
```

The provider keeps the resource in state when it sees that answer. Add the license under **Settings > License** and run `terraform apply` again.

## Two ways to bring a sidecar under Terraform

**Register it with a token.** `hoop_sidecar` creates the sidecar and receives its token once. The token lives in state as the sensitive `token` attribute; hand it to the sidecar as `HOOP_SIDECAR_TOKEN`.

**Let the sidecar identify itself.** `hoop_sidecar_service_account` maps a Kubernetes or Google identity to a sidecar name. The control plane creates the sidecar on the pod's first handshake, so there is no `hoop_sidecar` resource for it. `data.hoop_sidecar` reads it by name, and rule resources bind to `data.hoop_sidecar.x.id`.

## Rules reach sidecars from the rule, not from the sidecar

`hoop_guardrail`, `hoop_datamasking_rules` and `hoop_ai_session_analyzer_rule` carry two attributes:

- `sidecar_spec`: the rule in the sidecar's own vocabulary, as a JSON object.
- `sidecar_targets`: the sidecar listeners that enforce it, each `{ sidecar_id, listener_name }`. An empty `listener_name` means every listener of that sidecar.

`hoop_sidecar` has no list of rules. The control plane folds every bound rule into the document it serves on each handshake, and the gateway only accepts the binding through the rule's own API.

## How the binding attributes read absence

The gateway treats a request without `sidecar_targets` as "keep the bindings", and one with `sidecar_targets: []` as "unbind everywhere". The provider follows that, with one rule: **an attribute you never declared is not Terraform's**.

| Your configuration | What the provider sends | What state shows |
|---|---|---|
| never had `sidecar_targets` | nothing | null, whatever the control plane holds |
| has `sidecar_targets = [...]` | the list | the list |
| had `sidecar_targets`, now removed | `[]` | null |

`sidecar_spec`, `reviewers_groups`, `pending_ttl_sec`, `approval_ttl_sec` and `hoop_sidecar.slack_channels` follow the same table. A rule bound from the control plane UI keeps its bindings under a Terraform configuration that does not mention them. To make Terraform the owner of an empty binding set, declare `sidecar_targets = []`.

## `hoop_datamasking_rules`

Two optional attributes, `sidecar_spec` and `sidecar_targets`, as above. A configuration from 0.0.21 plans no change.

## `hoop_sidecar.configuration` is a JSON string

Write it with `jsonencode(...)`, or `jsonencode(yamldecode(file("sidecar.yaml")))` to keep the file you already deploy. Two things the gateway does to the document show up in a plan if you fight them:

- It drops every zero value. Leave `""`, `0`, `false`, `[]` and `{}` out of your document.
- It refuses a `load_from_disk` key inside the document. Use the `load_from_disk` attribute; the provider routes the change through the separate call the gateway requires.
