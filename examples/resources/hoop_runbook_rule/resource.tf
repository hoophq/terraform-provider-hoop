# manage which runbook to display based on a set of users and groups
resource "hoop_runbook_rule" "myrule" {
  name        = "My Runbook Rule"
  description = "My runbook rule description"
  connections = ["pgdemo", "pgprod"]
  user_groups = ["developers", "dba"]
  runbooks = [
    {
      repository = hoop_runbook_configuration.hoop-public-runbooks.repository // assuming this configuration exists
      name       = "postgres-demo/update-customer-email.runbook.sql"
    },
    {
      repository = hoop_runbook_configuration.hoop-public-runbooks.repository
      name       = "postgres-demo/delete-customer-by-id.runbook.sql"
    },
    {
      repository = hoop_runbook_configuration.hoop-public-runbooks.repository
      name       = "postgres-demo/fetch-customer-by-id.runbook.sql"
    }
  ]
}
