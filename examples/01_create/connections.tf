# PostgreSQL Connections

# 1. Basic connection with minimal configuration
resource "hoop_connection" "basic_postgres" {
  name     = "basic-postgres"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = var.database_credentials.basic.user
    pass     = var.database_credentials.basic.password
    db       = "basic_db"
    sslmode  = "prefer"
  }

  tags = ["example", "basic"]
}

# 2. Connection with data masking enabled
resource "hoop_connection" "masked_postgres" {
  name     = "masked-postgres"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = var.database_credentials.basic.user
    pass     = var.database_credentials.basic.password
    db       = "masked_db"
    sslmode  = "prefer"
  }

  datamasking = true
  redact_types = [
    "EMAIL_ADDRESS",
    "CREDIT_CARD_NUMBER",
    "PHONE_NUMBER"
  ]

  tags = ["example", "masked"]
}

# 3. Connection with review process
resource "hoop_connection" "review_postgres" {
  name     = "review-postgres"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = var.database_credentials.basic.user
    pass     = var.database_credentials.basic.password
    db       = "review_db"
    sslmode  = "verify-full"
  }

  review_groups = ["dba-team", "security-team"]

  tags = ["example", "review"]
}

# 4. Connection with guardrails
resource "hoop_connection" "secure_postgres" {
  name     = "secure-postgres"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = var.database_hosts.postgres.port
    user     = var.database_credentials.basic.user
    pass     = var.database_credentials.basic.password
    db       = "secure_db"
    sslmode  = "verify-full"
  }

  guardrails = [
    "593ad7f2-a1cd-4c33-a0e0-b6bdebd65c5c",
    "f8c68e05-7e2c-43b7-9038-cac70a469fa0"
  ]

  tags = ["example", "secure"]
}

# MySQL Connections

# 1. Standard MySQL connection
resource "hoop_connection" "basic_mysql" {
  name     = "basic-mysql"
  subtype  = "mysql"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.mysql.host
    port = var.database_hosts.mysql.port
    user = var.database_credentials.basic.user
    pass = var.database_credentials.basic.password
    db   = "standard_db"
  }

  tags = ["example", "mysql"]
}

# 2. Replica database setup
resource "hoop_connection" "replica_mysql" {
  name     = "replica-mysql"
  subtype  = "mysql"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.mysql_replica.host
    port = var.database_hosts.mysql_replica.port
    user = var.database_credentials.replica.user
    pass = var.database_credentials.replica.password
    db   = "replica_db"
  }

  access_mode {
    runbook = true
    web     = true
    native  = false  # Disable direct connection to replica
  }

  tags = ["example", "mysql", "replica"]
}

# 3. MySQL with security features
resource "hoop_connection" "secure_mysql" {
  name     = "secure-mysql"
  subtype  = "mysql"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.mysql.host
    port = var.database_hosts.mysql.port
    user = var.database_credentials.basic.user
    pass = var.database_credentials.basic.password
    db   = "secure_db"
  }

  datamasking = true
  redact_types = ["EMAIL_ADDRESS"]
  review_groups = ["dba-team"]
  guardrails = [
    "593ad7f2-a1cd-4c33-a0e0-b6bdebd65c5c",
    "f8c68e05-7e2c-43b7-9038-cac70a469fa0"
    ]

  tags = ["example", "mysql", "secure"]
}

# Oracle Database

# 1. Basic Oracle connection
resource "hoop_connection" "basic_oracle" {
  name     = "basic-oracle"
  subtype  = "oracledb"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.oracle.host
    port = var.database_hosts.oracle.port
    user = var.database_credentials.oracle.user
    pass = var.database_credentials.oracle.password
    sid  = var.database_hosts.oracle.sid
  }

  tags = ["example", "oracle"]
}

# 2. Oracle Enterprise setup
resource "hoop_connection" "enterprise_oracle" {
  name     = "enterprise-oracle"
  subtype  = "oracledb"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.oracle.host
    port = var.database_hosts.oracle.port
    user = var.database_credentials.oracle_enterprise.user
    pass = var.database_credentials.oracle_enterprise.password
    sid  = var.database_hosts.oracle.sid
  }

  datamasking = true
  redact_types = [
    "EMAIL_ADDRESS",
    "CREDIT_CARD_NUMBER"
  ]
  review_groups = ["oracle-dba"]

  tags = ["example", "oracle", "enterprise"]
}

# MongoDB

# 1. Simple MongoDB connection
resource "hoop_connection" "basic_mongodb" {
  name     = "basic-mongodb"
  subtype  = "mongodb"
  agent_id = var.agent_id

  secrets = {
    connection_string = var.database_hosts.mongodb.connection_string
  }

  tags = ["example", "mongodb"]
}

# 2. MongoDB with connection string and security
resource "hoop_connection" "secure_mongodb" {
  name     = "secure-mongodb"
  subtype  = "mongodb"
  agent_id = var.agent_id

  secrets = {
    connection_string = var.database_hosts.mongodb_replica.connection_string
  }

  review_groups = ["mongodb-dba"]
  
  tags = ["example", "mongodb", "secure"]
}
