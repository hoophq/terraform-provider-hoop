// Copyright (c) HashiCorp, Inc.

package provider

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

func createFakeAgentTestServer() clientFunc {
	store := map[string]*hoop.Agent{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		path := strings.TrimPrefix(req.URL.Path, "/api")
		switch {
		case req.Method == http.MethodPost && path == "/agents":
			var payload hoop.AgentCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				return httpTestErr(http.StatusInternalServerError, "unable to decode request, reason: %v", err), nil
			}
			if _, ok := store[payload.Name]; ok {
				return httpTestErr(http.StatusConflict, "agent already exists"), nil
			}
			id := uuid.New().String()
			store[payload.Name] = &hoop.Agent{
				ID:     id,
				Name:   payload.Name,
				Mode:   payload.Mode,
				Status: "DISCONNECTED",
			}
			return httpTestOk(http.StatusCreated, hoop.AgentCreateResponse{
				Token: "grpcs://" + payload.Name + ":secret@gateway.example:443?mode=" + payload.Mode,
			}), nil

		case req.Method == http.MethodGet && strings.HasPrefix(path, "/agents/"):
			nameOrID := strings.TrimPrefix(path, "/agents/")
			for _, agent := range store {
				if agent.Name == nameOrID || agent.ID == nameOrID {
					copyAgent := *agent
					copyAgent.Token = ""
					return httpTestOk(http.StatusOK, copyAgent), nil
				}
			}
			return httpTestErr(http.StatusNotFound, "agent not found"), nil

		case req.Method == http.MethodDelete && strings.HasPrefix(path, "/agents/"):
			nameOrID := strings.TrimPrefix(path, "/agents/")
			for name, agent := range store {
				if agent.Name == nameOrID || agent.ID == nameOrID {
					delete(store, name)
					return httpTestErr(http.StatusNoContent, ""), nil
				}
			}
			return httpTestErr(http.StatusNotFound, "agent not found"), nil
		}
		return httpTestErr(http.StatusInternalServerError, "test: url path not implemented path: %s, method: %s", req.URL.Path, req.Method), nil
	})
}

func TestAccAgentResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeAgentTestServer())()),
		},
		Steps: []resource.TestStep{
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_agent" "test" {
  name = "terraform-agent"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_agent.test", "name", "terraform-agent"),
					resource.TestCheckResourceAttr("hoop_agent.test", "mode", "standard"),
					resource.TestCheckResourceAttr("hoop_agent.test", "status", "DISCONNECTED"),
					resource.TestCheckResourceAttrSet("hoop_agent.test", "id"),
					resource.TestCheckResourceAttrSet("hoop_agent.test", "token"),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_agent.test", "name", "terraform-agent"),
					resource.TestCheckResourceAttrSet("hoop_agent.test", "token"),
				),
			},
			{
				ResourceName:            "hoop_agent.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token"},
			},
		},
	})
}

func TestAgentResourceRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"ab", "bad name", "bad--name", "bad..name"} {
		t.Run(name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"hoop": providerserver.NewProtocol6WithError(New("test", createFakeAgentTestServer())()),
				},
				Steps: []resource.TestStep{{
					Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_agent" "test" {
  name = "` + name + `"
}
`,
					ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
				}},
			})
		})
	}
}

func TestAgentResourceDeleteAlreadyAbsent(t *testing.T) {
	base := createFakeAgentTestServer()
	server := clientFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/api/agents/") {
			return httpTestErr(http.StatusNotFound, "agent not found"), nil
		}
		return base(req)
	})

	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", server)()),
		},
		Steps: []resource.TestStep{{
			Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_agent" "test" {
  name = "terraform-agent"
}
`,
		}},
	})
}

func TestAgentResourceEmbeddedMode(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeAgentTestServer())()),
		},
		Steps: []resource.TestStep{
			{
				Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_agent" "test" {
  name = "terraform-agent-embedded"
  mode = "embedded"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_agent.test", "mode", "embedded"),
				),
			},
		},
	})
}
