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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// fakeAuthGateway serves /serverconfig/auth the way the gateway does: one
// row, answered from the environment until the first PUT, and a PUT that
// replaces the row, defaulting groups_claim and the role names.
type fakeAuthGateway struct {
	rec *requestRecorder
	mu  sync.Mutex
	row *hoop.ServerAuthConfig
}

func newFakeAuthGateway() *fakeAuthGateway {
	return &fakeAuthGateway{rec: &requestRecorder{}}
}

func (g *fakeAuthGateway) view() *hoop.ServerAuthConfig {
	if g.row != nil {
		return g.row
	}
	return &hoop.ServerAuthConfig{
		AuthMethod:                  "local",
		WebappUsersManagementStatus: "inactive",
		AdminRoleName:               "admin",
		AuditorRoleName:             "auditor",
	}
}

func (g *fakeAuthGateway) handle(req *http.Request) (*http.Response, error) {
	body := g.rec.record(req)
	g.mu.Lock()
	defer g.mu.Unlock()
	if !strings.HasSuffix(req.URL.Path, "/serverconfig/auth") {
		return httpTestErr(http.StatusInternalServerError, "test: not implemented: %s %s", req.Method, req.URL.Path), nil
	}
	switch req.Method {
	case http.MethodGet:
		return httpTestOk(http.StatusOK, g.view()), nil
	case http.MethodPut:
		var in struct {
			hoop.ServerAuthConfig
			RolloutApiKey *string `json:"rollout_api_key"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		if in.RolloutApiKey != nil {
			return httpTestErr(http.StatusUnprocessableEntity, "rollout api key does match with stored value"), nil
		}
		if in.WebappUsersManagementStatus != "active" && in.WebappUsersManagementStatus != "inactive" {
			return httpTestErr(http.StatusUnprocessableEntity, "invalid attribute for webapp_users_management_status"), nil
		}
		if in.AuthMethod == "oidc" && in.OidcConfig == nil {
			return httpTestErr(http.StatusUnprocessableEntity, "attribute 'oidc_config' is required for OIDC auth method"), nil
		}
		if in.AuthMethod == "saml" && in.SamlConfig == nil {
			return httpTestErr(http.StatusUnprocessableEntity, "attribute 'saml_config' is required for SAML auth method"), nil
		}
		row := in.ServerAuthConfig
		if row.OidcConfig != nil && row.OidcConfig.GroupsClaim == "" {
			row.OidcConfig.GroupsClaim = "groups"
		}
		if row.SamlConfig != nil {
			if row.SamlConfig.GroupsClaim == "" {
				row.SamlConfig.GroupsClaim = "groups"
			}
			row.SamlConfig.ResolvedMetadata = &hoop.ServerAuthSamlResolvedMetadata{
				EntityID:             "https://idp.example.com/metadata",
				SsoURL:               "https://idp.example.com/sso",
				CertificateExpiresAt: "2030-01-01T00:00:00Z",
				ResolvedAt:           "2026-11-04T12:00:00Z",
			}
		}
		if row.AdminRoleName == "" {
			row.AdminRoleName = "admin"
		}
		if row.AuditorRoleName == "" {
			row.AuditorRoleName = "auditor"
		}
		row.ApiKey = nil
		g.row = &row
		return httpTestOk(http.StatusOK, g.row), nil
	}
	return httpTestErr(http.StatusMethodNotAllowed, "test: %s not allowed", req.Method), nil
}

const testAuthOidcConfig = testProviderBlock + `
resource "hoop_server_auth_config" "this" {
  auth_method = "oidc"
  oidc_config = {
    issuer_url    = "https://auth.example.com/oidc"
    client_id     = "hoop"
    client_secret = "s3cret"
    scopes        = ["openid", "groups"]
    audience      = "hoop-gateway"
  }
  provider_name                  = "Okta"
  webapp_users_management_status = "active"
}`

func TestServerAuthConfigResource(t *testing.T) {
	gw := newFakeAuthGateway()
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		CheckDestroy: func(*terraform.State) error {
			// Destroy only forgets the row: the gateway keeps it.
			gw.mu.Lock()
			defer gw.mu.Unlock()
			if gw.row == nil {
				return fmt.Errorf("the gateway row is gone after destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_server_auth_config" "this" {
  auth_method                    = "local"
  webapp_users_management_status = "inactive"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "id", "server"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "auth_method", "local"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "admin_role_name", "admin"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "auditor_role_name", "auditor"),
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "provider_name"),
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "api_key"),
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "oidc_config.issuer_url"),
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "saml_config.idp_metadata_url"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/auth", map[string]string{
						"auth_method":                    `"local"`,
						"oidc_config":                    `null`,
						"saml_config":                    `null`,
						"provider_name":                  `""`,
						"webapp_users_management_status": `"inactive"`,
						"admin_role_name":                `""`,
						"auditor_role_name":              `""`,
						"api_key":                        "<absent>",
						"rollout_api_key":                "<absent>",
					}),
				),
			},
			{
				Config: testAuthOidcConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "auth_method", "oidc"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "provider_name", "Okta"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "oidc_config.groups_claim", "groups"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "oidc_config.audience", "hoop-gateway"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "oidc_config.scopes.#", "2"),
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "oidc_config.client_assertion_file"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/auth", map[string]string{
						"auth_method": `"oidc"`,
						"oidc_config": `{"issuer_url":"https://auth.example.com/oidc","client_id":"hoop","client_secret":"s3cret",` +
							`"scopes":["openid","groups"],"audience":"hoop-gateway","groups_claim":""}`,
						"saml_config":                    `null`,
						"provider_name":                  `"Okta"`,
						"webapp_users_management_status": `"active"`,
						"rollout_api_key":                "<absent>",
					}),
				),
			},
			{
				ResourceName:      "hoop_server_auth_config.this",
				ImportState:       true,
				ImportStateId:     "anything",
				ImportStateVerify: true,
			},
			{
				// audience left out of the configuration: the body sends it
				// empty, so the gateway clears it.
				Config: testProviderBlock + `
resource "hoop_server_auth_config" "this" {
  auth_method = "oidc"
  oidc_config = {
    issuer_url    = "https://auth.example.com/oidc"
    client_id     = "hoop"
    client_secret = "s3cret"
    scopes        = ["openid", "groups"]
  }
  provider_name                  = "Okta"
  webapp_users_management_status = "active"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "oidc_config.audience"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "oidc_config.groups_claim", "groups"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/auth", map[string]string{
						"oidc_config": `{"issuer_url":"https://auth.example.com/oidc","client_id":"hoop","client_secret":"s3cret",` +
							`"scopes":["openid","groups"],"audience":"","groups_claim":""}`,
					}),
				),
			},
			{
				Config: testProviderBlock + `
resource "hoop_server_auth_config" "this" {
  auth_method = "saml"
  saml_config = {
    idp_metadata_url = "https://idp.example.com/metadata.xml"
  }
  webapp_users_management_status = "inactive"
  admin_role_name                = "hoop-admins"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_server_auth_config.this", "oidc_config.issuer_url"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "saml_config.groups_claim", "groups"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "saml_config.resolved_metadata.entity_id", "https://idp.example.com/metadata"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "admin_role_name", "hoop-admins"),
					resource.TestCheckResourceAttr("hoop_server_auth_config.this", "auditor_role_name", "auditor"),
					checkLastBody(gw.rec, "PUT", "/serverconfig/auth", map[string]string{
						"oidc_config":     `null`,
						"saml_config":     `{"idp_metadata_url":"https://idp.example.com/metadata.xml","groups_claim":""}`,
						"admin_role_name": `"hoop-admins"`,
					}),
				),
			},
		},
	})
}

func TestServerAuthConfigResourceRejectsAtPlan(t *testing.T) {
	cases := map[string]struct {
		config string
		want   string
	}{
		"oidc without oidc_config": {
			config: `
resource "hoop_server_auth_config" "this" {
  auth_method                    = "oidc"
  webapp_users_management_status = "active"
}`,
			want: `OIDC Configuration Required`,
		},
		"saml without saml_config": {
			config: `
resource "hoop_server_auth_config" "this" {
  auth_method                    = "saml"
  webapp_users_management_status = "active"
}`,
			want: `SAML Configuration Required`,
		},
		"users management on": {
			config: `
resource "hoop_server_auth_config" "this" {
  auth_method                    = "local"
  webapp_users_management_status = "on"
}`,
			want: `webapp_users_management_status value must be one of`,
		},
		"two client credentials": {
			config: `
resource "hoop_server_auth_config" "this" {
  auth_method = "oidc"
  oidc_config = {
    issuer_url            = "https://auth.example.com/oidc"
    client_id             = "hoop"
    client_secret         = "s3cret"
    client_assertion_file = "/var/run/secrets/hoop/idp/token"
  }
  webapp_users_management_status = "active"
}`,
			want: `Client Credential Conflict`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gw := newFakeAuthGateway()
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
