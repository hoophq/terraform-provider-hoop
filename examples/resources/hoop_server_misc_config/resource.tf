# Copyright (c) HashiCorp, Inc.

# The block is written as written on every apply, so a change made in the web
# app shows as drift and the next apply puts it back. The RDP, PostgreSQL and
# SSH listeners stay with the web app and are passed back unchanged.
resource "hoop_server_misc_config" "this" {
  http_proxy_server_config = {
    listen_address = "0.0.0.0:18888"
  }
}
