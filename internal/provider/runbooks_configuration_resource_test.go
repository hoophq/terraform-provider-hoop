package provider

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

func createFakeRunbookConfigurationTestServer() clientFunc {
	store := map[string]*hoop.RunbookRepo{}
	return clientFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		// POST /api/runbooks/configurations endpoint
		case http.MethodPost:
			var resource hoop.RunbookRepo
			if err := json.NewDecoder(req.Body).Decode(&resource); err != nil {
				return httpTestErr(http.StatusBadRequest, `test: unable to decode request body: %v`, err), nil
			}
			resourceID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(resource.GitURL)).String()
			if _, ok := store[resourceID]; ok {
				return httpTestErr(http.StatusConflict, `runbook configuration with git url %q already exists`, resource.GitURL), nil
			}
			resource.Repository = "fake-git-url-normalization" // no need to check this value for this tests
			store[resourceID] = &resource
			return httpTestOk(http.StatusCreated, &resource), nil
		// GET /api/runbooks/configurations endpoint
		case http.MethodGet:
			var resource hoop.RunbookConfig
			resource.ID = uuid.NewString() // not used
			// it will not preserve order, it may fail with tests with multiple resources
			for _, repo := range store {
				resource.Repositories = append(resource.Repositories, *repo)
			}
			return httpTestOk(http.StatusOK, &resource), nil
		// PUT /api/datamasking-rules/{id} endpoint
		case http.MethodPut:
			var resource hoop.RunbookRepo
			if err := json.NewDecoder(req.Body).Decode(&resource); err != nil {
				return httpTestErr(http.StatusBadRequest, `test: unable to decode request body: %v`, err), nil
			}
			resourceID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(resource.GitURL)).String()
			existentRepo, ok := store[resourceID]
			if !ok {
				return httpTestErr(http.StatusNotFound, `runbook repository %q not found`, resource.GitURL), nil
			}
			if existentRepo != nil {
				existentID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(existentRepo.GitURL)).String()
				if existentID != resourceID {
					return httpTestErr(http.StatusNotFound, `git url in the path and body do not match, store=%v, body=%v`,
						existentRepo.GitURL, resource.GitURL), nil
				}
			}
			resource.Repository = "fake-git-url-normalization" // no need to check this value for this tests
			store[resourceID] = &resource
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

func TestRunbooksConfigurationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", createFakeRunbookConfigurationTestServer())()),
		},
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_runbook_configuration" "repo" {
  git_url         = "https://github.com/hoophq/runbooks.git"
  git_hook_ttl    = 122
  git_user        = "gituser"
  git_password    = "gitpwd"
  ssh_user        = "sshuser"
  ssh_key         = "sshkey"
  ssh_keypass     = "sshkeypass"
  ssh_known_hosts = "ssh-known-hosts-file"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_url", "https://github.com/hoophq/runbooks.git"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_hook_ttl", "122"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_user", "gituser"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_password", "gitpwd"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_user", "sshuser"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_key", "sshkey"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_keypass", "sshkeypass"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_known_hosts", "ssh-known-hosts-file"),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_runbook_configuration.repo", "repository"),
				),
			},
			// Update Testing
			{
				Config: `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_runbook_configuration" "repo" {
  git_url         = "https://github.com/hoophq/runbooks.git"
  git_hook_ttl    = 0
  git_user        = "gituser"
  git_password    = "gitpwd"
  ssh_user        = ""
  ssh_key         = ""
  ssh_keypass     = ""
  ssh_known_hosts = ""
}
						`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_url", "https://github.com/hoophq/runbooks.git"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_hook_ttl", "0"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_user", "gituser"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "git_password", "gitpwd"),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_user", ""),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_key", ""),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_keypass", ""),
					resource.TestCheckResourceAttr("hoop_runbook_configuration.repo", "ssh_known_hosts", ""),

					// Verify dynamic values have any value set in the state.
					resource.TestCheckResourceAttrSet("hoop_runbook_configuration.repo", "repository"),
				),
			},
			// ImportState testing
			{
				ResourceName:                         "hoop_runbook_configuration.repo",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "git_url",
				ImportStateId:                        "https://github.com/hoophq/runbooks.git",
			},
		},
	})
}
