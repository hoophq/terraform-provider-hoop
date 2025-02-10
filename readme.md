# Terraform Provider for Hoop.dev

This provider allows you to manage Hoop.dev resources through Terraform. Currently, it supports managing database connections with various configurations and security settings.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19 (for building the provider)
- [Hoop.dev](https://hoop.dev) account and API credentials

## Installation

```hcl
terraform {
  required_providers {
    hoop = {
      source = "registry.terraform.io/local/hoop"
      version = "1.0.0"
    }
  }
}

provider "hoop" {
  # See how to get your API key at: https://hoop.dev/docs/learn/api-key-usage
  api_key = var.hoop_api_key
  api_url = "http://localhost:8009/api"  # Your Hoop.dev API URL
}
```

## Usage Examples

### Basic Database Connection

```hcl
resource "hoop_connection" "simple_postgres" {
  name    = "user-service-db"
  subtype = "postgres"
  agent_id = "your-agent-id"

  secrets = {
    host     = "localhost"
    port     = "5432"
    user     = "postgres"
    pass     = "your-password"
    db       = "users"
    sslmode  = "verify-full"  # Optional
  }

  tags = ["production", "user-service"]
}
```

### Multiple Databases Using for_each

```hcl
locals {
  databases = {
    "users" = {
      subtype = "postgres"
      host    = "users-db.internal"
      db      = "users"
      tags    = ["prod", "core"]
    }
    "payments" = {
      subtype = "mysql"
      host    = "payments-db.internal"
      db      = "payments"
      tags    = ["prod", "financial"]
    }
  }
}

resource "hoop_connection" "service_databases" {
  for_each = local.databases

  name    = "${each.key}-db"
  subtype = each.value.subtype
  agent_id = var.agent_id

  secrets = {
    host = each.value.host
    port = "5432"
    user = var.db_user
    pass = var.db_password
    db   = each.value.db
  }

  tags = each.value.tags
}
```

### Using With Modules

```hcl
# modules/database/main.tf
variable "environment" {}
variable "service_name" {}
variable "database_config" {}

resource "hoop_connection" "database" {
  name    = "${var.service_name}-${var.environment}"
  subtype = var.database_config.type
  agent_id = var.agent_id

  secrets = var.database_config.secrets

  tags = concat(
    var.database_config.tags,
    [var.environment, var.service_name]
  )
}

# main.tf
module "user_service_db" {
  source = "./modules/database"

  environment  = "production"
  service_name = "user-service"
  
  database_config = {
    type = "postgres"
    secrets = {
      host = "user-db.prod.internal"
      port = "5432"
      user = var.db_user
      pass = var.db_password
      db   = "users"
    }
    tags = ["core"]
  }
}
```

## Resource: hoop_connection

### Required Arguments

- `name` - (Required) The name of the connection. Must be unique and follow the pattern: `^[a-zA-Z0-9_]+(?:[-\.]?[a-zA-Z0-9_]+){2,253}$`
- `subtype` - (Required) The database type. Valid values: "postgres", "mysql", "mongodb", "mssql", "oracledb"
- `agent_id` - (Required) The ID of the agent that will manage this connection
- `secrets` - (Required) Connection credentials. Required fields vary by database type:
  - PostgreSQL: host, port, user, pass, db (optional: sslmode)
  - MySQL: host, port, user, pass, db
  - MongoDB: connection_string
  - MSSQL: host, port, user, pass, db (optional: insecure)
  - OracleDB: host, port, user, pass, sid

### Optional Arguments

- `access_mode` - (Optional) Configuration for different types of access
  - `runbook` - (Optional) Enable runbook access. Default: true
  - `web` - (Optional) Enable web access. Default: true
  - `native` - (Optional) Enable native access. Default: true
- `access_schema` - (Optional) Enable schema access. Default: true
- `datamasking` - (Optional) Enable data masking. Default: false
- `redact_types` - (Optional) List of info types to redact. Default: []
- `review_groups` - (Optional) List of groups that can review connection access. Default: []
- `guardrails` - (Optional) List of guardrail ids. Default: []
- `jira_template_id` - (Optional) ID of the Jira template for access requests. Default: ""
- `tags` - (Optional) List of tags to categorize the connection. Default: []

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

MIT
