---
page_title: "hoop_runbooks_path Resource - hoop"
subcategory: ""
description: |-
  Manages a runbooks path in Hoop.dev for a specific connection.
---

# hoop_runbooks_path (Resource)

Provides a Hoop runbooks path resource. This allows you to configure the path where runbooks are stored for a specific connection.

## Example Usage

### Basic Runbooks Path Configuration

```hcl
resource "hoop_connection" "postgres_dev" {
  name     = "postgres-dev"
  # ... other connection attributes
}

resource "hoop_runbooks_path" "postgres_dev_runbooks" {
  connection_id   = hoop_connection.postgres_dev.id
  connection_name = hoop_connection.postgres_dev.name
  path            = "/path/to/runbooks"
  
  # Ensures the connection is created first
  depends_on = [hoop_connection.postgres_dev]
}
```

### Multiple Connection Configuration

```hcl
resource "hoop_connection" "postgres_prod" {
  name     = "postgres-prod"
  # ... other connection attributes
}

resource "hoop_connection" "mysql_prod" {
  name     = "mysql-prod"
  # ... other connection attributes
}

resource "hoop_runbooks_path" "postgres_prod_runbooks" {
  connection_id   = hoop_connection.postgres_prod.id
  connection_name = hoop_connection.postgres_prod.name
  path            = "/path/to/prod/pg/runbooks"
  
  depends_on = [hoop_connection.postgres_prod]
}

resource "hoop_runbooks_path" "mysql_prod_runbooks" {
  connection_id   = hoop_connection.mysql_prod.id
  connection_name = hoop_connection.mysql_prod.name
  path            = "/path/to/prod/mysql/runbooks"
  
  depends_on = [hoop_connection.mysql_prod]
}
```

## Argument Reference

The following arguments are supported:

### Required

* `connection_id` - (Required, ForceNew) The ID of the connection to configure with a runbooks path.

* `connection_name` - (Required, ForceNew) The name of the connection to configure with a runbooks path.

* `path` - (Required) The path to set for runbooks. Set to an empty string to remove the path.

## How It Works

The `hoop_runbooks_path` resource configures the path where runbooks are stored for a specific connection. Behind the scenes, this resource manages the "runbooks" plugin in Hoop, updating the configuration for the specified connection.

When a runbooks path is configured for a connection, Hoop will look for runbook files in the specified directory. This allows you to organize your runbooks by connection and have them automatically available in the Hoop UI.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the runbooks path resource (in the format "runbooks:{connection_id}")
* `last_updated` - The timestamp of the last update to this resource

## Import

Runbooks paths can be imported using the format `runbooks:{connection_id}`:

```shell
terraform import hoop_runbooks_path.example runbooks:5364ec99-653b-41ba-8165-67236e894990
``` 
