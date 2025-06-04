---
page_title: "hoop_access_group Data Source - hoop"
subcategory: ""
description: |-
  Get information about an existing Hoop access group.
---

# hoop_access_group (Data Source)

Use this data source to access information about an existing Hoop access group. This is useful when you want to reference an existing access group without managing it through Terraform.

## Example Usage

### Basic Usage

```hcl
data "hoop_access_group" "existing" {
  group = "existing-group"
}

# Use the data source outputs
output "existing_group_connections" {
  value       = data.hoop_access_group.existing.connections
  description = "The connections that this group has access to"
}
```

### Adding a Connection to an Existing Access Group

```hcl
data "hoop_access_group" "existing" {
  group = "existing-group"
}

resource "hoop_connection" "new_connection" {
  name     = "new-postgres"
  subtype  = "postgres"
  agent_id = var.agent_id
  # ... other connection attributes
}

resource "hoop_access_group" "updated" {
  group       = data.hoop_access_group.existing.group
  description = data.hoop_access_group.existing.description
  
  # Add the new connection to the existing list
  connections = concat(
    data.hoop_access_group.existing.connections,
    [hoop_connection.new_connection.name]
  )
}
```

### Referencing an Existing Access Group in a Constraint

```hcl
data "hoop_access_group" "dba" {
  group = "database-admins"
}

# Use the data in other configurations
output "dba_connections" {
  value = "DBA team has access to: ${join(", ", data.hoop_access_group.dba.connections)}"
}
```

## Argument Reference

The following arguments are supported:

* `group` - (Required) The name of the existing access group to look up.

## Attribute Reference

In addition to the argument above, the following attributes are exported:

* `id` - The ID of the access group (same as group name)
* `group` - The name of the access group
* `description` - The description of the access group
* `connections` - The list of connection names that this group can access 
