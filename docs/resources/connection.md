---
page_title: "hoop_connection Resource - hoop"
subcategory: ""
description: |-
  Manages a database connection in Hoop.dev.
---

# hoop_connection (Resource)

Provides a Hoop database connection resource. This allows database connections to be created, updated, and deleted.

## Example Usage

### Basic PostgreSQL Connection

```hcl
resource "hoop_connection" "postgres_example" {
  name     = "my-postgres"
  subtype  = "postgres"
  agent_id = "your-agent-id"

  secrets = {
    host     = "localhost"
    port     = "5432"
    user     = "postgres"
    pass     = "your-password"
    db       = "mydatabase"
    sslmode  = "prefer"
  }

  tags = ["production", "database"]
}
```

### MySQL Connection with Data Masking

```hcl
resource "hoop_connection" "mysql_secure" {
  name     = "secure-mysql"
  subtype  = "mysql"
  agent_id = "your-agent-id"

  secrets = {
    host = "localhost"
    port = "3306"
    user = "mysql"
    pass = "your-password"
    db   = "secure_db"
  }

  datamasking = true
  redact_types = ["EMAIL_ADDRESS", "CREDIT_CARD_NUMBER"]
  review_groups = ["dba-team"]
  
  tags = ["production", "secure"]
}
```

## Argument Reference

The following arguments are supported:

### Required

* `name` - (Required) The name of the connection. Must be unique and follow the pattern: `^[a-zA-Z0-9_]+(?:[-\.]?[a-zA-Z0-9_]+){2,253}$`
* `subtype` - (Required) The database type. Valid values: "postgres", "mysql", "mongodb", "mssql", "oracledb"
* `agent_id` - (Required) The ID of the agent that will manage this connection
* `secrets` - (Required) Connection credentials. Required fields vary by database type:
  * PostgreSQL:
    * `host` - Database host
    * `port` - Database port
    * `user` - Username
    * `pass` - Password
    * `db` - Database name
    * `sslmode` - (Optional) SSL mode
  * MySQL:
    * `host` - Database host
    * `port` - Database port
    * `user` - Username
    * `pass` - Password
    * `db` - Database name
  * MongoDB:
    * `connection_string` - MongoDB connection URI

### Optional

* `access_mode` - (Optional) Configuration for different types of access
  * `runbook` - (Optional) Enable runbook access. Default: true
  * `web` - (Optional) Enable web access. Default: true
  * `native` - (Optional) Enable native access. Default: true
* `access_schema` - (Optional) Enable schema access. Default: true
* `datamasking` - (Optional) Enable data masking. Default: false
* `redact_types` - (Optional) List of info types to redact. Default: []
* `review_groups` - (Optional) List of groups that can review connection access. Default: []
* `guardrails` - (Optional) List of guardrail IDs. Default: []
* `jira_template_id` - (Optional) ID of the Jira template for access requests. Default: ""
* `tags` - (Optional) List of tags to categorize the connection. Default: []

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the connection (same as name)

## Import

Connections can be imported using the connection name:

```shell
terraform import hoop_connection.example my-connection-name
```
