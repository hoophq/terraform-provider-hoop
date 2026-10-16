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

// fakeFlagGateway serves /feature-flags the way the gateway does: the
// catalog is fixed, the organization stores only the flags it changed, and
// an unknown name on PUT is a 400.
type fakeFlagGateway struct {
	rec     *requestRecorder
	mu      sync.Mutex
	catalog []hoop.FeatureFlag
	org     map[string]bool
}

func newFakeFlagGateway() *fakeFlagGateway {
	return &fakeFlagGateway{
		rec: &requestRecorder{},
		catalog: []hoop.FeatureFlag{
			{Name: "experimental.sidecar_session_events", Description: "Record sidecar traffic as sessions", Default: false, Stability: "experimental", Components: []string{"gateway"}},
			{Name: "experimental.sidecar_stream_chunks", Description: "Chunked event storage", Default: true, Stability: "experimental", Components: []string{"gateway"}},
		},
		org: map[string]bool{},
	}
}

func (g *fakeFlagGateway) view(f hoop.FeatureFlag) hoop.FeatureFlag {
	f.Enabled = f.Default
	if v, ok := g.org[f.Name]; ok {
		f.Enabled = v
	}
	return f
}

func (g *fakeFlagGateway) handle(req *http.Request) (*http.Response, error) {
	body := g.rec.record(req)
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/feature-flags"):
		out := make([]hoop.FeatureFlag, 0, len(g.catalog))
		for _, f := range g.catalog {
			out = append(out, g.view(f))
		}
		return httpTestOk(http.StatusOK, out), nil
	case req.Method == http.MethodPut:
		name := lastPathSegment(req.URL.Path)
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		for _, f := range g.catalog {
			if f.Name == name {
				g.org[name] = in.Enabled
				return httpTestOk(http.StatusOK, g.view(f)), nil
			}
		}
		return httpTestErr(http.StatusBadRequest, "unknown feature flag: %s", name), nil
	}
	return httpTestErr(http.StatusInternalServerError, "test: not implemented: %s %s", req.Method, req.URL.Path), nil
}

func TestFeatureFlagResource(t *testing.T) {
	gw := newFakeFlagGateway()
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		CheckDestroy: func(*terraform.State) error {
			// Destroy restores the catalog default: the org row is back to it.
			gw.mu.Lock()
			defer gw.mu.Unlock()
			if v, ok := gw.org["experimental.sidecar_session_events"]; ok && v {
				return fmt.Errorf("experimental.sidecar_session_events still enabled after destroy")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_feature_flag" "session_events" {
  name    = "experimental.sidecar_session_events"
  enabled = true
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_feature_flag.session_events", "enabled", "true"),
					resource.TestCheckResourceAttr("hoop_feature_flag.session_events", "default", "false"),
					resource.TestCheckResourceAttr("hoop_feature_flag.session_events", "stability", "experimental"),
					resource.TestCheckResourceAttr("hoop_feature_flag.session_events", "components.0", "gateway"),
					checkLastBody(gw.rec, "PUT", "/feature-flags/experimental.sidecar_session_events", map[string]string{
						"enabled": `true`,
					}),
				),
			},
			{
				Config: testProviderBlock + `
resource "hoop_feature_flag" "session_events" {
  name    = "experimental.sidecar_session_events"
  enabled = false
}`,
				Check: resource.TestCheckResourceAttr("hoop_feature_flag.session_events", "enabled", "false"),
			},
			{
				ResourceName:                         "hoop_feature_flag.session_events",
				ImportState:                          true,
				ImportStateId:                        "experimental.sidecar_session_events",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				Config: testProviderBlock + `
resource "hoop_feature_flag" "session_events" {
  name    = "experimental.sidecar_session_events"
  enabled = true
}`,
			},
		},
	})
}

func TestFeatureFlagResourceUnknownName(t *testing.T) {
	gw := newFakeFlagGateway()
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{{
			Config: testProviderBlock + `
resource "hoop_feature_flag" "typo" {
  name    = "experimental.does_not_exist"
  enabled = true
}`,
			ExpectError: regexp.MustCompile(`unknown feature flag`),
		}},
	})
}
