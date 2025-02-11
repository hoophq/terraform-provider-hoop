# Test Case 1: Simple Connection Removal
# This connection has no dependencies or active sessions
resource "hoop_connection" "simple_db" {
  name     = "cleanup-simple-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = "app_user"
    pass     = "mock-password-123"
    db       = "simple_db"
    sslmode  = "prefer"
  }

  # Disable all access modes to prevent new connections
  access_mode {
    runbook = false
    web     = false
    native  = false
  }

  tags = ["cleanup-test", "simple"]
}

# Test Case 2: Connection with Review Process
resource "hoop_connection" "review_db" {
  name     = "cleanup-review-db"
  subtype  = "mysql"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.mysql.host
    port = var.database_hosts.mysql.port
    user = "app_user"
    pass = "mock-password-123"
    db   = "review_db"
  }

  review_groups = ["dba-team"]
  
  # Disable access during cleanup
  access_mode {
    runbook = false
    web     = false
    native  = false
  }

  tags = ["cleanup-test", "review"]
}

# Test Case 3: Connection with Security Features
resource "hoop_connection" "secure_db" {
  name     = "cleanup-secure-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = "app_user"
    pass     = "mock-password-123"
    db       = "secure_db"
    sslmode  = "verify-full"
  }

  datamasking = true
  redact_types = [
    "EMAIL_ADDRESS",
    "CREDIT_CARD_NUMBER"
  ]

  # Disable access during cleanup
  access_mode {
    runbook = false
    web     = false
    native  = false
  }

  tags = ["cleanup-test", "secure"]
}

# Status Outputs
output "connection_status" {
  value = {
    simple_db = hoop_connection.simple_db.name
    review_db = hoop_connection.review_db.name
    secure_db = hoop_connection.secure_db.name
  }
  description = "Status of test connections"
}
