// Copyright (c) HashiCorp, Inc.

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// The blocks the web app owns, as the gateway answers them. The SSH block
// carries every sub-field so a write that drops one is caught.
const (
	testMiscPostgresBlock = `{"listen_address":"0.0.0.0:15432"}`
	testMiscSshBlock      = `{"listen_address":"0.0.0.0:12222","hosts_key":"fake-hosts-key",` +
		`"trusted_cas":["ssh-ed25519 AAAA ca@example.com"],"user_mapping":{"cert_attr":"principal","user_attr":"email"}}`
)

// fakeMiscGateway serves /serverconfig/misc the way the gateway does: one
// row that a PUT replaces.
type fakeMiscGateway struct {
	rec *requestRecorder
	mu  sync.Mutex
	row hoop.ServerMiscConfig
}

func newFakeMiscGateway() *fakeMiscGateway {
	return &fakeMiscGateway{
		rec: &requestRecorder{},
		row: hoop.ServerMiscConfig{
			GrpcServerURL:        "grpc://127.0.0.1:8010",
			PostgresServerConfig: json.RawMessage(testMiscPostgresBlock),
			SSHServerConfig:      json.RawMessage(testMiscSshBlock),
		},
	}
}

func (g *fakeMiscGateway) handle(req *http.Request) (*http.Response, error) {
	body := g.rec.record(req)
	g.mu.Lock()
	defer g.mu.Unlock()
	if !strings.HasSuffix(req.URL.Path, "/serverconfig/misc") {
		return httpTestErr(http.StatusInternalServerError, "test: not implemented: %s %s", req.Method, req.URL.Path), nil
	}
	switch req.Method {
	case http.MethodGet:
		return httpTestOk(http.StatusOK, g.row), nil
	case http.MethodPut:
		var in hoop.ServerMiscConfig
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		if h := in.HttpProxyServerConfig; h != nil && h.ListenAddress != "" && !strings.Contains(h.ListenAddress, ":") {
			return httpTestErr(http.StatusBadRequest, "invalid attribute for 'listen_address', it must be in the format 'ip:port'"), nil
		}
		g.row = in
		return httpTestOk(http.StatusOK, g.row), nil
	}
	return httpTestErr(http.StatusMethodNotAllowed, "test: %s not allowed", req.Method), nil
}

const testMiscHttpProxyConfig = testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  http_proxy_server_config = {
    listen_address = "0.0.0.0:18888"
  }
}`

func TestServerMiscConfigResource(t *testing.T) {
	gw := newFakeMiscGateway()
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		CheckDestroy: func(*terraform.State) error {
			// Destroy stops the HTTP proxy; the blocks the web app owns keep
			// their values.
			gw.mu.Lock()
			defer gw.mu.Unlock()
			if h := gw.row.HttpProxyServerConfig; h == nil || h.ListenAddress != "" {
				return fmt.Errorf("http proxy still runs after destroy: %+v", h)
			}
			if pg := gw.row.PostgresServerConfig; !jsonEqual(pg, json.RawMessage(`{"listen_address":"0.0.0.0:25432"}`)) {
				return fmt.Errorf("postgres block changed by destroy: %s", pg)
			}
			if ssh := gw.row.SSHServerConfig; !jsonEqual(ssh, json.RawMessage(testMiscSshBlock)) {
				return fmt.Errorf("ssh block changed by destroy: %s", ssh)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testMiscHttpProxyConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "id", "server"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "http_proxy_server_config.listen_address", "0.0.0.0:18888"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "postgres_server_config"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "ssh_server_config"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"grpc_server_url":          `"grpc://127.0.0.1:8010"`,
						"http_proxy_server_config": `{"listen_address":"0.0.0.0:18888"}`,
						"postgres_server_config":   testMiscPostgresBlock,
						"ssh_server_config":        testMiscSshBlock,
						"rdp_server_config":        `null`,
					}),
				),
			},
			{
				// The web app changes a block the provider never manages:
				// nothing to plan.
				PreConfig: func() {
					gw.mu.Lock()
					defer gw.mu.Unlock()
					gw.row.PostgresServerConfig = json.RawMessage(`{"listen_address":"0.0.0.0:25432"}`)
				},
				Config: testMiscHttpProxyConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// The web app changes the managed block: the plan puts it
				// back, and the web app's postgres value rides along
				// untouched.
				PreConfig: func() {
					gw.mu.Lock()
					defer gw.mu.Unlock()
					gw.row.HttpProxyServerConfig = &hoop.ServerProxyConfig{ListenAddress: "0.0.0.0:28888"}
				},
				Config: testMiscHttpProxyConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("hoop_server_misc_config.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "http_proxy_server_config.listen_address", "0.0.0.0:18888"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"http_proxy_server_config": `{"listen_address":"0.0.0.0:18888"}`,
						"postgres_server_config":   `{"listen_address":"0.0.0.0:25432"}`,
						"ssh_server_config":        testMiscSshBlock,
					}),
				),
			},
			{
				// The import reads the block in when the gateway holds it.
				ResourceName:      "hoop_server_misc_config.this",
				ImportState:       true,
				ImportStateId:     "anything",
				ImportStateVerify: true,
			},
			{
				// http_proxy_server_config left out: the proxy stops.
				Config: testProviderBlock + `
resource "hoop_server_misc_config" "this" {
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "http_proxy_server_config"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"http_proxy_server_config": `{"listen_address":""}`,
						"postgres_server_config":   `{"listen_address":"0.0.0.0:25432"}`,
						"ssh_server_config":        testMiscSshBlock,
					}),
				),
			},
			{
				// Declared again, so the destroy that follows has a proxy
				// to stop.
				Config: testMiscHttpProxyConfig,
				Check: checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
					"http_proxy_server_config": `{"listen_address":"0.0.0.0:18888"}`,
				}),
			},
		},
	})
}

func TestServerMiscConfigResourceRejectsAtPlan(t *testing.T) {
	gw := newFakeMiscGateway()
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{{
			Config: testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  http_proxy_server_config = {
    listen_address = "18888"
  }
}`,
			ExpectError: regexp.MustCompile(`must be .host:port.`),
		}},
	})
}
