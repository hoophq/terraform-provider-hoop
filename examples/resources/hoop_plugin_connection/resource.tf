# Copyright (c) HashiCorp, Inc.

# manage which user groups can access this connection
resource "hoop_plugin_connection" "access_control" {
  plugin_name   = "access_control"
  connection_id = "5001a4a4-9cba-4f2a-9147-d763cd070e0a"
  config        = ["devops", "sre"]
}

# manage Slack configuration and which channel to send messages to
resource "hoop_plugin_connection" "slack" {
  plugin_name   = "slack"
  connection_id = "5001a4a4-9cba-4f2a-9147-d763cd070e0a"
  config        = ["C082KCG5NJU"]
}

# manage runbooks configuration and which folder to display
resource "hoop_plugin_connection" "runbooks" {
  plugin_name   = "runbooks"
  connection_id = "5001a4a4-9cba-4f2a-9147-d763cd070e0a"
  config        = ["ops/"]
}

# when webhooks are configured, sends events when interacting with this connection
resource "hoop_plugin_connection" "webhooks" {
  plugin_name   = "webhooks"
  connection_id = "5001a4a4-9cba-4f2a-9147-d763cd070e0a"
  config        = []
}