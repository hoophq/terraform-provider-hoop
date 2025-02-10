# Updating Database Connections

This guide demonstrates how to update existing database connections using the Hoop Terraform Provider.

## Getting Started

1. Prepare your environment:
   ```bash
   cd 02_updates
   cp terraform.tfvars.example terraform.tfvars

   # Edit terraform.tfvars with your values:
   # - Add your Hoop API key (see: https://hoop.dev/docs/learn/api-key-usage)
   # - Set your agent ID
   # - Configure database hosts
   # - Set your test credentials
   ```

2. Initialize Terraform:
   ```bash
   terraform init
   ```

## Test Scenarios

### 1. Security Updates
This scenario shows how to enhance a basic connection with security features.

```bash
# Create initial basic connection
terraform plan
terraform apply -target=hoop_connection.basic_to_secure

# Verify initial state:
- Check connection in Hoop UI
- Confirm no data masking is active
- Note current SSL mode
```

Now update the security settings:
```hcl
# Edit in connections.tf
resource "hoop_connection" "basic_to_secure" {
  # ... existing config ...
  
  datamasking = true
  redact_types = [
    "EMAIL_ADDRESS",
    "CREDIT_CARD_NUMBER"
  ]
  
  secrets = {
    # ... other settings ...
    sslmode = "verify-full"
  }
}
```

```bash
# Apply the changes
terraform plan    # Review security changes
terraform apply -target=hoop_connection.basic_to_secure

# Verify updates:
- Query sensitive data to confirm masking
- Check SSL mode in connection details
- Try accessing masked data
```

### 2. Access Mode Updates
Demonstrates changing how users can access the connection.

```bash
# Deploy initial access configuration
terraform apply -target=hoop_connection.access_update

# Verify initial state:
- All access modes should be enabled
- No review groups configured
```

Update access settings:
```hcl
# Edit in connections.tf
resource "hoop_connection" "access_update" {
  # ... existing config ...
  
  access_mode {
    runbook = true
    web     = true
    native  = false    # Disable native access
  }
  
  review_groups = ["dba-team"]
}
```

```bash
# Apply changes
terraform plan    # Review access changes
terraform apply -target=hoop_connection.access_update

# Verify updates:
- Try connecting via native client (should fail)
- Check review process is triggered
- Verify runbook access still works
```

### 3. Credential Rotation
Shows how to safely update connection credentials.

```bash
# Deploy initial connection
terraform apply -target=hoop_connection.credential_update

# Verify current access:
- Test connection with initial credentials
- Note any active sessions
```

Update credentials:
```hcl
# Edit in connections.tf
resource "hoop_connection" "credential_update" {
  # ... existing config ...
  
  secrets = {
    host = var.database_hosts.mysql.host
    port = var.database_hosts.mysql.port
    user = var.db_credentials.updated.user
    pass = var.db_credentials.updated.pass
    db   = "credential_db"
  }
}
```

```bash
# Apply credential update
terraform plan    # Review credential changes
terraform apply -target=hoop_connection.credential_update

# Verify updates:
- Test connection with new credentials
- Check if existing sessions were handled properly
- Verify access with old credentials is denied
```

## Best Practices

1. Always test updates in development first
2. Update one component at a time
3. Verify changes before and after
4. Have rollback plan for credential updates
5. Document all security changes
6. Consider active sessions when updating

## Cleanup

Remove test connections:
```bash
# Remove single connection
terraform destroy -target=hoop_connection.basic_to_secure

# Remove all test connections
terraform destroy
```

## Reference Configurations

### Security Update Example
```hcl
# Before
resource "hoop_connection" "example" {
  name     = "test-db"
  subtype  = "postgres"
  agent_id = var.agent_id
  
  secrets = {
    host = "localhost"
    port = "5432"
    user = "app_user"
    pass = "password123"
    db   = "testdb"
  }
}

# After
resource "hoop_connection" "example" {
  name     = "test-db"
  subtype  = "postgres"
  agent_id = var.agent_id
  
  secrets = {
    host     = "localhost"
    port     = "5432"
    user     = "app_user"
    pass     = "password123"
    db       = "testdb"
    sslmode  = "verify-full"
  }
  
  datamasking = true
  redact_types = ["EMAIL_ADDRESS"]
  review_groups = ["dba-team"]
}
```

## Next Steps

- Try cleanup scenarios in 03_cleanup
- Adapt these update patterns for your environment
- Document your own update procedures
