---
page_title: "Managing a sidecar fleet with Terraform"
subcategory: ""
---

A hoop sidecar is a relay in front of a database or an API. It fetches its configuration from the control plane on every heartbeat, enforces the rules the control plane binds to it, and reports back what it is running. This guide takes a fleet from zero to enforced rules with Terraform alone, and shows what the plan catches before anything reaches a pod.

Every resource here except the rules needs an Enterprise license on the organization. The plan tells you when it is missing.

## Choose how sidecars authenticate

| | Token | Platform identity |
|---|---|---|
| Resource | `hoop_sidecar` | `hoop_sidecar_service_account` |
| Who creates the sidecar | Terraform | the control plane, on the pod's first handshake |
| Secret in state | yes, the `token` attribute | none |
| Per-sidecar work | one resource each | none; the name comes from a template |
| Fits | a few long-lived sidecars, VMs, compose | Kubernetes and GCE fleets, autoscaled replicas |

Both end in the same place: a sidecar id that rule resources bind to through `sidecar_targets`.

## Platform identity, on GKE

One mapping covers every pod that runs as the `hoop-sidecar` service account, in any namespace of the cluster. The subject pattern's `*` becomes the namespace, and `name_template` turns it into the sidecar name.

```terraform
resource "hoop_sidecar_service_account" "gke_eu" {
  name            = "gke-eu"
  issuer          = "https://container.googleapis.com/v1/projects/my-project/locations/europe-west1/clusters/eu"
  audience        = "https://hoop.example.com"
  claim           = "sub"
  subject_pattern = "system:serviceaccount:*:hoop-sidecar"
  name_template   = "gke-eu-{1}"
}
```

`audience` is the control plane URL the sidecar uses. The issuer is the cluster's OIDC issuer; `kubectl get --raw /.well-known/openid-configuration | jq .issuer` prints it.

Deploy the chart with `controlPlane.identity`. The chart mounts a projected token with that audience and the relay presents it on every request.

```terraform
resource "helm_release" "sidecar" {
  name       = "hoopsidecar"
  namespace  = "ws-123"
  repository = "oci://ghcr.io/hoophq/helm-charts"
  chart      = "hoopsidecar-chart"

  values = [yamlencode({
    controlPlane = {
      url      = "https://hoop.example.com"
      identity = "kubernetes"
    }
    serviceAccount = { create = true, name = "hoop-sidecar" }
    laneServices = {
      internal = { enabled = true, ports = [{ name = "postgres", port = 15432 }] }
    }
  })]

  depends_on = [hoop_sidecar_service_account.gke_eu]
}
```

The pod's first handshake creates the sidecar `gke-eu-ws-123`. It needs a document with listeners, and there are two places it can come from:

- **The chart's `config` block.** A sidecar that finds an empty control plane seeds it with its own file, once. After that the control plane owns the document, and the file's listeners are ignored. This is the smallest change for a fleet that already runs from a ConfigMap.
- **A `hoop_sidecar` resource.** Import the sidecar by name after the first handshake and declare `configuration`; Terraform owns the document from then on. `terraform import hoop_sidecar.ws_123 gke-eu-ws-123`.

Either way, rule resources need the sidecar id. Read it by name:

```terraform
data "hoop_sidecar" "ws_123" {
  name       = "gke-eu-ws-123"
  depends_on = [helm_release.sidecar]
}
```

A `data` block fails while the sidecar does not exist yet, so the first `terraform apply` after the deploy may need a retry while the pod comes up. Once the sidecar is there, `data.hoop_sidecar.ws_123.id` is stable across pod restarts, rollouts and replicas: they all land on the same sidecar.

## Token, anywhere

```terraform
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    admin     = { listen = "0.0.0.0:19000" }
    audit     = { file = "-" }
    listeners = [{
      name     = "appdb"
      protocol = "postgres"
      listen   = "0.0.0.0:15432"
      upstream = "appdb.default.svc.cluster.local:5432"
    }]
  })
}

resource "helm_release" "payments" {
  name       = "payments-sidecar"
  repository = "oci://ghcr.io/hoophq/helm-charts"
  chart      = "hoopsidecar-chart"

  values = [yamlencode({
    controlPlane = { url = "https://hoop.example.com" }
  })]

  set_sensitive {
    name  = "controlPlane.token"
    value = hoop_sidecar.payments.token
  }
}
```

The gateway shows the token once. It lives in Terraform state from then on, so treat the state as you treat the token.

`configuration` is the sidecar's own config file as JSON. `jsonencode(yamldecode(file("sidecar.yaml")))` keeps the YAML you already have. Omit zero values or leave them in; the provider compares the way the gateway stores the document, so `""`, `0`, `false` and empty blocks do not show up as changes.

## Bind rules to listeners

Rules are their own resources, and each one names the listeners that enforce it. `sidecar_spec` is the rule in the sidecar's vocabulary; `sidecar_targets` is where it goes.

```terraform
resource "hoop_guardrail" "no_destructive" {
  name = "no-destructive"

  sidecar_spec = jsonencode({
    rules = [{
      name       = "no-destructive"
      type       = "operation"
      operations = ["delete", "drop", "truncate"]
      message    = "destructive statements are not permitted on appdb"
    }]
  })

  sidecar_targets = [
    { sidecar_id = hoop_sidecar.payments.id, listener_name = "appdb" },
    { sidecar_id = data.hoop_sidecar.ws_123.id, listener_name = "appdb" },
  ]
}

resource "hoop_datamasking_rules" "emails" {
  name                   = "mask-emails"
  description            = "Redact email addresses on the way back"
  score_threshold        = 0.5
  connection_ids         = []
  custom_entity_types    = []
  supported_entity_types = []

  sidecar_spec = jsonencode({
    rules = [{ name = "emails", entities = ["EMAIL_ADDRESS"], strategy = "redact" }]
  })

  sidecar_targets = [{ sidecar_id = hoop_sidecar.payments.id, listener_name = "appdb" }]
}
```

