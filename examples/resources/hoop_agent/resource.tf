resource "hoop_agent" "agent" {
  name = "my-agent"
  mode = "standard"
}

resource "hoop_connection" "postgres" {
  name     = "postgres"
  type     = "database"
  agent_id = hoop_agent.agent.id

  access_mode_runbooks = "enabled"
  access_mode_exec     = "enabled"
  access_mode_connect  = "enabled"
  access_schema        = "enabled"
}

output "agent_token" {
  value     = hoop_agent.agent.token
  sensitive = true
}
