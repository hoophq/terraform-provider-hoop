// Copyright (c) HashiCorp, Inc.

package provider

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// TestImportSidecarConnectionFails checks that a connection that mirrors a
// sidecar listener cannot come under Terraform management (ENG-532).
func TestImportSidecarConnectionFails(t *testing.T) {
	managedBy := connectionManagedBySidecar
	transport := clientFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Errorf("unexpected %s %s: the provider must not change a sidecar connection", req.Method, req.URL.Path)
			return httpTestErr(http.StatusForbidden, "connection is managed by a sidecar"), nil
		}
		return httpTestOk(http.StatusOK, hoop.Connection{
			ID: "1", Name: "pay-appdb", Type: "database", SubType: "postgres", ManagedBy: &managedBy,
		}), nil
	})
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", transport)()),
		},
		Steps: []resource.TestStep{{
			Config: `
provider "hoop" {
  api_key = "orgid|hash"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "pay" {
  name     = "pay-appdb"
  type     = "database"
  subtype  = "postgres"
  agent_id = "75122bce-f957-49eb-a812-2ab60977cd9f"
}
`,
			ResourceName:  "hoop_connection.pay",
			ImportState:   true,
			ImportStateId: "pay-appdb",
			ExpectError:   regexp.MustCompile(`mirrors a sidecar listener`),
		}},
	})
}

// TestRefuseSidecarConnection checks the Update and Delete guard, which runs
// when Terraform skips the refresh. It must fail closed.
func TestRefuseSidecarConnection(t *testing.T) {
	sidecar := connectionManagedBySidecar
	for _, tc := range []struct {
		name       string
		get        *http.Response
		wantRefuse bool
	}{
		{"sidecar connection", httpTestOk(http.StatusOK, hoop.Connection{Name: "c", ManagedBy: &sidecar}), true},
		{"lookup fails", httpTestErr(http.StatusInternalServerError, "boom"), true},
		{"gateway connection", httpTestOk(http.StatusOK, hoop.Connection{Name: "c"}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := hoop.NewClient("http://localhost:8009/api", "orgid|hash", hoop.ClientOptions{
				HttpClient: clientFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodGet {
						t.Fatalf("unexpected %s: the guard must only read", req.Method)
					}
					return tc.get, nil
				}),
			})
			if err != nil {
				t.Fatal(err)
			}
			var diags diag.Diagnostics
			r := &connectionResource{client: client}
			if got := r.refuseSidecarConnection(&diags, "c"); got != tc.wantRefuse || diags.HasError() != tc.wantRefuse {
				t.Fatalf("refuse = %v, has error = %v; want %v", got, diags.HasError(), tc.wantRefuse)
			}
		})
	}
}
