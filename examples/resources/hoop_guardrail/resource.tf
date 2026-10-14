# Copyright (c) HashiCorp, Inc.

# A guardrail enforced on the gateway, for the sessions of one connection.
resource "hoop_guardrail" "no_drop" {
  name        = "no-drop"
  description = "Refuse DROP statements"

  input = jsonencode({
    rules = [
      { type = "deny_words_list", words = ["DROP"], message = "DROP is not allowed here" }
    ]
  })

  connection_ids = [hoop_connection.pg_prod.id]
}

# The same intent on a sidecar fleet. sidecar_spec is the guardrails block in
# the sidecar's own vocabulary; sidecar_targets names the listeners that run it.
resource "hoop_guardrail" "no_destructive" {
  name        = "no-destructive"
  description = "No destructive statements on the appdb lane"

  sidecar_spec = jsonencode({
    rules = [
      {
        name       = "no-destructive"
        type       = "operation"
        operations = ["delete", "drop", "truncate"]
        message    = "destructive statements are not permitted on appdb"
      }
    ]
  })

  sidecar_targets = [
    # One listener of a sidecar this configuration manages.
    { sidecar_id = hoop_sidecar.payments.id, listener_name = "appdb" },
    # A listener of a sidecar a service account identity created.
    { sidecar_id = data.hoop_sidecar.gke_eu_ws_123.id, listener_name = "appdb" },
  ]
}
