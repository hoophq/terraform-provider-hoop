# Test Case 1: Basic Connection that will receive security updates
resource "hoop_connection" "basic_to_secure" {
  name     = "update-basic-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  # Initial basic configuration
  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = var.db_credentials.initial.user
    pass     = var.db_credentials.initial.pass
    db       = "basic_db"
    sslmode  = "prefer"  # Will be updated to verify-full
  }

  # Start with minimal security
  datamasking = false
  access_mode {
    runbook = true
    web     = true
    native  = true
  }

  tags = ["update-demo", "security-update"]

  # After apply, update with:
  # - Enable data masking
  # - Add redact types
  # - Change SSL mode
  # - Restrict access modes
  lifecycle {
    ignore_changes = []
  }
}

# Test Case 2: Connection demonstrating access mode updates
resource "hoop_connection" "access_update" {
  name     = "update-access-db"
  subtype  = "mysql"
  agent_id = var.agent_id

  # Initial configuration
  secrets = {
    host = var.database_hosts.mysql.host
    port = var.database_hosts.mysql.port
    user = var.db_credentials.initial.user
    pass = var.db_credentials.initial.pass
    db   = "access_db"
  }

  # Start with all access enabled
  access_mode {
    runbook = true
    web     = true
    native  = true
  }

  tags = ["update-demo", "access-update"]

  # After apply, update with:
  # - Disable native access
  # - Add review groups
  lifecycle {
    ignore_changes = []
  }
}

# Test Case 3: Connection showing guardrail updates
resource "hoop_connection" "guardrail_update" {
  name     = "update-guardrail-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = var.db_credentials.initial.user
    pass     = var.db_credentials.initial.pass
    db       = "guardrail_db"
    sslmode  = "verify-full"
  }

  # Start with basic guardrails
  guardrails = [
    "593ad7f2-a1cd-4c33-a0e0-b6bdebd65c5c",
    "f8c68e05-7e2c-43b7-9038-cac70a469fa0"
  ]

  tags = ["update-demo", "guardrail-update"]

  # After apply, update with:
  # - Add more complex guardrails
  # - Add pattern matching rules
  lifecycle {
    ignore_changes = []
  }
}

# Test Case 4: Connection demonstrating credential rotation
resource "hoop_connection" "credential_update" {
  name     = "update-credential-db"
  subtype  = "mysql"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.mysql.host
    port = var.database_hosts.mysql.port
    user = var.db_credentials.initial.user
    pass = var.db_credentials.initial.pass
    db   = "credential_db"
  }

  tags = ["update-demo", "credential-update"]

  # After apply, update with:
  # - New credentials
  # - Add SSL configuration
  lifecycle {
    ignore_changes = []
  }
}

# Outputs to show current state
output "connections_to_update" {
  value = {
    basic_to_secure = {
      name           = hoop_connection.basic_to_secure.name
      current_status = "initial setup - ready for security updates"
    }
    access_update = {
      name           = hoop_connection.access_update.name
      current_status = "initial setup - ready for access mode updates"
    }
    guardrail_update = {
      name           = hoop_connection.guardrail_update.name
      current_status = "initial setup - ready for guardrail updates"
    }
    credential_update = {
      name           = hoop_connection.credential_update.name
      current_status = "initial setup - ready for credential rotation"
    }
  }
  description = "Connections available for update demonstrations"
}
