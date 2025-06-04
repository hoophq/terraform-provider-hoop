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
  subtype  = "postgres"
  agent_id = var.agent_id
  # ... other connection attributes
}

resource "hoop_runbooks_path" "postgres_dev_runbooks" {
  connection_id   = hoop_connection.postgres_dev.id
  connection_name = hoop_connection.postgres_dev.name
  path            = "/path/to/runbooks"
  
  # Connection reference creates an implicit dependency
  # No need for explicit depends_on
}
```

### Multiple Connection Configuration

```hcl
resource "hoop_connection" "postgres_prod" {
  name     = "postgres-prod"
  subtype  = "postgres"
  agent_id = var.agent_id
  # ... other connection attributes
}

resource "hoop_connection" "mysql_prod" {
  name     = "mysql-prod"
  subtype  = "mysql"
  agent_id = var.agent_id
  # ... other connection attributes
}

resource "hoop_runbooks_path" "postgres_prod_runbooks" {
  connection_id   = hoop_connection.postgres_prod.id
  connection_name = hoop_connection.postgres_prod.name
  path            = "/path/to/prod/pg/runbooks"
}

resource "hoop_runbooks_path" "mysql_prod_runbooks" {
  connection_id   = hoop_connection.mysql_prod.id
  connection_name = hoop_connection.mysql_prod.name
  path            = "/path/to/prod/mysql/runbooks"
}
```

### Using with Existing Connections

If you want to manage runbooks paths for connections that already exist, you can use string literals:

```hcl
resource "hoop_runbooks_path" "existing_connection_runbooks" {
  connection_id   = "existing-connection-id"
  connection_name = "existing-connection-name"
  path            = "/path/to/existing/runbooks"
}
```

Or use a data source for more safety:

```hcl
data "hoop_connection" "existing" {
  name = "existing-connection"
}

resource "hoop_runbooks_path" "existing_connection_runbooks" {
  connection_id   = data.hoop_connection.existing.id
  connection_name = data.hoop_connection.existing.name
  path            = "/path/to/existing/runbooks"
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

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The ID of the runbooks path resource (in the format "runbooks:{connection_id}")
* `connection_id` - The ID of the connection associated with this runbooks path
* `connection_name` - The name of the connection associated with this runbooks path
* `path` - The configured runbooks path
* `last_updated` - The timestamp of the last update to this resource (RFC3339 format)

## Import

Runbooks paths can be imported using the format `runbooks:{connection_id}`:

```shell
terraform import hoop_runbooks_path.example runbooks:5364ec99-653b-41ba-8165-67236e894990
``` 
