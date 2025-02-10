# Installing Hoop Provider

## Prerequisites
- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- A Hoop instance and API key (see: https://hoop.dev/docs/learn/api-key-usage)

## Provider Installation

The Hoop provider needs to be installed in your system's Terraform plugin directory. Follow the steps below for your operating system:

### Local Installation

#### Linux
```bash
# Create plugin directory
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/hoophq/hoop/1.0.0/linux_amd64

# Download provider
curl -LO https://github.com/hoophq/terraform-provider-hoop/releases/download/v1.0.0/terraform-provider-hoop_1.0.0_linux_amd64.zip

# Extract to plugin directory
unzip terraform-provider-hoop_1.0.0_linux_amd64.zip -d ~/.terraform.d/plugins/registry.terraform.io/hoophq/hoop/1.0.0/linux_amd64
```

#### MacOS
```bash
# Create plugin directory
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/hoophq/hoop/1.0.0/darwin_amd64

# Download provider
curl -LO https://github.com/hoophq/terraform-provider-hoop/releases/download/v1.0.0/terraform-provider-hoop_1.0.0_darwin_amd64.zip

# Extract to plugin directory
unzip terraform-provider-hoop_1.0.0_darwin_amd64.zip -d ~/.terraform.d/plugins/registry.terraform.io/hoophq/hoop/1.0.0/darwin_amd64
```

#### Windows
```powershell
# Create plugin directory
mkdir %APPDATA%\terraform.d\plugins\registry.terraform.io\hoophq\hoop\1.0.0\windows_amd64

# Download and extract the ZIP file to the above directory
# terraform-provider-hoop_1.0.0_windows_amd64.zip
```

### System-wide Installation

#### Linux/MacOS
```bash
# Create system-wide plugin directory
sudo mkdir -p /usr/share/terraform/plugins/registry.terraform.io/hoophq/hoop/1.0.0/linux_amd64  # For Linux
# or
sudo mkdir -p /usr/share/terraform/plugins/registry.terraform.io/hoophq/hoop/1.0.0/darwin_amd64  # For MacOS

# Download and extract
sudo unzip terraform-provider-hoop_1.0.0_<OS>_amd64.zip -d /usr/share/terraform/plugins/registry.terraform.io/hoophq/hoop/1.0.0/<OS>_amd64
```

## Verifying Installation

1. Create a test configuration:
```hcl
# main.tf
terraform {
  required_providers {
    hoop = {
      source = "registry.terraform.io/hoophq/hoop"
      version = "1.0.0"
    }
  }
}

provider "hoop" {
  # See how to get your API key at: https://hoop.dev/docs/learn/api-key-usage
  api_key = "your-api-key"
  api_url = "http://localhost:8009/api"
}
```

2. Initialize Terraform:
```bash
terraform init
```

You should see output like:
```
Initializing provider plugins...
- Finding latest version of hoophq/hoop...
- Installing hoophq/hoop v1.0.0...
```

3. Verify provider installation:
```bash
terraform providers

# Expected output:
# provider[registry.terraform.io/hoophq/hoop]
```

## Common Installation Issues

### Provider Not Found
```
Error: Failed to query available provider packages... 
Provider registry.terraform.io/hoophq/hoop was not found
```

**Solution**: Verify the provider binary is in the correct directory and has the correct permissions.

### Wrong Architecture
```
Error: Incompatible provider version
```

**Solution**: Ensure you downloaded the correct version for your system architecture (amd64, arm64, etc).

### Permission Issues
```
Error: Could not load plugin
```

**Solution**: Check file permissions:
```bash
# For local installation
chmod 755 ~/.terraform.d/plugins/registry.terraform.io/hoophq/hoop/1.0.0/<OS>_<ARCH>/terraform-provider-hoop_v1.0.0

# For system-wide installation
sudo chmod 755 /usr/share/terraform/plugins/registry.terraform.io/hoophq/hoop/1.0.0/<OS>_<ARCH>/terraform-provider-hoop_v1.0.0
```

## Next Steps

After successful installation, you can:
1. Configure your provider with your API key
2. Try the example configurations in this repository
3. Create your own Hoop resources using Terraform
