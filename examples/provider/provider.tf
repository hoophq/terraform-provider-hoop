# Copyright (c) HashiCorp, Inc.

# Configuration-based authentication.
#
# Use an organization API key created under Settings > API Keys. The key must be
# granted the admin group to manage the resources of this provider.
provider "hoop" {
  api_key = "<hpk_...>"
  api_url = "http://localhost:8009/api"
}
