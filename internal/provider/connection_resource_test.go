// Copyright (c) HashiCorp, Inc.

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

func createFakeConnectionTestServer() clientFunc {
	client, _ := createFakeConnectionTestServerWithStore()
	return client
}

// createFakeConnectionTestServerWithStore also returns the store, so a test
// can change a connection the way another resource would.
func createFakeConnectionTestServerWithStore() (clientFunc, map[string]*hoop.Connection) {
	store := map[string]*hoop.Connection{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodPost:
			var reqConn hoop.Connection
			if err := json.NewDecoder(req.Body).Decode(&reqConn); err != nil {
				return httpTestErr(http.StatusInternalServerError, `unable to decode  request, reason: %v`, err), nil
			}
			if _, ok := store[reqConn.Name]; ok {
				return httpTestErr(http.StatusConflict, `connection with name %q already exists`, reqConn.Name), nil
			}
			reqConn.ID, _ = uuid.GenerateUUID()
			store[reqConn.Name] = &reqConn
			return httpTestOk(http.StatusCreated, reqConn), nil
		case http.MethodDelete:
			if _, ok := store["bash"]; !ok {
				return httpTestErr(http.StatusNotFound, `connection with name bash not found`), nil
			}
			delete(store, "bash")
			return httpTestErr(http.StatusNoContent, ""), nil
		case http.MethodGet:
			if conn, ok := store["bash"]; ok {
				return httpTestOk(http.StatusOK, conn), nil
			}

			return httpTestErr(http.StatusNotFound, `connection with name bash not found`), nil
		case http.MethodPut:
			var reqConn hoop.Connection
			if err := json.NewDecoder(req.Body).Decode(&reqConn); err != nil {
				return httpTestErr(http.StatusInternalServerError, `unable to decode  request, reason: %v`, err), nil
			}
			existentConn, ok := store[reqConn.Name]
			if !ok {
				return httpTestErr(http.StatusNotFound, `connection with name %q not found`, reqConn.Name), nil
			}
			reqConn.ID = existentConn.ID // keep the same ID
			store[reqConn.Name] = &reqConn
			return httpTestOk(http.StatusOK, reqConn), nil
		}
		return httpTestErr(http.StatusInternalServerError, `test: url path not implemented path: %s, method: %s`, req.URL.Path, req.Method), nil
	}), store
}

func TestAccConnectionResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeConnectionTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "bash" {
  name     = "bash"
  type     = "custom"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  command = [
    "/bin/bash",
  ]

  secrets = {
    "envvar:MYENV" = "value"
    "filesystem:MYFILE" = "file-content"
  }

  reviewers = [
    "admin",
  ]

  redact_types = [
    "EMAIL_ADDRESS",
    "PHONE_NUMBER"
  ]

  access_mode_runbooks = "enabled"
  access_mode_exec = "enabled"
  access_mode_connect = "disabled"
  access_schema = "enabled"

  tags = {
    environment = "development"
    type        = "custom"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_connection.bash", "name", "bash"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "type", "custom"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "command.0", "/bin/bash"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "secrets.envvar:MYENV", "value"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "secrets.filesystem:MYFILE", "file-content"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "reviewers.0", "admin"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "redact_types.0", "EMAIL_ADDRESS"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "redact_types.1", "PHONE_NUMBER"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_mode_runbooks", "enabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_mode_exec", "enabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_mode_connect", "disabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_schema", "enabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "tags.environment", "development"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "tags.type", "custom"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_connection.bash", "id"),
				),
			},
			// Update Testing
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "bash" {
  name     = "bash"
  type     = "custom"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  command = [
    "/bin/bash",
	"-x"
  ]

  access_mode_runbooks = "disabled"
  access_mode_exec = "enabled"
  access_mode_connect = "disabled"
  access_schema = "enabled"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_connection.bash", "name", "bash"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "type", "custom"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "command.0", "/bin/bash"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "command.1", "-x"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_mode_runbooks", "disabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_mode_exec", "enabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_mode_connect", "disabled"),
					resource.TestCheckResourceAttr("hoop_connection.bash", "access_schema", "enabled"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_connection.bash", "id"),
				),
			},
			{
				ResourceName:                         "hoop_connection.bash",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateId:                        "bash",
			},
		},
	})
}

func TestConnectionResourceRemovingOptionalAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeConnectionTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "bash" {
  name     = "bash"
  type     = "custom"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  subtype = "a"
  jira_issue_template_id = "a"
  reviewers = [ "admin" ]
  redact_types = [ "EMAIL_ADDRESS" ]
  tags = { "environment" = "development" }
  secrets = { "envvar:MYENV" = "value" }
  guardrail_rules = [ "rule1" ]

  access_mode_runbooks = "enabled"
  access_mode_exec = "enabled"
  access_mode_connect = "enabled"
  access_schema = "enabled"
}
`,
			},
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "bash" {
  name     = "bash"
  type     = "custom"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"

  access_mode_runbooks = "enabled"
  access_mode_exec = "enabled"
  access_mode_connect = "enabled"
  access_schema = "enabled"
}
`,
			},
		},
	})
}

// TestConnectionResourceKeepsGuardrailsLinkedElsewhere: hoop_guardrail links
// itself to a connection through connection_ids. A connection that does not
// declare guardrail_rules plans no change for that link, and an update, which
// replaces the whole connection, sends it back instead of dropping it.
func TestConnectionResourceKeepsGuardrailsLinkedElsewhere(t *testing.T) {
	client, store := createFakeConnectionTestServerWithStore()
	config := func(command string) string {
		return `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "bash" {
  name     = "bash"
  type     = "custom"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"
  command  = ["` + command + `"]

  access_mode_runbooks = "enabled"
  access_mode_exec     = "enabled"
  access_mode_connect  = "enabled"
  access_schema        = "enabled"
}
`
	}
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", client)()),
		},
		Steps: []resource.TestStep{
			{Config: config("bash")},
			{
				// Links made elsewhere: a guardrail through hoop_guardrail, an
				// attribute on the attribute itself. Neither is drift.
				PreConfig: func() {
					store["bash"].GuardRailRules = []string{"guardrail-1"}
					store["bash"].Attributes = []string{"pci"}
				},
				Config:   config("bash"),
				PlanOnly: true,
			},
			{
				Config: config("sh"),
				Check: func(*terraform.State) error {
					if got := store["bash"].GuardRailRules; len(got) != 1 || got[0] != "guardrail-1" {
						return fmt.Errorf("guardrail_rules after the update: got %v, want [guardrail-1]", got)
					}
					// The resource has no attributes attribute, and the
					// gateway reads an absent key as none.
					if got := store["bash"].Attributes; len(got) != 1 || got[0] != "pci" {
						return fmt.Errorf("attributes after the update: got %v, want [pci]", got)
					}
					return nil
				},
			},
		},
	})
}
