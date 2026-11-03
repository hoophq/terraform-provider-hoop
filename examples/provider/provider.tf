# Copyright (c) HashiCorp, Inc.

# Configuration-based authentication.
#
# Use an organization API key created under Settings > API Keys. The key must be
# granted the admin group to manage the resources of this provider.
#
# api_url is the REST API root: the gateway serves it under /api, so keep that
# suffix (https://gateway.example.com/api).
provider "hoop" {
  api_key = "<hpk_...>"
  api_url = "http://localhost:8009/api"
}
