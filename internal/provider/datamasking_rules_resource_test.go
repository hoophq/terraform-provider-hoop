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

func createFakeDataMaskingRulesTestServer() clientFunc {
	store := map[string]*hoop.DataMaskingRule{}
	resourceID := "c2f81d5c-8d08-4416-9205-4b88993c6ce7"
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		// POST /api/datamasking-rules endpoint

		case http.MethodPost:
			var resource hoop.DataMaskingRule
			if err := json.NewDecoder(req.Body).Decode(&resource); err != nil {
				return httpTestErr(http.StatusBadRequest, `test: unable to decode request body: %v`, err), nil
			}
			if _, ok := store[resource.Name]; ok {
				return httpTestErr(http.StatusConflict, `datamasking rule with name %q already exists`, resource.Name), nil
			}
			resource.ID = resourceID
			store[resource.ID] = &resource
			return httpTestOk(http.StatusCreated, resource), nil
		// GET /api/datamasking-rules/{id} endpoint
		case http.MethodGet:
			parts := strings.Split(req.URL.Path, "/")
			id := parts[len(parts)-1]
			rule, ok := store[id]
			if !ok {
				return httpTestErr(http.StatusNotFound, `datamasking rule with id %q not found`, id), nil
			}
			return httpTestOk(http.StatusOK, rule), nil
		// PUT /api/datamasking-rules/{id} endpoint
		case http.MethodPut:
			var resource hoop.DataMaskingRule
			if err := json.NewDecoder(req.Body).Decode(&resource); err != nil {
				return httpTestErr(http.StatusBadRequest, `test: unable to decode request body: %v`, err), nil
			}
			if _, ok := store[resource.ID]; !ok {
				return httpTestErr(http.StatusNotFound, `datamasking rule with id %q not found`, resource.ID), nil
			}
			store[resource.ID] = &resource
			return httpTestOk(http.StatusOK, resource), nil

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

func TestDataMaskingRulesResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeDataMaskingRulesTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_datamasking_rules" "rule1" {
  name                   = "Example Rule 1"
  description            = "This is an example datamasking rule 1."
  score_threshold        = 0.5
  connection_ids         = ["c2f81d5c-8d08-4416-9205-4b88993c6ce7"]
  custom_entity_types    = []
  supported_entity_types = [
    {
      name         = "PII"
      entity_types = [
        "EMAIL_ADDRESS",
        "PHONE_NUMBER"
      ]
    },
    {
      name         = "PII-2"
      entity_types = [
        "PERSON",
        "URL"
      ]
    }
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "name", "Example Rule 1"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "description", "This is an example datamasking rule 1."),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "score_threshold", "0.5"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "connection_ids.0", "c2f81d5c-8d08-4416-9205-4b88993c6ce7"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.0.name", "PII"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.0.entity_types.0", "EMAIL_ADDRESS"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.0.entity_types.1", "PHONE_NUMBER"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.1.name", "PII-2"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.1.entity_types.0", "PERSON"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.1.entity_types.1", "URL"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_datamasking_rules.rule1", "id"),
				),
			},
			// Update Testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_datamasking_rules" "rule1" {
  name                   = "Example Rule 1"
  description            = "This is an example datamasking rule 1."
  score_threshold        = 0.91
  connection_ids         = ["c2f81d5c-8d08-4416-9205-4b88993c6ce7"]
  custom_entity_types    = []
  supported_entity_types = [
    {
      name         = "PII"
      entity_types = [
        "EMAIL_ADDRESS",
        "URL"
      ]
    },
    {
      name         = "PII-2"
      entity_types = [
        "PERSON",
		"CREDIT_CARD"
      ]
    }
  ]
}
			`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "name", "Example Rule 1"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "description", "This is an example datamasking rule 1."),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "score_threshold", "0.91"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "connection_ids.0", "c2f81d5c-8d08-4416-9205-4b88993c6ce7"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.0.name", "PII"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.0.entity_types.0", "EMAIL_ADDRESS"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.0.entity_types.1", "URL"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.1.name", "PII-2"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.1.entity_types.0", "PERSON"),
					resource.TestCheckResourceAttr("hoop_datamasking_rules.rule1", "supported_entity_types.1.entity_types.1", "CREDIT_CARD"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_datamasking_rules.rule1", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "hoop_datamasking_rules.rule1",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "c2f81d5c-8d08-4416-9205-4b88993c6ce7",
			},
		},
	})
}
