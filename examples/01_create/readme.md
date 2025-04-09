# Creating Database Connections

This guide will walk you through creating different types of database connections using the Hoop Terraform Provider.

## Getting Started

1. Prepare your environment:
   ```bash
   cd 01_create
   cp terraform.tfvars.example terraform.tfvars
   
   # Edit terraform.tfvars with your values:
   # - Add your Hoop API key (see: https://hoop.dev/docs/learn/api-key-usage)
   # - Set your agent ID
   # - Configure database hosts
   ```

2. Initialize Terraform:
   ```bash
   terraform init
   ```

## Testing Different Connection Types

### 1. Basic PostgreSQL Connection
```bash
# Edit connections.tf, keep only the basic_postgres connection
terraform plan
terraform apply -target=hoop_connection.basic_postgres

# Verify the connection:
- Check if connection appears in Hoop UI
- Try connecting using psql or another client
- Confirm SSL settings if configured
```

### 2. Secure MySQL Connection
```bash
# Edit connections.tf, enable data masking for mysql_secure
terraform apply -target=hoop_connection.secure_mysql

# Verify security settings:
- Check if data masking is active by querying sensitive data
- Confirm access modes work as expected
- Try accessing without proper review if review_groups is set
```

### 3. Oracle with Enterprise Settings
```bash
# Configure Oracle connection with SID
terraform apply -target=hoop_connection.enterprise_oracle

# Validation steps:
- Connect using SQLPlus to verify SID configuration
- Check if environment variables are set correctly
- Verify SSL/TLS settings if enabled
```

### 4. MongoDB Connection
```bash
# Deploy MongoDB connection
terraform apply -target=hoop_connection.secure_mongodb

# Verify:
- Test connection string format
- Confirm replica set configuration if used
- Check authentication method
```

## Working with Access Groups

When creating multiple access groups that reference the same connections, it's important to use `depends_on` to prevent race conditions:

```bash
# Create access groups with proper dependencies
terraform apply -target=hoop_access_group.dev_team
terraform apply -target=hoop_access_group.db_admins
terraform apply -target=hoop_access_group.analytics
```

### Important Note on Race Conditions

The Hoop access_control plugin processes updates asynchronously. When creating multiple access groups for the same connections in quick succession, you might encounter a race condition where one group's configuration overwrites another. Always use `depends_on` between access group resources to establish a clear creation order:

```hcl
resource "hoop_access_group" "second_group" {
  # ... configuration ...
  
  # This prevents race conditions by ensuring the first group is fully processed
  depends_on = [hoop_access_group.first_group]
}
```

## Cleanup Options

1. Remove all connections:
   ```bash
   terraform destroy
   ```

2. Remove specific connection:
   ```bash
   terraform destroy -target=hoop_connection.basic_postgres
   ```

## Example Configurations

### Basic Connection
```hcl
resource "hoop_connection" "basic_postgres" {
  name     = "basic-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host = var.database_hosts.postgres.host
    port = "5432"
    user = "app_user"
    pass = "password123"  # Use variables in production
    db   = "appdb"
  }
}
```

### Secure Connection
```hcl
resource "hoop_connection" "secure_postgres" {
  name     = "secure-db"
  subtype  = "postgres"
  agent_id = var.agent_id

  secrets = {
    host     = var.database_hosts.postgres.host
    port     = "5432"
    user     = "app_user"
    pass     = "password123"
    db       = "securedb"
    sslmode  = "verify-full"
  }

  datamasking = true
  redact_types = ["EMAIL_ADDRESS"]
  review_groups = ["dba-team"]
  
  access_mode {
    runbook = true
    web     = false
    native  = false
  }
}
```

## Best Practices

1. Always use variables for sensitive data
2. Start with basic connection before adding security features
3. Test each security feature individually
4. Use meaningful connection names
5. Tag connections appropriately
6. Document custom configurations
7. Use `depends_on` between access_group resources to prevent race conditions

## Next Steps

- Try updating these connections in the 02_updates example
- Test cleanup scenarios in 03_cleanup
- Adapt these examples for your production environment
