---
page_title: "Getting Started with Hoop Provider"
subcategory: ""
description: |-
  Learn how to use the Hoop provider to manage your database connections.
---

# Getting Started with Hoop Provider

This guide will help you get started with using the Hoop provider to manage your database connections.

## Prerequisites

Before you begin, ensure you have:

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- A [Hoop.dev](https://hoop.dev) account
- API credentials (see: [API key usage](https://hoop.dev/docs/learn/api-key-usage))
- A configured Hoop agent

## Installation

First, create a new directory for your Terraform configuration:

```bash
mkdir hoop-terraform
cd hoop-terraform
```

Create a file named `main.tf` with the following content:

```hcl
terraform {
  required_providers {
    hoop = {
      source = "hoophq/hoop"
      version = "~> 0.0.1"
    }
  }
}

provider "hoop" {
  # We recommend using environment variables for credentials
}
```

Create a file named `variables.tf`:

```hcl
variable "hoop_api_key" {
  type        = string
  description = "API Key for Hoop.dev"
  sensitive   = true
}

variable "api_url" {
  type        = string
  description = "API URL for Hoop.dev"
  default     = "http://localhost:8009/api"
}

variable "agent_id" {
  type        = string
  description = "Agent ID to use for connections"
}
```

Create a file named `terraform.tfvars` (and add it to your .gitignore):

```hcl
hoop_api_key = "your-api-key"
api_url      = "http://localhost:8009/api"
agent_id     = "your-agent-id"
```

## Creating Your First Connection

Add the following to your `main.tf`:

```hcl
resource "hoop_connection" "first_db" {
  name     = "my-first-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = "localhost"
    port     = "5432"
    user     = "postgres"
    pass     = "your-password"
    db       = "testdb"
    sslmode  = "prefer"
  }

  tags = ["test", "getting-started"]
}
```

## Setting Up Access Control

Hoop allows you to control which user groups can access specific connections using the `hoop_access_group` resource. This is a powerful feature for managing access in multi-team environments.

Add the following to your configuration to create an access group for your database team:

```hcl
resource "hoop_access_group" "db_team" {
  group       = "db_team"
  description = "Database team with access to specific databases"
  
  connections = [
    hoop_connection.first_db.name
  ]
}
```

In this example, users belonging to the "db_team" group in your organization will have access to the "first_db" connection.

You can create multiple access groups for different teams or purposes:

```hcl
resource "hoop_access_group" "dev_team" {
  group       = "dev_team"
  description = "Development team access"
  
  connections = [
    hoop_connection.first_db.name
  ]
}
```

Once these access groups are created, users will only be able to see and access the connections associated with their groups, providing a simple yet effective access control mechanism.

## Initialize and Apply

Initialize Terraform:

```bash
terraform init
```

Review the planned changes:

```bash
terraform plan
```

Apply the configuration:

```bash
terraform apply
```

## Next Steps

After you've created your first connection, you might want to:

1. Add security features like data masking
2. Configure access modes
3. Set up review groups
4. Add guardrails

Check out the [connection resource documentation](../resources/connection.md) for more details on these features.

For more information on access control, see the [access_group resource documentation](../resources/access_group.md).
