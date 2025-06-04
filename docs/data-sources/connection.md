---
page_title: "hoop_connection Data Source - hoop"
subcategory: ""
description: |-
  Get information about an existing Hoop connection.
---

# hoop_connection (Data Source)

Use this data source to access information about an existing Hoop connection. This is useful when you want to reference an existing connection without managing it through Terraform.

## Example Usage

### Basic Usage

```hcl
data "hoop_connection" "existing" {
  name = "my-existing-connection"
}

# Use the data source outputs
resource "hoop_access_group" "example" {
  group       = "example_group"
  description = "Example access group using data source"
  connections = [
    data.hoop_connection.existing.name
  ]
}
```

### Multiple Data Sources

```hcl
data "hoop_connection" "postgres" {
  name = "existing-postgres"
}

data "hoop_connection" "mysql" {
  name = "existing-mysql"
}

resource "hoop_access_group" "db_users" {
  group       = "db_users"
  description = "Database users group"
  connections = [
    data.hoop_connection.postgres.name,
    data.hoop_connection.mysql.name
  ]
}
```

### Setting Runbooks Path for Existing Connection

```hcl
data "hoop_connection" "existing" {
  name = "existing-connection"
}

resource "hoop_runbooks_path" "example" {
  connection_id   = data.hoop_connection.existing.id
  connection_name = data.hoop_connection.existing.name
  path            = "/path/to/runbooks"
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the existing connection to look up.

## Attribute Reference

In addition to the argument above, the following attributes are exported:

* `id` - The ID of the connection
* `name` - The name of the connection
* `type` - The type of the connection (usually "database")
* `subtype` - The database subtype of the connection (postgres, mysql, etc.)
* `agent_id` - The agent ID associated with the connection
* `access_mode` - The configured access modes:
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
