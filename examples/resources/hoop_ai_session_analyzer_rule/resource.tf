# Copyright (c) HashiCorp, Inc.

# Session analysis on the gateway for two connections.
resource "hoop_ai_session_analyzer_rule" "prod" {
  name             = "prod-analysis"
  description      = "Block high-risk sessions on production databases"
  connection_names = [hoop_connection.pg_prod.name, hoop_connection.mysql_prod.name]

  risk_evaluation = {
    low_risk    = { action = "allow_execution" }
    medium_risk = { action = "allow_execution" }
    high_risk   = { action = "require_access_request", access_request_rule_name = "prod-approvals" }
  }

  custom_prompt = "Treat any query that touches the payments schema as high risk."
}

# Statement analysis on a sidecar listener. sidecar_spec is the analyzer block
# the listener receives; a held statement waits up to 15 minutes for one of
# the dba-leads to release it.
resource "hoop_ai_session_analyzer_rule" "appdb" {
  name = "appdb-analysis"

  risk_evaluation = {
    low_risk    = { action = "allow_execution" }
    medium_risk = { action = "allow_execution" }
    high_risk   = { action = "block_execution" }
  }

  sidecar_spec = jsonencode({
    trigger = "statement"
    high    = "defer"
  })

  sidecar_targets  = [{ sidecar_id = hoop_sidecar.payments.id, listener_name = "appdb" }]
  reviewers_groups = ["dba-leads"]
  pending_ttl_sec  = 900
}
