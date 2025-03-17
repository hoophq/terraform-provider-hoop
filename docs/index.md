---
page_title: "Provider: Hoop"
subcategory: ""
description: |-
  The Hoop provider provides resources to manage database connections in Hoop.dev.
---

# Hoop Provider

The Hoop provider is used to interact with resources supported by [Hoop.dev](https://hoop.dev). The provider allows you to manage database connections with various configurations and security settings.

## Example Usage

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
  # Configure the provider using environment variables:
  # HOOP_API_KEY and HOOP_API_URL
}
```

## Authentication

The provider needs to be configured with proper credentials before it can be used. There are two ways to provide credentials:

### Environment Variables (recommended)

```bash
export HOOP_API_KEY="your-api-key"
export HOOP_API_URL="http://localhost:8009/api"
```

### Provider Configuration

```hcl
provider "hoop" {
  api_key = "your-api-key"
  api_url = "http://localhost:8009/api"
}
```

-> **Note:** We recommend using environment variables to supply credentials to avoid storing sensitive values in your Terraform configuration.

## Provider Arguments

* `api_key` - (Required) API key for authentication with Hoop.dev. Can be set using the `HOOP_API_KEY` environment variable.
* `api_url` - (Required) The URL of your Hoop instance API. Can be set using the `HOOP_API_URL` environment variable.

## Troubleshooting

The Hoop provider includes comprehensive logging capabilities to help debug issues. To enable verbose logging, set:

```bash
# Set logging level (TRACE, DEBUG, INFO, WARN, ERROR)
export TF_LOG=DEBUG

# Optionally save logs to a file
export TF_LOG_PATH=terraform.log
```

For detailed troubleshooting information, please refer to the [Troubleshooting Guide](guides/troubleshooting.md).
