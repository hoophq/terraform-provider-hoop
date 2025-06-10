// Copyright (c) HashiCorp, Inc.

package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

func createFakePluginConnectionTestServer() clientFunc {
	store := map[string]*hoop.PluginConnection{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPut:
			var pluginConn hoop.PluginConnection
			if err := json.NewDecoder(req.Body).Decode(&pluginConn); err != nil {
				return httpTestErr(http.StatusInternalServerError, `unable to decode request, reason: %v`, err), nil
			}
			urlParts := strings.Split(req.URL.Path, "/")
			pluginConn.ID, _ = uuid.GenerateUUID()
			pluginConn.PluginID, _ = uuid.GenerateUUID()
			pluginConn.ConnectionID = urlParts[len(urlParts)-1]

			store[""] = &pluginConn
			return httpTestOk(http.StatusOK, pluginConn), nil
		case http.MethodGet:
			pluginConn, ok := store[""]
			if !ok {
				return httpTestErr(http.StatusNotFound, `plugin connection not found`), nil
			}
			return httpTestOk(http.StatusOK, pluginConn), nil
		case http.MethodDelete:
			delete(store, "")
			return httpTestErr(http.StatusNoContent, ""), nil
		}
		_ = store
		return httpTestErr(http.StatusInternalServerError, `test: url path not implemented path: %s, method: %s`, req.URL.Path, req.Method), nil
	})
}

func TestPluginConnectionResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakePluginConnectionTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_plugin_connection" "slack" {
  plugin_name = "slack"
  connection_id = "ab13b0b5-b69b-4b6e-8073-765ff7e7ebfa"
  config = ["SLACK-CHANNEL-ID-1", "SLACK-CHANNEL-ID-2"]
}

`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "plugin_name", "slack"),
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "connection_id", "ab13b0b5-b69b-4b6e-8073-765ff7e7ebfa"),
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "config.0", "SLACK-CHANNEL-ID-1"),
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "config.1", "SLACK-CHANNEL-ID-2"),
				),
			},
			// Update Testing
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_plugin_connection" "slack" {
  plugin_name = "slack"
  connection_id = "ab13b0b5-b69b-4b6e-8073-765ff7e7ebfa"
  config = ["SLACK-CHANNEL-ID-3"]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "plugin_name", "slack"),
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "connection_id", "ab13b0b5-b69b-4b6e-8073-765ff7e7ebfa"),
					resource.TestCheckResourceAttr("hoop_plugin_connection.slack", "config.0", "SLACK-CHANNEL-ID-3"),
				),
			},

			// ImportState testing
			{
				ResourceName:                         "hoop_plugin_connection.slack",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        "slack/ab13b0b5-b69b-4b6e-8073-765ff7e7ebfa",
				ImportStateVerifyIdentifierAttribute: "plugin_name",
			},
		},
	})
}
