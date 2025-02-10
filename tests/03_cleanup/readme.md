# Cleaning Up Database Connections

This guide demonstrates different scenarios for removing database connections using the Hoop Terraform Provider.

## Getting Started

1. Prepare your environment:
   ```bash
   cd 03_cleanup
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

## Test Scenarios

### 1. Simple Connection Removal
This scenario shows basic cleanup of a connection with no active sessions.

```bash
# First create the test connection
terraform plan
terraform apply -target=hoop_connection.simple_db

# Verify initial state:
- Connection appears in Hoop UI
- Test the connection works
- Note the connection ID
```

Remove the connection:
```bash
# Option 1: Using terraform destroy
terraform destroy -target=hoop_connection.simple_db

# Option 2: Remove from configuration and apply
# Comment out or delete the resource in connections.tf
terraform apply

# Verify cleanup:
- Connection no longer appears in Hoop UI
- Attempts to connect should fail
- Check for any lingering references
```

### 2. Cleanup with Review Process
Shows how to handle removal when review process is enabled.

```bash
# Create connection with review process
terraform apply -target=hoop_connection.review_db

# Verify initial state:
- Connection exists with review process
- Review groups are configured
- Test review process works
```

Before removal:
1. Check for pending reviews
2. Notify review groups
3. Close active reviews

```bash
# Remove connection
terraform destroy -target=hoop_connection.review_db

# Verify cleanup:
- Connection is removed
- Review processes are cleaned up
- Review groups are properly notified
```

### 3. Cleanup with Security Features
Demonstrates removing a connection with enhanced security.

```bash
# Create secure connection
terraform apply -target=hoop_connection.secure_db

# Verify initial state:
- Data masking is active
- Guardrails are in place
- Security features are working
```

Cleanup process:
```bash
# First, disable new connections
# Edit in connections.tf to disable access modes
resource "hoop_connection" "secure_db" {
  # ... existing config ...
  access_mode {
    runbook = false
    web     = false
    native  = false
  }
}

terraform apply -target=hoop_connection.secure_db

# Then remove the connection
terraform destroy -target=hoop_connection.secure_db

# Verify:
- Connection is removed
- Security configurations are cleaned up
- No lingering masked data references
```

## Best Practices

1. Pre-removal Checklist:
   - Check for active sessions
   - Verify pending reviews
   - Document removal reason
   - Notify affected users
   - Backup connection configuration

2. Cleanup Order:
   - Disable new connections
   - Handle active sessions
   - Clear security features
   - Remove connection
   - Verify cleanup

3. Post-removal Verification:
   - Confirm connection removed
   - Check for orphaned resources
   - Verify access removed
   - Update documentation

## Cleanup Verification

Complete verification checklist:
```bash
# 1. Check Hoop UI
- Connection should not appear
- No active sessions listed
- No pending reviews

# 2. Test Access
- Connection attempts should fail
- Review processes should be gone
- Security features should be cleared

# 3. Check Resources
- No lingering configurations
- No orphaned security settings
- No remaining access grants
```

## Reference: Complete Cleanup Process

1. Prepare for removal:
   ```hcl
   # Disable access first
   resource "hoop_connection" "example" {
     # ... existing config ...
     access_mode {
       runbook = false
       web     = false
       native  = false
     }
   }
   ```

2. Apply access changes:
   ```bash
   terraform apply
   ```

3. Wait for sessions to end:
   - Monitor active sessions
   - Notify users if needed

4. Remove connection:
   ```bash
   terraform destroy
   ```

5. Verify cleanup:
   - Check all resources
   - Test access points
   - Verify security cleanup

## Next Steps

- Document your cleanup procedures
- Create cleanup automation scripts
- Implement cleanup monitoring
- Setup cleanup notifications
