# Copyright (c) HashiCorp, Inc.

# Who may release a statement a sidecar held. Several analyzer rules name it
# through approval_rule in their sidecar_spec.
resource "hoop_access_request_rule" "dba_approvals" {
  name             = "dba-approvals"
  description      = "DBA leads release statements the sidecars hold"
  access_type      = "sidecar"
  reviewers_groups = ["dba-leads"]
  min_approvals    = 1
  pending_ttl_sec  = 900 # a held statement waits 15 minutes for a decision
  approval_ttl_sec = 600 # an approval is good for 10 minutes
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

# A connection rule: production databases need two approvals from SRE.
resource "hoop_access_request_rule" "prod_jit" {
  name                     = "prod-jit"
  access_type              = "jit"
  connection_names         = [hoop_connection.pg_prod.name]
  approval_required_groups = ["developers"]
  reviewers_groups         = ["sre"]
  force_approval_groups    = ["admin"]
  min_approvals            = 2
  access_max_duration      = 3600
}
