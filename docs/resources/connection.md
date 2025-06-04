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
    pass     = var.db_password # Use variables for sensitive information
    db       = "mydatabase"
    sslmode  = "prefer"
  }

  tags = {
    environment = "production"
    team        = "data-platform"
    managed-by  = "terraform"
  }
}
```

### MySQL Connection with Data Masking

```hcl
resource "hoop_connection" "mysql_secure" {
  name     = "secure-mysql"
  subtype  = "mysql"
  agent_id = var.agent_id

  secrets = {
    host = "localhost"
    port = "3306"
    user = "mysql"
    pass = var.mysql_password
    db   = "secure_db"
  }

  datamasking = true
  redact_types = ["EMAIL_ADDRESS", "CREDIT_CARD_NUMBER"]
  review_groups = ["dba-team"]
  
  access_mode {
    runbook = true
    web     = true
    native  = false # Disable direct native connections for security
  }
  
  tags = {
    environment = "production"
    security    = "high"
    compliance  = "pci-dss"
  }
}
```

### MongoDB Connection with Connection String

```hcl
resource "hoop_connection" "mongodb_example" {
  name     = "analytics-mongodb"
  subtype  = "mongodb"
  agent_id = var.agent_id

  secrets = {
    connection_string = var.mongodb_connection_string
  }

  access_schema = true
  
  tags = {
    environment = "production"
    purpose     = "analytics"
  }
}
```

### Custom Connection Example

```hcl
resource "hoop_connection" "custom_command" {
  name     = "my-custom-script"
  type     = "custom"  # Specify "custom" for custom command connections
  subtype  = "shell"   # Can be any identifier describing your custom implementation
  agent_id = var.agent_id

  # Command to execute as an array of strings
  command = [
    "/bin/bash",
    "-c",
    "echo 'Hello from custom connection'"
  ]

  # Environment variables for the command (optional)
  secrets = {
    API_TOKEN = var.api_token
    DEBUG     = "true"
  }

  # Configure access modes as needed
  access_mode {
    runbook = true
    web     = true
    native  = false
  }

  tags = {
    environment = "development"
    type        = "custom"
    purpose     = "demo"
  }
}
```

## Argument Reference

The following arguments are supported:

### Required

* `name` - (Required) The name of the connection. Must be unique and follow the pattern: `^[a-zA-Z0-9_]+(?:[-\.]?[a-zA-Z0-9_]+){2,253}$`
* `subtype` - (Required) The database type or custom subtype identifier. For database connections, valid values include: "postgres", "mysql", "mongodb", "mssql", "oracledb". For custom connections, can be any identifier that describes the custom implementation.
* `agent_id` - (Required) The ID of the agent that will manage this connection

* `secrets` - (Required) Connection credentials. Required fields vary by connection type:
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
  * Custom connections:
    * Any key-value pairs that should be available as environment variables to the command

### Optional

* `type` - (Optional) The type of connection. Valid values: "database" (default) or "custom".
* `command` - (Optional) Command to execute for custom connections. List of strings where the first element is the program to run, and remaining elements are passed as arguments. Recommended for type "custom" to define the command to execute.
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
* `tags` - (Optional) Key-value map of tags to categorize the connection. Each tag consists of a key and a value string. Default: {}

-> **Note:** Prior versions of this provider used `connection_tags` instead of `tags`. The field has been renamed for consistency in the 2.0 version, but the functionality remains the same.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the connection (system-generated UUID)
* `name` - The name of the connection (same as input)
* `type` - The type of the connection
* `subtype` - The database subtype of the connection
* `agent_id` - The agent ID associated with the connection
* `access_mode` - The configured access modes with their settings:
  * `runbook` - Whether runbook access is enabled
  * `web` - Whether web access is enabled
  * `native` - Whether native access is enabled
* `access_schema` - Whether schema access is enabled
* `datamasking` - Whether data masking is enabled
* `redact_types` - The list of types being redacted when datamasking is enabled
* `review_groups` - The list of groups that can review access requests
* `guardrails` - The list of guardrail IDs applied to the connection
* `jira_template_id` - The ID of the Jira template associated with the connection
* `tags` - The key-value tags associated with the connection
* `command` - The command to execute (for custom connections)

## Import

Connections can be imported using the connection name:

```shell
terraform import hoop_connection.example my-connection-name
```
