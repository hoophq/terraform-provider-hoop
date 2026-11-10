# Copyright (c) HashiCorp, Inc.

# A declared block is written as written on every apply, so a change made in
# the web app shows as drift and the next apply puts it back. Removing a
# declared block stops that proxy. postgres_server_config is left out, so it
# stays with the web app and is passed back unchanged.
resource "hoop_server_misc_config" "this" {
  rdp_server_config = {
    listen_address = "0.0.0.0:13389"
  }

  # hosts_key is left out: the gateway generates one when the proxy starts
  # and keeps it across applies.
  ssh_server_config = {
    listen_address = "0.0.0.0:12222"
    trusted_cas    = ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEXAMPLE ca@example.com"]
    user_mapping = {
      cert_attr = "principal"
      user_attr = "email"
    }
  }
}
