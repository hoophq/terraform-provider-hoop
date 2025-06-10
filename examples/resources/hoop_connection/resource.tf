resource "hoop_connection" "bash" {
  name     = "bash-console"
  type     = "custom"
  subtype  = ""
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  command = [
    "bash",
  ]

  secrets = {
    "envvar:MYENV"      = "value"
    "filesystem:MYFILE" = "file-content"
  }

  reviewers = [
    "admin",
  ]

  redact_types = [
    "EMAIL_ADDRESS",
    "PHONE_NUMBER"
  ]

  access_mode_runbooks = "enabled"
  access_mode_exec     = "enabled"
  access_mode_connect  = "enabled"
  access_schema        = "enabled"

  # guardrail_rules = []
  # jira_issue_template_id = ""


  tags = {
    environment = "development"
    type        = "custom"
    purpose     = "demo"
  }
}

