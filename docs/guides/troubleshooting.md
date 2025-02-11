---
page_title: "Troubleshooting"
subcategory: "Guides"
description: |-
  Common issues and their solutions when using the Hoop Provider.
---

# Troubleshooting the Hoop Provider

This guide helps you diagnose and solve common issues when using the Hoop Provider.

## Common Issues

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

## Debugging

### Enable Detailed Logs

Set the following environment variables:

```bash
export TF_LOG=DEBUG
export TF_LOG_PATH=terraform.log
```

### Common Log Messages

1. "Failed to parse response body"
   - Usually indicates API format changes
   - Check provider version compatibility

2. "Connection timeout"
   - Network connectivity issues
   - Firewall rules
   - VPN/proxy settings

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
