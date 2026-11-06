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

// fakeMiscGateway serves /serverconfig/misc the way the gateway does: one
// row that a PUT replaces, and a hosts key generated when the SSH proxy
// starts without one.
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
			PostgresServerConfig: &hoop.ServerProxyConfig{ListenAddress: "0.0.0.0:15432"},
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
		for _, block := range []*hoop.ServerProxyConfig{in.PostgresServerConfig, in.RDPServerConfig, in.HttpProxyServerConfig} {
			if block != nil && block.ListenAddress != "" && !strings.Contains(block.ListenAddress, ":") {
				return httpTestErr(http.StatusBadRequest, "invalid attribute for 'listen_address', it must be in the format 'ip:port'"), nil
			}
		}
		if ssh := in.SSHServerConfig; ssh != nil {
			if ssh.ListenAddress != "" && !strings.Contains(ssh.ListenAddress, ":") {
				return httpTestErr(http.StatusBadRequest, "invalid attribute for 'listen_address', it must be in the format 'ip:port'"), nil
			}
			if len(ssh.TrustedCAs) > 0 && ssh.UserMapping == nil {
				return httpTestErr(http.StatusBadRequest, "user_mapping is required when trusted_cas is configured"), nil
			}
			if ssh.ListenAddress != "" && ssh.HostsKey == "" {
				ssh.HostsKey = "fake-hosts-key"
			}
		}
		g.row = in
		return httpTestOk(http.StatusOK, g.row), nil
	}
	return httpTestErr(http.StatusMethodNotAllowed, "test: %s not allowed", req.Method), nil
}

