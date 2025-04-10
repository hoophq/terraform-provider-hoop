---
page_title: "Troubleshooting"
subcategory: "Guides"
description: |-
  Common errors and their solutions when using the Hoop Provider.
---

# Troubleshooting the Hoop Provider

This guide helps you diagnose and solve common errors when using the Hoop Provider.

## Common Errors

### Authentication Failures

```
Error: Error creating connection: API returned 401: Unauthorized
```

**Possible causes:**
1. Invalid API key
2. Expired API key
3. Wrong API URL

**Solutions:**
1. Verify your API key in Hoop.dev dashboard
2. Ensure environment variables are correctly set
3. Check API URL format

### Connection Creation Failures

```
Error: Error creating connection: API returned 400: Bad Request
```

**Common causes and solutions:**

1. Invalid connection name
   - Must follow pattern: `^[a-zA-Z0-9_]+(?:[-\.]?[a-zA-Z0-9_]+){2,253}$`
   - Cannot use special characters except `-` and `.`

2. Missing required credentials
   - Each database type needs specific credentials
   - Check the connection resource documentation

3. Invalid agent ID
   - Ensure agent is running
   - Verify agent ID in Hoop dashboard

## Access Group Errors

### Understanding Access Control Implementation

The `hoop_access_group` resource works by manipulating a plugin called "access_control" in the Hoop backend. This plugin manages the association between user groups and connections. When troubleshooting access control errors, keep the following points in mind:

1. **Plugin-Based Implementation**: Behind the scenes, the `hoop_access_group` resource creates or updates the "access_control" plugin with the appropriate configurations. This plugin determines which connections are visible to which groups.

2. **Connection IDs vs Names**: While you specify connection names in your Terraform configuration, the underlying implementation uses connection IDs. This is handled automatically by the provider but can be relevant when debugging errors.

### Common Errors and Solutions

#### Access Group Created but No Access

**Problem**: You've created an access group and associated it with connections, but users in the group still can't access those connections.

**Possible causes and solutions**:

- **User Group Mapping**: Ensure the group name in Terraform matches exactly the group name in your authentication system.
- **Connection Existence**: Verify the connections referenced actually exist. If the provider can't find a connection with the given name, it will log an error but may not fail the Terraform operation.
- **Plugin Status**: Check if the "access_control" plugin is enabled and running correctly in your Hoop instance.

#### Unexpected Loss of Access

**Problem**: After updating an access group, users lose access to connections they previously had access to.

**Possible causes and solutions**:

- **Resource Updates**: When updating an access_group resource, make sure you include ALL connections the group should have access to, not just new ones.
- **Multiple Group Memberships**: Remember that a user can belong to multiple groups. If access is removed in one group but exists in another, the user may still have access.

#### Terraform State Inconsistencies

**Problem**: Terraform state doesn't match the actual state in Hoop.

**Possible causes and solutions**:

- **External Changes**: If changes were made directly in the Hoop UI or API outside of Terraform, the Terraform state will be out of sync.
- **Resolution**: Use `terraform refresh` to update the state or `terraform import` to bring existing resources under management.

### Debugging Techniques

1. **Check Provider Logs**: Enable detailed logging to see API interactions:
   ```bash
   export TF_LOG=DEBUG
   export TF_LOG_PATH=terraform.log
   ```

2. **Verify Plugin Configuration**: You can check the access_control plugin directly through the Hoop API:
   ```bash
   curl -H "Api-Key: your-api-key" https://your-hoop-instance/api/plugins/access_control
   ```

3. **Check User Associations**: Verify that users are correctly associated with groups in your authentication system.

## Debugging

### Enable Verbose Logging

The Hoop provider uses Terraform's structured logging system to provide detailed information for troubleshooting. To enable logging, set the following environment variables:

```bash
# Set log level - options: TRACE, DEBUG, INFO, WARN, ERROR
export TF_LOG=DEBUG

# Save logs to a file (optional)
export TF_LOG_PATH=terraform.log
```

#### Log Levels

- **TRACE**: Most verbose, includes HTTP request/response details and API payloads
- **DEBUG**: Detailed debugging information, useful for most troubleshooting
- **INFO**: Normal operation information, confirmations of actions
- **WARN**: Warning conditions that might need attention
- **ERROR**: Error conditions that prevented an operation

For the most comprehensive debugging information when reporting errors, use:

```bash
export TF_LOG=TRACE
```

### Structured Log Output

The logs include the following structured information:

- Resource type and name
- API requests details (URLs, headers, method)
- Response status codes and bodies
- Error messages with context
- Changed fields during updates

#### Example Log Output

```
[INFO]  provider.terraform-provider-hoop: Configuring Hoop provider
[DEBUG] provider.terraform-provider-hoop: Creating Hoop client: api_url=http://localhost:8009/api
[INFO]  provider.terraform-provider-hoop: Creating connection resource: resource_type=connection connection_name=test-connection
[DEBUG] provider.terraform-provider-hoop: Validating connection credentials
[DEBUG] provider.terraform-provider-hoop: Processing access mode settings
[TRACE] provider.terraform-provider-hoop: Creating POST request: url=http://localhost:8009/api/connections
[TRACE] provider.terraform-provider-hoop: Sending POST request: url=http://localhost:8009/api/connections
[DEBUG] provider.terraform-provider-hoop: Received API response for create: status_code=201
[INFO]  provider.terraform-provider-hoop: Connection created successfully: name=test-connection
```

### Common Log Messages

1. "Failed to parse response body"
   - Usually indicates API format changes
   - Check provider version compatibility

2. "Connection timeout"
   - Network connectivity errors
   - Firewall rules
   - VPN/proxy settings

3. "API returned error"
   - Check error response details in the log
   - Verify API key and permissions
   - Validate input parameters

## Best Practices

1. Always use variables for sensitive values
2. Test connections in isolation first
3. Use meaningful names and tags
4. Document your configurations

## Getting Help

1. Check [Hoop documentation](https://hoop.dev/docs)
2. Open GitHub issues for bugs
3. Contact Hoop support for account issues

## Provider Version Check

Verify you're running the latest version:

```bash
terraform version
terraform providers
```

Update provider:

```bash
terraform init -upgrade
```
