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

### Managing Multiple Access Groups for Same Connections

When creating multiple access groups that target the same connections, a race condition can occur due to asynchronous updates to the access_control plugin. To prevent this issue, use the `depends_on` attribute to establish a clear creation order:

```hcl
resource "hoop_access_group" "first_group" {
  group       = "first_group"
  description = "First group to access database connections"
  connections = [
    "postgres-shared",
    "mysql-shared"
  ]
}

resource "hoop_access_group" "second_group" {
  group       = "second_group"
  description = "Second group to access database connections"
  connections = [
    "postgres-shared",
    "mysql-shared"
  ]
  
  # This ensures that the first group is fully created before the second group is processed
  depends_on = [hoop_access_group.first_group]
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

## Race Conditions and Asynchronous Updates

The access_control plugin in Hoop may process updates asynchronously. When creating multiple access groups that reference the same connections in quick succession, the changes from the first group may not be fully synchronized when the second group is created. This can result in the second group overwriting rather than appending to the connection's group list.

To prevent this issue, always use Terraform's `depends_on` attribute to ensure a clear creation order when multiple access groups need to be assigned to the same connections.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the access group (same as group name)

## Import

Access groups can be imported using the group name:

```shell
terraform import hoop_access_group.example my-group-name
``` 
