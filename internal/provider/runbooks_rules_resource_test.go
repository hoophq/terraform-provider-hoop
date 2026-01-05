package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var runbookRuleResourceFakeID = "3f3cb338-b812-4e31-b6c2-c79a08cc3221"

func createFakeRunbookRulesTestServer() clientFunc {
	store := map[string]*hoop.RunbookRule{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		// POST /api/runbooks/rules endpoint
		case http.MethodPost:
			var resource hoop.RunbookRule
			if err := json.NewDecoder(req.Body).Decode(&resource); err != nil {
				return httpTestErr(http.StatusBadRequest, `test: unable to decode request body: %v`, err), nil
			}
			resource.ID = runbookRuleResourceFakeID
			store[resource.ID] = &resource
			return httpTestOk(http.StatusCreated, &resource), nil
		// GET /api/runbooks/rules/{id} endpoint
		case http.MethodGet:
			parts := strings.Split(req.URL.Path, "/")
			id := parts[len(parts)-1]
			resource, ok := store[id]
			if !ok {
				return httpTestErr(http.StatusNotFound, `runbook rule with id %q not found`, id), nil
			}
			return httpTestOk(http.StatusOK, resource), nil
		// PUT /api/runbooks/rules/{id} endpoint
		case http.MethodPut:
			var resource hoop.RunbookRule
			if err := json.NewDecoder(req.Body).Decode(&resource); err != nil {
				return httpTestErr(http.StatusBadRequest, `test: unable to decode request body: %v`, err), nil
			}

			// resourceID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(resource.GitURL)).String()
			if _, ok := store[resource.ID]; !ok {
				return httpTestErr(http.StatusNotFound, `runbook rule %q not found`, resource.ID), nil
			}
			store[resource.ID] = &resource
			return httpTestOk(http.StatusOK, &resource), nil
		case http.MethodDelete:
			parts := strings.Split(req.URL.Path, "/")
			id := parts[len(parts)-1]
			delete(store, id)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       http.NoBody,
			}, nil
		}

		return httpTestErr(http.StatusInternalServerError, `test: url path not implemented path: %s, method: %s`, req.URL.Path, req.Method), nil
	})
}

func TestRunbooksRulesResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeRunbookRulesTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_runbook_rule" "myrule" {
  name        = "My Rule"
  description = "My Rule Description"
  connections = ["pgdemo"]
  user_groups = ["developers"]
  runbooks = [
    { 
      repository = "normalized-git-url-repo"
      name       = "postgres-demo/update-customer-email.runbook.sql"
    },
    { 
      repository = "normalized-git-url-repo"
      name       = "postgres-demo/delete-customer-by-id.runbook.sql"
    }
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "name", "My Rule"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "description", "My Rule Description"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "connections.0", "pgdemo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "user_groups.0", "developers"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.0.repository", "normalized-git-url-repo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.0.name", "postgres-demo/update-customer-email.runbook.sql"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.1.repository", "normalized-git-url-repo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.1.name", "postgres-demo/delete-customer-by-id.runbook.sql"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_runbook_rule.myrule", "id"),
				),
			},
			// Update Testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_runbook_rule" "myrule" {
  name        = "My Rule"
  description = "My Rule Description Updated"
  connections = ["pgdemo", "pgprod"]
  user_groups = ["developers", "dba"]
  runbooks = [
    { 
      repository = "normalized-git-url-repo"
      name       = "postgres-demo/run-migration.sql"
    },
    { 
      repository = "normalized-git-url-repo"
      name       = "postgres-demo/delete-customer-by-id.runbook.sql"
    },
	{ 
      repository = "normalized-git-url-repo"
      name       = "postgres-demo/fetch-customer-by-id.runbook.sql"
    }
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "name", "My Rule"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "description", "My Rule Description Updated"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "connections.0", "pgdemo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "connections.1", "pgprod"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "user_groups.0", "developers"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "user_groups.1", "dba"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.0.repository", "normalized-git-url-repo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.0.name", "postgres-demo/run-migration.sql"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.1.repository", "normalized-git-url-repo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.1.name", "postgres-demo/delete-customer-by-id.runbook.sql"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.2.repository", "normalized-git-url-repo"),
					resource.TestCheckResourceAttr("hoop_runbook_rule.myrule", "runbooks.2.name", "postgres-demo/fetch-customer-by-id.runbook.sql"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_runbook_rule.myrule", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:                         "hoop_runbook_rule.myrule",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				ImportStateId:                        runbookRuleResourceFakeID,
			},
		},
	})
}
