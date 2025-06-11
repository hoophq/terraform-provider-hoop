---
page_title: "Terraform Hoop Provider 0.0.14 Upgrade Guide"
subcategory: ""
---

Version 0.0.14 of the Hoop Terraform Provider introduces several changes and improvements. This guide outlines the key updates and how to adapt your configurations accordingly. The older version of this provider had some bugs that were affecting the provider and had no backwards-compatible solutions. This version aims to correct those issues while introducing new features and improvements.

## Resource: `hoop_connection`

The following deprecated fields have been removed or replaced and will now throw an error if you try to use them:

- `datamasking`
- `access_mode`
- `review_groups`
- `guardrails`
- `jira_template_id`

The following fields have been added and are now required:

- `type`: The type of the connection
- `access_mode_runbooks`: Enable runbooks for this connection
- `access_mode_exec`: Enable exec for this connection
- `access_mode_connect`: Enable connect for this connection
- `access_schema`: Enables or disables displaying the introspection schema tree of database type connections

The `secrets` block has also been updated that aligns with the API.
Each key could now use the `envvar:` or `filesystem:` prefix to indicate how the secret should be handled in runtime.
For database type connections, the secrets should be provided with the `envvar:` prefix, as shown below:

```hcl
secrets = {
  "envvar:HOST"     = "localhost"
  "envvar:PORT"     = "5432"
  "envvar:USER"     = "postgres"
  "envvar:PASS"     = "your-password"
  "envvar:DB"       = "mydatabase"
  "envvar:SSLMODE"  = "prefer"
}
```

## Deprecated Resources

The following resources have been removed in this version:

- `hoop_access_group`
- `hoop_runbooks_path`

## State Migration

If you have existing state files that include the removed resources, you will need to manually remove them from your state. You can do this using the `terraform state rm` command:

```sh
terraform state rm hoop_access_group.example
terraform state rm hoop_runbooks_path.example
```

### Practical Example

Here's an old resource configuration that needs to be updated. In the example below, we will update the `hoop_connection` resource to include the new required fields and remove the deprecated ones.

```hcl
terraform {
  required_providers {
    hoop = {
      source = "hoophq/hoop"
      version = "0.0.11"
    }
  }
}

provider "hoop" {
  api_url = "http://localhost:8009/api"
  api_key = "<org-id>|<random-key>"
}

resource "hoop_connection" "postgres_dev" {
  name     = "postgres-dev"
  subtype  = "postgres"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  secrets = {
    host     = "localhost"
    port     = "5432"
    user     = "postgres"
    pass     = "your-password"
    db       = "mydatabase"
    sslmode  = "prefer"
  }

  datamasking = true
  redact_types = ["EMAIL_ADDRESS", "CREDIT_CARD_NUMBER"]
  review_groups = ["dba-team"]

  tags = {
    environment = "development"
    type        = "database"
  }
}

resource "hoop_runbooks_path" "postgres_dev_runbooks" {
  connection_id   = hoop_connection.postgres_dev.id
  connection_name = hoop_connection.postgres_dev.name
  path            = "ops/"

  # Ensures the connection is created first
  depends_on = [hoop_connection.postgres_dev]
}
```

- **Updating the new resources**

```hcl
terraform {
  required_providers {
    hoop = {
      source = "hoophq/hoop"
      version = "0.0.14"
    }
  }
}

provider "hoop" {
  api_url = "http://localhost:8009/api"
  api_key = "fd57eb76-8ab2-4396-b1e8-5a07da2f1a21|VOH806wYuWtwWgDjDikW6wKmk1sCG0T4T5OE7tDFnytHe3RfjLqKGiEX/EOG/6z7I6T/YQfEAsriaysxZcZbhw=="
}

resource "hoop_connection" "postgres_dev" {
  name     = "postgres-dev"
  type     = "database"
  subtype  = "postgres"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  secrets = {
    "envvar:HOST"     = "localhost"
    "envvar:PORT"     = "5432"
    "envvar:USER"     = "postgres"
    "envvar:PASS"     = "your-password"
    "envvar:DB"       = "mydatabase"
    "envvar:SSLMODE"  = "prefer"
  }

  redact_types = ["EMAIL_ADDRESS", "CREDIT_CARD_NUMBER"]
  reviewers    = ["dba-team"]

  access_mode_runbooks = "enabled"
  access_mode_exec     = "enabled"
  access_mode_connect  = "enabled"
  access_schema        = "enabled"

  tags = {
    environment = "development"
    type        = "database"
  }
}

resource "hoop_plugin_connection" "postgres_dev_runbooks" {
  plugin_name   = "runbooks"
  connection_id = hoop_connection.postgres_dev.id
  config        = ["ops/"]
}
```

2. Upgrade the provider with the new version

```sh
terraform init -upgrade
```

3. Remove the old resources from the state

```sh
terraform state rm hoop_runbooks_path.postgres_dev_runbooks
```

4. Import the new resource `hoop_plugin_connection.postgres_dev_runbooks` into the state

```sh
# obtain the ID of the connection via cli
CONNECTION_ID=$(hoop admin get conn postgres_dev -o json |jq -r '.id')
terraform import hoop_plugin_connection.postgres_dev_runbooks runbooks/$CONNECTION_ID
```

5. Apply the changes

```sh
terraform apply
```

> *It's important to note that the apply may present changes to some attributes to update the local state. It should be safe to apply these changes as they are related to the new required fields.*
