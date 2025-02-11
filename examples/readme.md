# Hoop Provider Examples

This repository contains examples demonstrating how to use the Hoop Terraform Provider to manage database connections.

## Prerequisites

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- Access to a Hoop instance
- API key with appropriate permissions (see: https://hoop.dev/docs/learn/api-key-usage)
- A configured Hoop agent

## Getting Started

1. First, follow the [Installation Guide](./INSTALLATION.md) to set up the Hoop provider in your environment.

2. Clone this repository:
```bash
git clone https://github.com/hoophq/terraform-provider-hoop-examples.git
cd terraform-provider-hoop-examples
```

## Examples Structure

Each directory contains independent examples that can be executed separately:

- `01_create/` - Examples of creating different types of database connections
- `02_updates/` - Examples of updating existing connections
- `03_cleanup/` - Examples of cleaning up and removing connections

## Prerequisites

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- Access to a Hoop instance
- API key with appropriate permissions (see: https://hoop.dev/docs/learn/api-key-usage)
- A configured Hoop agent

## Quick Start

Each example directory is independent and follows the same pattern:

1. Change to the example directory:
```bash
cd 01_create/
```

2. Copy and configure variables:
```bash
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
```

3. Initialize and apply:
```bash
terraform init
terraform plan
terraform apply
```

4. Clean up when done:
```bash
terraform destroy
```

## Example Organization

### Creating Connections (01_create)
Shows how to create different types of database connections with various configurations:
- Different database types (PostgreSQL, MySQL, Oracle, etc.)
- Security settings (data masking, review groups)
- Access modes configuration
- Guardrail setup

### Updating Connections (02_updates)
Demonstrates how to update existing connections:
- Credential rotation
- Security configuration changes
- Access mode modifications
- Adding/removing guardrails

### Cleaning Up (03_cleanup)
Shows different scenarios for removing connections:
- Simple removal
- Cleanup with active sessions
- Removal with review processes

## Best Practices

1. **Independent Testing**
   - Each example directory is self-contained
   - Run examples independently
   - Don't try to share state between examples

2. **Safety**
   - Use a development/test environment
   - Never test in production
   - Always clean up after testing (`terraform destroy`)

3. **Customization**
   - Copy and adapt examples for your needs
   - Don't use example configurations directly in production
   - Maintain your own state files

4. **Organization**
   - Keep production configurations separate
   - Use meaningful names and tags
   - Document your adaptations

## Common Tasks

### Testing a New Connection
```bash
cd 01_create/
# Configure variables
terraform apply
# Test connection
terraform destroy
```

### Practicing Updates
```bash
cd 02_updates/
# Configure variables
terraform apply
# Make updates
terraform apply
# Clean up
terraform destroy
```

### Testing Cleanup Scenarios
```bash
cd 03_cleanup/
# Configure variables
terraform apply
# Test cleanup scenarios
terraform destroy
```

## Notes

- Examples use mock credentials and hosts
- Each example directory has its own README with specific details
- Examples demonstrate concepts but aren't production configurations
- Always review security settings before using in production

## Support

For questions and issues:
- Check the Hoop documentation
- Review example-specific READMEs
- Contact Hoop support for production issues
