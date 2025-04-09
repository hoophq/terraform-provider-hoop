---
page_title: "hoop_access_group Resource - hoop"
subcategory: ""
description: |-
  Manages a access group in Hoop.dev for connection access control.
---

# hoop_access_group (Resource)

Provides a Hoop access group resource. This allows you to create, update, and delete access groups that control which groups of users can access specific connections.

## Example Usage

### Basic Access Control Group

```hcl
resource "hoop_access_group" "developers" {
  group       = "developers"
  description = "Access group for development team"
  connections = [
    "postgres-dev",
    "mysql-dev"
  ]
}
```

### Production Access Control

```hcl
resource "hoop_access_group" "database_admins" {
  group       = "database_admins"
  description = "Access group for database administrators"
  connections = [
    "postgres-prod",
    "mysql-prod",
    "mongo-prod"
  ]
}
```

## Argument Reference

The following arguments are supported:

### Required

* `group` - (Required, ForceNew) The name of the access group. This is the identifier for the group of users that will have access to the connections. This cannot be changed after creation.

* `connections` - (Required) A list of connection names that this group is allowed to access. This controls which connections are accessible to the users in this group.

### Optional

* `description` - (Optional) A description for the access group to help identify its purpose.

## How It Works

The `hoop_access_group` resource creates and manages an access control mechanism that associates user groups with connections. When a user belonging to a specified group attempts to access Hoop, they will only be able to see and connect to the connections associated with their groups.

Behind the scenes, this resource utilizes Hoop's access_control plugin to manage these associations. Each connection can be accessible by multiple groups, and each group can access multiple connections.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the access group (same as group name)

## Import

Access groups can be imported using the group name:

```shell
terraform import hoop_access_group.example my-group-name
``` 
