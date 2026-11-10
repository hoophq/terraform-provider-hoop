# Copyright (c) HashiCorp, Inc.

variable "oidc_client_secret" {
  type      = string
  sensitive = true
}

# The whole row is written on every apply: an attribute left out here is
# cleared on the gateway.
resource "hoop_server_auth_config" "this" {
  auth_method = "oidc"

  oidc_config = {
    issuer_url    = "https://auth.example.com/oidc"
    client_id     = "hoop"
    client_secret = var.oidc_client_secret
    scopes        = ["groups"]
    audience      = "hoop-gateway"
    groups_claim  = "groups"
  }

  provider_name                  = "Okta"
  webapp_users_management_status = "inactive"
  admin_role_name                = "admin"
  auditor_role_name              = "auditor"
}
