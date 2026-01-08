---
page_title: "Terraform Hoop Provider 0.0.19 Upgrade Guide"
subcategory: ""
---

Version 0.0.19 introduces integration with new endpoints for defining multiple repository Runbook Configurations and rules. This allows you to specify which user groups and connections can interact with a given runbook. This guide outlines the key updates and how to adapt your configurations accordingly.

Hoop Gateway version 1.46+ introduces breaking changes that remove support for managing `hoop_plugin_connection` and `hoop_plugin_config` for the **runbooks plugin**.

## Resource: `hoop_plugin_connection`

This resource no longer supports the **runbooks** plugin. It has been superseded by the new `hoop_runbook_rule` resource.

## Resource: `hoop_plugin_config`

This resource no longer supports the **runbooks** plugin. It has been superseded by the new `hoop_runbook_configuration` resource.

## Deprecated Resources

The following resources are deprecated for managing the **plugin runbooks**

- `hoop_plugin_connection`
- `hoop_plugin_config`

## State Migration

If your state files contain the removed resources, you'll need to manually remove them from your state using the terraform state rm command:

```sh
terraform state rm hoop_plugin_config.runbooks
terraform state rm hoop_plugin_connection.myrule1
terraform state rm hoop_plugin_connection.myrule2
```

### Practical Example

Here's an outdated resource configuration that requires updating. The example below shows a connection using the deprecated plugin runbook configuration.

```hcl
terraform {
  required_providers {
    hoop = {
      source = "hoophq/hoop"
      version = "0.0.18"
    }
  }
}

provider "hoop" {
  api_url = "http://localhost:8009/api"
  api_key = "<api-key>"
}

resource "hoop_connection" "postgres-demo" {
  name     = "postgres-demo"
  type     = "database"
  subtype  = "postgres"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  secrets = {
    "envvar:HOST"     = "pg-demo-public-dns-url"
    "envvar:PORT"     = "5432"
    "envvar:USER"     = "pguser"
    "envvar:PASS"     = "pgpwd"
    "envvar:DB"       = "dellstore"
    "envvar:SSLMODE"  = "prefer"
  }

  command = [
    "psql",
    "-v",
    "ON_ERROR_STOP=1",
    "-A",
    "-F\t",
    "-P",
    "pager=off",
    "-h",
    "$HOST",
    "-U",
    "$USER",
    "--port=$PORT",
    "$DB",
  ]

  access_mode_runbooks = "enabled"
  access_mode_exec     = "enabled"
  access_mode_connect  = "enabled"
  access_schema        = "enabled"

  tags = {
    environment = "development"
    type        = "database"
  }
}

resource "hoop_plugin_config" "runbooks" {
  plugin_name = "runbooks"
  config = {
    GIT_URL = "https://github.com/hoophq/runbooks.git"
  }
}

resource "hoop_plugin_connection" "postgres-demo" {
  plugin_name   = "runbooks"
  connection_id = hoop_connection.postgres-demo.id
  config        = ["postgres-demo/fetch-customer-by-id.runbook.sql"]
}
```

- **Updating the new resources**

```hcl
terraform {
  required_providers {
    hoop = {
      source = "hoophq/hoop"
      version = "0.0.19"
    }
  }
}

provider "hoop" {
  api_url = "http://localhost:8009/api"
  api_key = "<api-key>"
}

resource "hoop_connection" "postgres-demo" {
  name     = "postgres-demo"
  type     = "database"
  subtype  = "postgres"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  secrets = {
    "envvar:HOST"     = "demo-pg-db.public-dns"
    "envvar:PORT"     = "5432"
    "envvar:USER"     = "pgdemo-user"
    "envvar:PASS"     = "pgdemo-pwd"
    "envvar:DB"       = "dellstore"
    "envvar:SSLMODE"  = "prefer"
  }

  command = [
    "psql",
    "-v",
    "ON_ERROR_STOP=1",
    "-A",
    "-F\t",
    "-P",
    "pager=off",
    "-h",
    "$HOST",
    "-U",
    "$USER",
    "--port=$PORT",
    "$DB",
  ]

  access_mode_runbooks = "enabled"
  access_mode_exec     = "enabled"
  access_mode_connect  = "enabled"
  access_schema        = "enabled"

  tags = {
    environment = "development"
    type        = "database"
  }
}

resource "hoop_runbook_configuration" "runbooks" {
  git_url         = "https://github.com/hoophq/runbooks.git"
  git_hook_ttl    = 0
  git_user        = ""
  git_password    = ""
  ssh_user        = ""
  ssh_key         = ""
  ssh_keypass     = ""
  ssh_known_hosts = ""
}

resource "hoop_runbook_rule" "postgres-demo" {
  name        = "Postgres Dev Migration"
  description = "Postgres Dev Migration Tool Rule"
  connections = [hoop_connection.postgres-demo.name]
  user_groups = []
  runbooks = [
    { 
      repository = hoop_runbook_configuration.runbooks.repository
      name       = "postgres-dev/run-migration.runbook.sql"
    }
  ]
}

```

1. Upgrade the provider with the new version

```sh
terraform init -upgrade
```

2. Remove the old resources from the state

```sh
terraform state rm hoop_plugin_config.runbooks
terraform state rm hoop_plugin_connection.postgres-demo
```

3. Import the new resources into the state

To import the runbook rule resource into the state, you must first retrieve the current state from the API.

When you start the gateway on version 1.46 or later, it automatically migrates plugin connections to runbook rules and removes the old resources. As a result, you'll need to list the rules from the API to identify which rules are associated with this repository.

You can list the runbook rules from the command line using the following command:

```sh
hoop config create --api-url https://your-public-gateway-url
hoop login
curl -H "Authorization: Bearer $(hoop config view token)" https://your-public-gateway-url/api/runbooks/rules |jq
```

Then, import the state of the new resource:

```sh
terraform import hoop_runbook_configuration.runbooks https://github.com/hoophq/runbooks.git
terraform import hoop_runbook_rule.postgres-demo <runbook-rule-resource-id>
```

In case you have many rules and it's hard to map all the resources properly, it's possible to erase all the **Runbook Rules** migrated and just build the new resources via terraform and apply it.

```sql
DELETE FROM private.runbook_rules
WHERE description = 'Auto-migrated from Runbooks';
```

4. Apply the changes

```sh
terraform apply
```
