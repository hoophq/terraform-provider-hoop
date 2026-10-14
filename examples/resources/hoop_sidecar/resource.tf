# Copyright (c) HashiCorp, Inc.

# A sidecar registered with a token. The token lands in state; pass it to the
# sidecar as HOOP_SIDECAR_TOKEN together with HOOP_CONTROL_PLANE_URL.
resource "hoop_sidecar" "payments" {
  name = "payments"

  configuration = jsonencode({
    admin = { listen = "0.0.0.0:19000" }
    audit = { file = "-" }
    listeners = [
      {
        name     = "appdb"
        protocol = "postgres"
        listen   = "0.0.0.0:15432"
        upstream = "appdb.default.svc.cluster.local:5432"
      }
    ]
  })

  # Reviews raised on the appdb listener go to these Slack channels.
  slack_channels = {
    appdb = ["C0123456789"]
  }
}

# The same document kept in a YAML file next to the Terraform code.
resource "hoop_sidecar" "reporting" {
  name          = "reporting"
  configuration = jsonencode(yamldecode(file("${path.module}/reporting-sidecar.yaml")))
}

output "payments_sidecar_token" {
  value     = hoop_sidecar.payments.token
  sensitive = true
}
