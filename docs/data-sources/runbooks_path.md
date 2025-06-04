---
page_title: "hoop_runbooks_path Data Source - hoop"
subcategory: ""
description: |-
  Get information about an existing Hoop runbooks path configuration.
---

# hoop_runbooks_path (Data Source)

Use this data source to access information about an existing Hoop runbooks path configuration. This is useful when you want to reference an existing runbooks path without managing it through Terraform.

## Example Usage

### Basic Usage

```hcl
data "hoop_connection" "existing" {
  name = "existing-connection"
}

data "hoop_runbooks_path" "existing" {
  connection_id = data.hoop_connection.existing.id
}

# Use the data source outputs
output "runbooks_path" {
  value       = data.hoop_runbooks_path.existing.path
  description = "The configured runbooks path for the connection"
}
```

### Using the Path in Another Context

```hcl
data "hoop_runbooks_path" "prod_db" {
  connection_id = "existing-connection-id"
}

# Use the path in other resources or outputs
output "runbooks_location" {
  value = "Runbooks for production database are located at: ${data.hoop_runbooks_path.prod_db.path}"
}
```

### Checking if Runbooks are Configured

```hcl
data "hoop_connection" "all_connections" {
  # for each connection you want to check...
  name = "connection-name"
}

data "hoop_runbooks_path" "runbooks_check" {
  connection_id = data.hoop_connection.all_connections.id
}

output "has_runbooks" {
  value = data.hoop_runbooks_path.runbooks_check.path != "" ? "Yes" : "No"
}
```

## Argument Reference

The following arguments are supported:

* `connection_id` - (Required) The ID of the connection to look up runbooks path for.

## Attribute Reference

In addition to the argument above, the following attributes are exported:

* `id` - The ID of the runbooks path configuration (in the format "runbooks:{connection_id}")
* `connection_id` - The ID of the connection (same as input)
* `connection_name` - The name of the connection
* `path` - The configured path for runbooks
* `last_updated` - Timestamp of when the runbooks path was last updated (if available) 
