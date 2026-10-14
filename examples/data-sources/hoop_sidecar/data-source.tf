# Copyright (c) HashiCorp, Inc.

# A sidecar the gke-eu service account identity created on its first
# handshake, named by that account's name_template.
data "hoop_sidecar" "gke_eu_ws_123" {
  name = "gke-eu-ws-123"
}

resource "hoop_guardrail" "no_destructive" {
  name = "no-destructive"

  sidecar_spec = jsonencode({
    rules = [{ name = "no-destructive", type = "operation", operations = ["delete", "drop"] }]
  })

  sidecar_targets = [{ sidecar_id = data.hoop_sidecar.gke_eu_ws_123.id }]
}

output "gke_eu_ws_123_config_state" {
  value = data.hoop_sidecar.gke_eu_ws_123.config_state
}
