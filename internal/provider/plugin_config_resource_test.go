package provider

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

func createFakePluginConfigTestServer() clientFunc {
	pluginID := "c2f81d5c-8d08-4416-9205-4b88993c6ce7"
	store := map[string]*hoop.Plugin{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		// PUT /api/plugins/config endpoint
		case http.MethodPut:
			var envVars map[string]string
			if err := json.NewDecoder(req.Body).Decode(&envVars); err != nil {
				return httpTestErr(http.StatusInternalServerError, `unable to decode request, reason: %v`, err), nil
			}
			resource := &hoop.Plugin{
				ID:     pluginID,
				Name:   "slack",
				Config: &hoop.PluginConfig{ID: pluginID, EnvVars: envVars},
			}
			store[""] = resource
			return httpTestOk(http.StatusOK, resource), nil
		// GET /api/plugins/{plugin_name} endpoint
		case http.MethodGet:
			plugin, ok := store[""]
			if !ok {
				return httpTestOk(http.StatusOK, &hoop.Plugin{
					ID:     pluginID,
					Name:   "slack",
					Config: nil,
				}), nil
				// return httpTestErr(http.StatusNotFound, `plugin config not found`), nil
			}
			return httpTestOk(http.StatusOK, plugin), nil
		}
		return httpTestErr(http.StatusInternalServerError, `test: url path not implemented path: %s, method: %s`, req.URL.Path, req.Method), nil
	})
}

func TestPluginConfigResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakePluginConfigTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_plugin_config" "slack" {
  plugin_name = "slack"
  config = {
    SLACK_BOT_TOKEN = "xoxb-2136"
    SLACK_APP_TOKEN = "xapp-1-A08BV"
  }
}

`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_plugin_config.slack", "plugin_name", "slack"),
					resource.TestCheckResourceAttr("hoop_plugin_config.slack", "config.SLACK_BOT_TOKEN", "xoxb-2136"),
					resource.TestCheckResourceAttr("hoop_plugin_config.slack", "config.SLACK_APP_TOKEN", "xapp-1-A08BV"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_plugin_config.slack", "id"),
				),
			},
			// Update Testing
			{
				Config: `
provider "hoop" {
	api_key = "xapi-hash"
	api_url = "http://localhost:8009/api"
}

resource "hoop_plugin_config" "slack" {
	plugin_name = "slack"
	config = {
	SLACK_BOT_TOKEN = "xoxb-222"
	SLACK_APP_TOKEN = "xapp-1-ZZZ"
	}
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_plugin_config.slack", "plugin_name", "slack"),
					resource.TestCheckResourceAttr("hoop_plugin_config.slack", "config.SLACK_BOT_TOKEN", "xoxb-222"),
					resource.TestCheckResourceAttr("hoop_plugin_config.slack", "config.SLACK_APP_TOKEN", "xapp-1-ZZZ"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_plugin_config.slack", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:                         "hoop_plugin_config.slack",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        "slack",
				ImportStateVerifyIdentifierAttribute: "plugin_name",
			},
		},
	})
}
