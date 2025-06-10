# Copyright (c) HashiCorp, Inc.

resource "hoop_connection" "bash" {
  name = "bash-console"
  type = "custom"
  # subtype  = ""
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  # command contains the main entrypoint command that will be executed
  # each entry is an argument to the command
  command = [
    "bash",
    "--verbose"
  ]

  secrets = {
    # expose as environment variable where $MYSECRET will contain the secret value
    "envvar:MYENV" = "value"
    # expose as environment variable where $MYFILE will contain the path to the file content
    "filesystem:MYFILE" = "file-content"
  }

  # user groups that are allowed to approve sessions for this connection
  reviewers = [
    "admin",
  ]

  # the entity types that are used to redact sensitive information
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

