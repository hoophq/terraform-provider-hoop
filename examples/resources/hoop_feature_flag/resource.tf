# Copyright (c) HashiCorp, Inc.

# Record sidecar traffic as sessions. Off by default; without it the sidecars
# handshake and enforce, and nothing shows in the session list.
resource "hoop_feature_flag" "sidecar_session_events" {
  name    = "experimental.sidecar_session_events"
  enabled = true
}
