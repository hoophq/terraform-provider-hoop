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

func createFakeUserTestServer() clientFunc {
	store := map[string]*hoop.User{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {

		// POST /api/ endpoint
		case http.MethodPost:
			var user hoop.User
			if err := json.NewDecoder(req.Body).Decode(&user); err != nil {
				return httpTestErr(http.StatusInternalServerError, `unable to decode request, reason: %v`, err), nil
			}
			if _, ok := store[user.Email]; ok {
				return httpTestErr(http.StatusConflict, `user with email %q already exists`, user.Email), nil
			}
			user.ID, _ = uuid.GenerateUUID()
			store[user.Email] = &user
			return httpTestOk(http.StatusCreated, user), nil
		// GET /api/plugins/{plugin_name} endpoint
		case http.MethodGet:
			parts := strings.Split(req.URL.Path, "/")
			userEmail := parts[len(parts)-1]
			usr, ok := store[userEmail]
			if !ok {
				return httpTestErr(http.StatusNotFound, `user with email %q not found`, userEmail), nil
			}
			return httpTestOk(http.StatusOK, usr), nil
		// PUT /api/users/{user_email} endpoint
		case http.MethodPut:
			var user hoop.User
			if err := json.NewDecoder(req.Body).Decode(&user); err != nil {
				return httpTestErr(http.StatusInternalServerError, `unable to decode request, reason: %v`, err), nil
			}
			parts := strings.Split(req.URL.Path, "/")
			userEmail := parts[len(parts)-1]
			if _, ok := store[userEmail]; !ok {
				return httpTestErr(http.StatusNotFound, `user with email %q not found`, userEmail), nil
			}
			user.ID = store[userEmail].ID
			store[userEmail] = &user
			return httpTestOk(http.StatusOK, user), nil
		case http.MethodDelete:
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       http.NoBody,
			}, nil
		}

		return httpTestErr(http.StatusInternalServerError, `test: url path not implemented path: %s, method: %s`, req.URL.Path, req.Method), nil
	})
}

func TestUserResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeUserTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_user" "john-hoop-dev" {
  email  = "john@hoop.dev"
  status = "active"
  groups = ["engineering", "devops"]
}

`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "email", "john@hoop.dev"),
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "status", "active"),
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "groups.0", "engineering"),
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "groups.1", "devops"),
				),
			},
			// Update Testing
			{
				Config: `
			provider "hoop" {
				api_key = "xapi-hash"
				api_url = "http://localhost:8009/api"
			}

			resource "hoop_user" "john-hoop-dev" {
			  email  = "john@hoop.dev"
			  status = "inactive"
			  groups = ["engineering", "devops"]
			}

			resource "hoop_user" "billy-hoop-dev" {
			  email  = "billy@hoop.dev"
			  status = "active"
			  groups = ["banking", "finance"]
			}
			`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "email", "john@hoop.dev"),
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "status", "inactive"),
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "groups.0", "engineering"),
					resource.TestCheckResourceAttr("hoop_user.john-hoop-dev", "groups.1", "devops"),
					resource.TestCheckResourceAttr("hoop_user.billy-hoop-dev", "email", "billy@hoop.dev"),
					resource.TestCheckResourceAttr("hoop_user.billy-hoop-dev", "status", "active"),
					resource.TestCheckResourceAttr("hoop_user.billy-hoop-dev", "groups.0", "banking"),
					resource.TestCheckResourceAttr("hoop_user.billy-hoop-dev", "groups.1", "finance"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_user.john-hoop-dev", "id"),
					resource.TestCheckResourceAttrSet("hoop_user.billy-hoop-dev", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:                         "hoop_user.john-hoop-dev",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        "billy@hoop.dev",
				ImportStateVerifyIdentifierAttribute: "email",
			},
		},
	})
}
