# Copyright (c) HashiCorp, Inc.

resource "hoop_user" "john-mydomain-org" {
  email  = "john@mydomain.org"
  status = "active"
  groups = ["engineering", "admin"]
}
