// Copyright (c) HashiCorp, Inc.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const orgAPIKey = "hpk_dGVzdC1vcmdhbml6YXRpb24tYXBpLWtleQ"

func userConfig(providerBlock string) string {
	return providerBlock + `
resource "hoop_user" "john-hoop-dev" {
  email  = "john@hoop.dev"
  status = "active"
  groups = ["engineering"]
}
`
}

func runUserStep(t *testing.T, transport clientFunc, config string) {
	t.Helper()
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", transport)()),
		},
		Steps: []resource.TestStep{{
			Config: config,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "email", "john@hoop.dev"),
			),
		}},
	})
}

// TestOrganizationApiKeyUsesBearerToken is the end-to-end regression test for
// EVL-156. An hpk_ key used to be sent in the Api-Key header, which the gateway
// rejects with 401 before it ever reaches the bearer branch of its auth
// middleware.
func TestOrganizationApiKeyUsesBearerToken(t *testing.T) {
	transport := requireAuthHeaders(t, "Authorization", "Bearer "+orgAPIKey, createFakeUserTestServer())
	runUserStep(t, transport, userConfig(fmt.Sprintf(`
provider "hoop" {
  api_key = %q
  api_url = "http://localhost:8009/api"
}
`, orgAPIKey)))
}

func TestLegacyApiKeyKeepsTheApiKeyHeader(t *testing.T) {
	transport := requireAuthHeaders(t, "Api-Key", "xapi-hash", createFakeUserTestServer())
	runUserStep(t, transport, userConfig(`
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}
`))
}

func TestAuthSchemeOverridesTheDetectedScheme(t *testing.T) {
	t.Run("api_key forces the legacy header", func(t *testing.T) {
		transport := requireAuthHeaders(t, "Api-Key", orgAPIKey, createFakeUserTestServer())
		runUserStep(t, transport, userConfig(fmt.Sprintf(`
provider "hoop" {
  api_key     = %q
  api_url     = "http://localhost:8009/api"
  auth_scheme = "api_key"
}
`, orgAPIKey)))
	})

	t.Run("bearer forces a bearer token", func(t *testing.T) {
		transport := requireAuthHeaders(t, "Authorization", "Bearer xapi-hash", createFakeUserTestServer())
		runUserStep(t, transport, userConfig(`
provider "hoop" {
  api_key     = "xapi-hash"
  api_url     = "http://localhost:8009/api"
  auth_scheme = "bearer"
}
`))
	})

	t.Run("auto is the documented default", func(t *testing.T) {
		transport := requireAuthHeaders(t, "Authorization", "Bearer "+orgAPIKey, createFakeUserTestServer())
		runUserStep(t, transport, userConfig(fmt.Sprintf(`
provider "hoop" {
  api_key     = %q
  api_url     = "http://localhost:8009/api"
  auth_scheme = "auto"
}
`, orgAPIKey)))
	})
}

func TestAuthSchemeRejectsUnknownValues(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeUserTestServer())()),
		},
		Steps: []resource.TestStep{{
			Config: userConfig(`
provider "hoop" {
  api_key     = "xapi-hash"
  api_url     = "http://localhost:8009/api"
  auth_scheme = "basic"
}
`),
			ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
		}},
	})
}
