---
page_title: "Terraform Hoop Provider 0.0.21 Upgrade Guide"
subcategory: ""
---

Version 0.0.21 fixes authentication against Hoop Gateway when using an **organization API key** — the keys prefixed with `hpk_` that are created under **Settings > API Keys**.

Up to version 0.0.20 the provider always sent the credential in the legacy `Api-Key` header. The gateway only accepts that header for the value configured in its own `API_KEY` environment variable, and it never falls back to any other authentication method once the header is present. As a result every request made with an `hpk_` key failed with:

```
Error: status=401, payload={"message": "access denied"}
```

## What changed

The provider now selects the credential header from the format of `api_key`:

| `api_key` value                | Header sent                          |
|--------------------------------|--------------------------------------|
| Starts with `hpk_`             | `Authorization: Bearer <api_key>`    |
| Anything else                  | `Api-Key: <api_key>` (unchanged)     |

The two headers are mutually exclusive — only one is ever sent.

Legacy keys are unaffected: the gateway compares its `API_KEY` environment variable byte for byte, so any value that is not an organization API key keeps using the exact same header it uses today. **No configuration change is required to upgrade.**

## Grant the admin group to your API key

This is the one thing you may have to act on.

Legacy `API_KEY` credentials were always treated as organization administrators by the gateway. Organization API keys are not: they only carry the groups assigned to them when the key was created.

An `hpk_` key without the `admin` group now authenticates successfully but is rejected on admin-only endpoints:

```
Error: status=403, payload={"message": "access denied"}
```

If you hit this, edit the key under **Settings > API Keys** and add the `admin` group, or create a new key with it.

## Overriding the detection

An optional `auth_scheme` attribute is available as an escape hatch. It accepts `auto` (the default), `bearer` and `api_key`:

```hcl
provider "hoop" {
  api_url = "http://localhost:8009/api"
  api_key = "<hpk_...>"

  # Only needed if the automatic detection is wrong for your gateway, for
  # example when the gateway's legacy API_KEY value happens to start with hpk_.
  auth_scheme = "api_key"
}
```

Leave it unset unless you have a reason to override the default.

## Upgrading

```sh
terraform init -upgrade
terraform plan
```

No state migration and no resource changes are involved.
