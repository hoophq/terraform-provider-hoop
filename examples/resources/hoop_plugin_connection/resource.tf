resource "hoop_plugin_connection" "access_control" {
  plugin_name   = "access_control"
  connection_id = "5001a4a4-9cba-4f2a-9147-d763cd070e0a"
  config        = ["devops", "sre"]
}