// Copyright (c) HashiCorp, Inc.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestSidecarServiceAccountResource(t *testing.T) {
	gw := newFakeRuleGateway("/sidecar-service-accounts", false)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_sidecar_service_account" "gke" {
  name            = "gke-eu"
  issuer          = "https://container.googleapis.com/v1/projects/p/locations/europe-west1/clusters/eu"
  audience        = "https://hoop.example.com"
  claim           = "sub"
  subject_pattern = "system:serviceaccount:*:hoop-sidecar"
  name_template   = "gke-eu-{1}"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "id", "rule-1"),
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "allow_any_subject", "false"),
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "adopt_existing_sidecars", "false"),
					resource.TestCheckNoResourceAttr("hoop_sidecar_service_account.gke", "jwks"),
					checkLastBody(gw.rec, "POST", "/sidecar-service-accounts", map[string]string{
						"claim":                   `"sub"`,
						"jwks":                    "<absent>",
						"allow_any_subject":       `false`,
						"adopt_existing_sidecars": `false`,
					}),
				),
			},
			{
				Config: testProviderBlock + `
resource "hoop_sidecar_service_account" "gke" {
  name                    = "gke-eu"
  issuer                  = "https://container.googleapis.com/v1/projects/p/locations/europe-west1/clusters/eu"
  audience                = "https://hoop.example.com"
  claim                   = "sub"
  subject_pattern         = "*"
  name_template           = "gke-eu-{1}"
  allow_any_subject       = true
  adopt_existing_sidecars = true
  jwks                    = jsonencode({ keys = [{ kty = "RSA", kid = "k1", n = "AQAB", e = "AQAB" }] })
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "allow_any_subject", "true"),
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "adopt_existing_sidecars", "true"),
					checkLastBody(gw.rec, "PUT", "/sidecar-service-accounts/rule-1", map[string]string{
						"subject_pattern": `"*"`,
						"jwks":            `{"keys":[{"kty":"RSA","kid":"k1","n":"AQAB","e":"AQAB"}]}`,
					}),
				),
			},
			{
				Config: testProviderBlock + `
resource "hoop_sidecar_service_account" "gke" {
  name            = "gke-eu"
  issuer          = "https://accounts.google.com"
  audience        = "https://hoop.example.com"
  claim           = "email"
  subject_pattern = "sidecar-*@p.iam.gserviceaccount.com"
  name_template   = "gcp-{1}"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "claim", "email"),
					resource.TestCheckNoResourceAttr("hoop_sidecar_service_account.gke", "jwks"),
					resource.TestCheckResourceAttr("hoop_sidecar_service_account.gke", "allow_any_subject", "false"),
				),
			},
			{
				ResourceName:      "hoop_sidecar_service_account.gke",
				ImportState:       true,
				ImportStateId:     "rule-1",
				ImportStateVerify: true,
			},
		},
	})
}
