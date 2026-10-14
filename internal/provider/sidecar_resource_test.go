// Copyright (c) HashiCorp, Inc.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const sidecarConfigV1 = `jsonencode({
    admin     = { listen = "0.0.0.0:19000" }
    listeners = [{ name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" }]
  })`

const sidecarConfigV2 = `jsonencode({
    admin     = { listen = "0.0.0.0:19000" }
    listeners = [{ name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db-replica:5432" }]
  })`

func TestSidecarResource(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name          = "payments"
  configuration = ` + sidecarConfigV1 + `
  slack_channels = {
    appdb = ["C001", "C002"]
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "id", "sc-1"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "token", "hsc_sc-1"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "load_from_disk", "false"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "config_state", "applied"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "version", "1.0.0"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "slack_channels.appdb.0", "C001"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "slack_channels.appdb.1", "C002"),
					// The flag is not in the document the gateway stores.
					checkLastBody(gw.rec, "POST", "/sidecars", map[string]string{
						"name":          `"payments"`,
						"configuration": `{"admin":{"listen":"0.0.0.0:19000"},"listeners":[{"name":"appdb","protocol":"postgres","listen":"0.0.0.0:15432","upstream":"db:5432"}]}`,
					}),
					checkLastBody(gw.rec, "PUT", "/slack-channels", map[string]string{
						"listeners": `[{"name":"appdb","channels":["C001","C002"]}]`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// Switching the owner: the PATCH carries the flag alone and goes
			// out before the PUT, and the PUT carries the flag inside the
			// document so the gateway sees no second flip.
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name           = "payments"
  configuration  = ` + sidecarConfigV2 + `
  load_from_disk = true
  slack_channels = {
    appdb = ["C001", "C002"]
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "load_from_disk", "true"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "token", "hsc_sc-1"),
					checkWrites(gw.rec, "/sidecars/sc-1", "PATCH", "PUT"),
					checkLastBody(gw.rec, "PATCH", "/sidecars/sc-1", map[string]string{
						"configuration": `{"load_from_disk":true}`,
					}),
					checkLastBody(gw.rec, "PUT", "/sidecars/sc-1", map[string]string{
						"configuration": `{"admin":{"listen":"0.0.0.0:19000"},"listeners":[{"name":"appdb","protocol":"postgres","listen":"0.0.0.0:15432","upstream":"db-replica:5432"}],"load_from_disk":true}`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// Dropping slack_channels clears the listener it named, and the
			// attribute goes back to null rather than reporting the gateway.
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name           = "payments"
  configuration  = ` + sidecarConfigV2 + `
  load_from_disk = true
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_sidecar.payments", "slack_channels"),
					checkLastBody(gw.rec, "PUT", "/slack-channels", map[string]string{
						"listeners": `[{"name":"appdb","channels":[]}]`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// Switching back: the flag goes out alone, then the document.
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name          = "payments"
  configuration = ` + sidecarConfigV2 + `
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "load_from_disk", "false"),
					checkWrites(gw.rec, "/sidecars/sc-1", "PATCH", "PUT"),
					checkLastBody(gw.rec, "PATCH", "/sidecars/sc-1", map[string]string{
						"configuration": `{"load_from_disk":false}`,
					}),
					checkLastBody(gw.rec, "PUT", "/sidecars/sc-1", map[string]string{
						"configuration": `{"admin":{"listen":"0.0.0.0:19000"},"listeners":[{"name":"appdb","protocol":"postgres","listen":"0.0.0.0:15432","upstream":"db-replica:5432"}]}`,
					}),
				),
			},
			// Import by name. The token cannot be read back.
			{
				ResourceName:            "hoop_sidecar.payments",
				ImportState:             true,
				ImportStateId:           "payments",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token"},
			},
		},
	})
}

func TestSidecarResourceRefusesLoadFromDiskInConfiguration(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name          = "payments"
  configuration = jsonencode({ load_from_disk = true })
}`,
				ExpectError: regexp.MustCompile(`load_from_disk Belongs in Its Own Attribute`),
			},
		},
	})
}

func TestSidecarDataSource(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name          = "payments"
  configuration = ` + sidecarConfigV1 + `
}

data "hoop_sidecar" "payments" {
  name = hoop_sidecar.payments.name
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hoop_sidecar.payments", "id", "sc-1"),
					resource.TestCheckResourceAttr("data.hoop_sidecar.payments", "config_state", "applied"),
					resource.TestCheckResourceAttr("data.hoop_sidecar.payments", "load_from_disk", "false"),
					resource.TestCheckResourceAttrPair("data.hoop_sidecar.payments", "configuration", "hoop_sidecar.payments", "configuration"),
				),
			},
		},
	})
}