Every entry names a listener: the listener's protocol decides which rules it can run, so the gateway refuses a target without one. The control plane folds bound rules into the document it serves, so the sidecar picks them up on its next heartbeat and nothing restarts.

Three things about `sidecar_targets`:

- Leave it out and Terraform leaves the bindings alone, including ones made in the control plane UI.
- Declare it and Terraform owns the set. Removing the attribute later sends an empty set, and the plan warns with the number of listeners the rule leaves.
- Guardrails append to the listener's own guardrails block. Masking rules replace the listener's own mask block, and several bound to one listener concatenate into it; what two rules naming the same entity do is the sidecar's call, the control plane does not check it. Two analyzer rules on one listener is refused.

## Hold statements for approval

An analyzer rule can hold a statement until someone releases it. Who may do that is an access request rule with `access_type = "sidecar"`, named from the analyzer block:

```terraform
resource "hoop_access_request_rule" "dba_approvals" {
  name             = "dba-approvals"
  access_type      = "sidecar"
  reviewers_groups = ["dba-leads"]
  min_approvals    = 1
  pending_ttl_sec  = 900
  approval_ttl_sec = 600
}

resource "hoop_ai_session_analyzer_rule" "appdb" {
  name = "appdb-analysis"
  risk_evaluation = {
    low_risk    = { action = "allow_execution" }
    medium_risk = { action = "allow_execution" }
    high_risk   = { action = "block_execution" }
  }
  sidecar_spec = jsonencode({
    trigger       = "statement"
    high          = "require_review"
    approval_rule = hoop_access_request_rule.dba_approvals.name
  })
  sidecar_targets = [{ sidecar_id = hoop_sidecar.payments.id, listener_name = "appdb" }]
}
```

Leave `approval_rule` out and the analyzer rule holds under its own name: the gateway creates and manages the access request rule for it, and `reviewers_groups`, `pending_ttl_sec` and `approval_ttl_sec` on `hoop_ai_session_analyzer_rule` drive it. Use a separate `hoop_access_request_rule` when several analyzer rules share one approval policy.

Reviews raised this way are posted in Slack to the channels in `hoop_sidecar.slack_channels` for that listener.

## Record what the sidecars see

Sidecars enforce whether or not the control plane records their traffic. Recording is behind an organization flag, off by default:

```terraform
resource "hoop_feature_flag" "sidecar_session_events" {
  name    = "experimental.sidecar_session_events"
  enabled = true
}
```

With it on, each sidecar session shows in the session list with `identity_type = "sidecar"`.

## Watch the fleet converge

`hoop_sidecar` and `data.hoop_sidecar` report what each sidecar runs:

```terraform
output "payments_state" {
  value = {
    config_state = hoop_sidecar.payments.config_state
    version      = hoop_sidecar.payments.version
    last_error   = hoop_sidecar.payments.last_error
    bound_rules  = hoop_sidecar.payments.bound_rules
  }
}
```

`config_state` is `applied` when the sidecar runs the stored document, `applying` for a heartbeat after a change, `not_applied` when it stopped reporting, `refused` when it rejected the document (`last_error` says why), and `not_served` when the control plane refused to serve it to that build. A plan never changes because of these fields.

## What the plan catches

| You do | What happens |
|---|---|
| Plan `hoop_sidecar` on an organization without an Enterprise license | `Error: Enterprise License Required`, before any apply |
| Rename or remove a listener a rule is bound to | `Error: Listener Change Breaks Bound Rules`, naming the rules. Update their `sidecar_targets` first |
| Change a bound listener's protocol | the same error; a rule type the new protocol cannot run would be refused by the gateway |
| Put `load_from_disk` inside `configuration` | an error pointing at the attribute, which routes the change through the call the gateway requires |
| Remove `sidecar_targets` from a rule | a warning with the listener and sidecar counts |

## Import an existing fleet

```shell
terraform import hoop_sidecar.payments payments                       # by name or id
terraform import hoop_sidecar_service_account.gke_eu <uuid>
terraform import hoop_guardrail.no_destructive <uuid>
terraform import hoop_ai_session_analyzer_rule.appdb appdb-analysis   # by name
```

An imported `hoop_sidecar` has no `token`; the gateway cannot return it. An imported rule has null `sidecar_targets` and `sidecar_spec` in state until the configuration declares them.

## Destroying a sidecar

`terraform destroy` on a `hoop_sidecar` deletes it on the control plane and records its name. A pod that identifies itself with a platform token and maps to that name is refused from then on, so a fleet does not quietly recreate what an admin removed. To let the name be used again, clear it:

```shell
curl -X DELETE -H "Authorization: Bearer $HOOP_API_KEY" \
  https://hoop.example.com/api/sidecar-deleted-names/gke-eu-ws-123
```

A sidecar that was only bound to an identity, with no `hoop_sidecar` resource, is not touched by `terraform destroy`; removing the `hoop_sidecar_service_account` stops new pods from authenticating and leaves the sidecars in place.

## Switching a sidecar back to its own file

`load_from_disk = true` hands the document back to the sidecar's config file. The switch, in either direction, detaches every rule bound to the sidecar: rules that came from its file and nothing else uses are deleted, the rest are unbound. The plan warns with the count, and the apply lists each rule by name. Rules Terraform manages come back on the next apply; until then the sidecar runs without them.
