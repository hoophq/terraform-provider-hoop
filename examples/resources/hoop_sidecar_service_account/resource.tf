# Copyright (c) HashiCorp, Inc.

# Every pod running the hoop-sidecar service account in any namespace of this
# GKE cluster reaches a sidecar named after its namespace, with no token to
# hand out. The helm chart sets HOOP_SIDECAR_IDENTITY_TYPE=kubernetes.
resource "hoop_sidecar_service_account" "gke_eu" {
  name            = "gke-eu"
  issuer          = "https://container.googleapis.com/v1/projects/my-project/locations/europe-west1/clusters/eu"
  audience        = "https://hoop.example.com"
  claim           = "sub"
  subject_pattern = "system:serviceaccount:*:hoop-sidecar"
  name_template   = "gke-eu-{1}"
}

# A Google service account on GCE, matched by email.
resource "hoop_sidecar_service_account" "gce" {
  name            = "gce-sidecars"
  issuer          = "https://accounts.google.com"
  audience        = "https://hoop.example.com"
  claim           = "email"
  subject_pattern = "sidecar-*@my-project.iam.gserviceaccount.com"
  name_template   = "gce-{1}"

  # Sidecars an admin registered with a token may be adopted by this identity.
  adopt_existing_sidecars = true
}