const testMiscRdpAndSshConfig = testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  rdp_server_config = {
    listen_address = "0.0.0.0:13389"
  }
  ssh_server_config = {
    listen_address = "0.0.0.0:12222"
  }
}`

const testMiscRdpAndSshCertConfig = testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  rdp_server_config = {
    listen_address = "0.0.0.0:13389"
  }
  ssh_server_config = {
    listen_address = "0.0.0.0:12222"
    trusted_cas    = ["ssh-ed25519 AAAA ca@example.com"]
    user_mapping = {
      cert_attr = "principal"
      user_attr = "email"
    }
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
			// Destroy stops the declared proxy; the block it never declared
			// keeps the web app's value.
			gw.mu.Lock()
			defer gw.mu.Unlock()
			if rdp := gw.row.RDPServerConfig; rdp == nil || rdp.ListenAddress != "" {
				return fmt.Errorf("rdp proxy still runs after destroy: %+v", rdp)
			}
			if pg := gw.row.PostgresServerConfig; pg == nil || pg.ListenAddress != "0.0.0.0:25432" {
				return fmt.Errorf("postgres block changed by destroy: %+v", pg)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testMiscRdpAndSshConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "id", "server"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "rdp_server_config.listen_address", "0.0.0.0:13389"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.listen_address", "0.0.0.0:12222"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.hosts_key", "fake-hosts-key"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "ssh_server_config.trusted_cas"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "ssh_server_config.user_mapping"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "postgres_server_config"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "http_proxy_server_config"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"grpc_server_url":          `"grpc://127.0.0.1:8010"`,
						"rdp_server_config":        `{"listen_address":"0.0.0.0:13389"}`,
						"ssh_server_config":        `{"listen_address":"0.0.0.0:12222"}`,
						"postgres_server_config":   `{"listen_address":"0.0.0.0:15432"}`,
						"http_proxy_server_config": `null`,
					}),
				),
			},
			{
				// The web app changes the block Terraform never declared:
				// nothing to plan.
				PreConfig: func() {
					gw.mu.Lock()
					defer gw.mu.Unlock()
					gw.row.PostgresServerConfig = &hoop.ServerProxyConfig{ListenAddress: "0.0.0.0:25432"}
				},
				Config: testMiscRdpAndSshConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// The web app changes the declared rdp block: the plan puts
				// it back, the stored hosts key goes out again and the web
				// app's postgres value rides along untouched.
				PreConfig: func() {
					gw.mu.Lock()
					defer gw.mu.Unlock()
					gw.row.RDPServerConfig = &hoop.ServerProxyConfig{ListenAddress: "0.0.0.0:23389"}
				},
				Config: testMiscRdpAndSshConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("hoop_server_misc_config.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "rdp_server_config.listen_address", "0.0.0.0:13389"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"rdp_server_config":        `{"listen_address":"0.0.0.0:13389"}`,
						"ssh_server_config":        `{"listen_address":"0.0.0.0:12222","hosts_key":"fake-hosts-key"}`,
						"postgres_server_config":   `{"listen_address":"0.0.0.0:25432"}`,
						"http_proxy_server_config": `null`,
					}),
				),
			},
			{
				// Certificate authentication added to the ssh block: the
				// list and the mapping go out, the stored hosts key stays.
				Config: testMiscRdpAndSshCertConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.hosts_key", "fake-hosts-key"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.trusted_cas.#", "1"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.user_mapping.cert_attr", "principal"),
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.user_mapping.user_attr", "email"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"rdp_server_config": `{"listen_address":"0.0.0.0:13389"}`,
						"ssh_server_config": `{"listen_address":"0.0.0.0:12222","hosts_key":"fake-hosts-key",` +
							`"trusted_cas":["ssh-ed25519 AAAA ca@example.com"],"user_mapping":{"cert_attr":"principal","user_attr":"email"}}`,
						"postgres_server_config": `{"listen_address":"0.0.0.0:25432"}`,
					}),
				),
			},
			{
				// The import reads every block the gateway holds, the
				// undeclared postgres one included: the gateway cannot say
				// which ones Terraform owns.
				ResourceName:            "hoop_server_misc_config.this",
				ImportState:             true,
				ImportStateId:           "anything",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"postgres_server_config"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported state, got %d", len(states))
					}
					if got := states[0].Attributes["postgres_server_config.listen_address"]; got != "0.0.0.0:25432" {
						return fmt.Errorf("import did not read postgres_server_config in: %q", got)
					}
					if got := states[0].Attributes["ssh_server_config.hosts_key"]; got != "fake-hosts-key" {
						return fmt.Errorf("import did not read ssh_server_config.hosts_key in: %q", got)
					}
					return nil
				},
			},
			{
				// ssh_server_config left out: the proxy stops.
				Config: testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  rdp_server_config = {
    listen_address = "0.0.0.0:13389"
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "ssh_server_config"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"rdp_server_config":        `{"listen_address":"0.0.0.0:13389"}`,
						"ssh_server_config":        `{"listen_address":""}`,
						"postgres_server_config":   `{"listen_address":"0.0.0.0:25432"}`,
						"http_proxy_server_config": `null`,
					}),
				),
			},
		},
	})
}

// TestServerMiscConfigResourceStartsStoppedSsh starts an SSH proxy declared
// stopped: the gateway answers no hosts key while stopped and generates one
// on start, so the stored state must not pin the empty key.
func TestServerMiscConfigResourceStartsStoppedSsh(t *testing.T) {
	gw := newFakeMiscGateway()
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  ssh_server_config = {}
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "ssh_server_config.listen_address"),
					resource.TestCheckNoResourceAttr("hoop_server_misc_config.this", "ssh_server_config.hosts_key"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"ssh_server_config": `{"listen_address":""}`,
					}),
				),
			},
			{
				Config: testProviderBlock + `
resource "hoop_server_misc_config" "this" {
  ssh_server_config = {
    listen_address = "0.0.0.0:12222"
  }
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_misc_config.this", "ssh_server_config.hosts_key", "fake-hosts-key"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/misc", map[string]string{
						"ssh_server_config": `{"listen_address":"0.0.0.0:12222"}`,
					}),
				),
			},
		},
	})
}

func TestServerMiscConfigResourceRejectsAtPlan(t *testing.T) {
	cases := map[string]struct {
		config string
		want   string
	}{
		"listen_address without a port": {
			config: `
resource "hoop_server_misc_config" "this" {
  rdp_server_config = {
    listen_address = "13389"
  }
}`,
			want: `must be .host:port.`,
		},
		"trusted_cas without user_mapping": {
			config: `
resource "hoop_server_misc_config" "this" {
  ssh_server_config = {
    listen_address = "0.0.0.0:12222"
    trusted_cas    = ["ssh-ed25519 AAAA"]
  }
}`,
			want: `SSH User Mapping Required`,
		},
		"unknown cert_attr": {
			config: `
resource "hoop_server_misc_config" "this" {
  ssh_server_config = {
    listen_address = "0.0.0.0:12222"
    trusted_cas    = ["ssh-ed25519 AAAA"]
    user_mapping = {
      cert_attr = "x"
      user_attr = "email"
    }
  }
}`,
			want: `cert_attr value must be one of`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gw := newFakeMiscGateway()
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest: true,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
				},
				Steps: []resource.TestStep{{
					Config:      testProviderBlock + tc.config,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}
