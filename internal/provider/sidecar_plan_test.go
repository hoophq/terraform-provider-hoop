// Copyright (c) HashiCorp, Inc.

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

func sidecarFactories(gw *fakeSidecarGateway) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
	}
}

const sidecarWithListener = `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    listeners = [{ name = "%s", protocol = "%s", listen = "0.0.0.0:15432", upstream = "db:5432" }]
  })
}`

func sidecarListenerConfig(name, protocol string) string {
	return testProviderBlock + fmt.Sprintf(sidecarWithListener, name, protocol)
}

// TestSidecarResourceRefusesBreakingBoundListener covers the plan guard: a
// rule bound to a listener blocks a rename and a protocol change of that
// listener, and rebinding it elsewhere lifts the block.
func TestSidecarResourceRefusesBreakingBoundListener(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config: sidecarListenerConfig("appdb", "postgres"),
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "0"),
			},
			// The binding arrives from the rule API, between runs. The next
			// refresh reports it and the unchanged configuration plans nothing.
			{
				PreConfig: func() {
					gw.bind(hoop.SidecarRuleBinding{Kind: "guardrail", Name: "no-delete", ListenerName: "appdb"})
				},
				Config: sidecarListenerConfig("appdb", "postgres"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "1"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.0.name", "no-delete"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.0.listener_name", "appdb"),
				),
			},
			{
				Config:      sidecarListenerConfig("appdb-v2", "postgres"),
				ExpectError: regexp.MustCompile(`Listener Change Breaks Bound Rules[\s\S]*guardrail "no-delete" is bound to listener "appdb"[\s\S]*removes\s+or renames`),
			},
			{
				Config:      sidecarListenerConfig("appdb", "mysql"),
				ExpectError: regexp.MustCompile(`guardrail "no-delete" is bound to listener "appdb"[\s\S]*moves\s+from postgres to\s+mysql`),
			},
			// The rule moved to another sidecar (its sidecar_targets were
			// applied first): nothing binds appdb and the rename goes through.
			{
				PreConfig: func() {
					gw.mu.Lock()
					gw.byID["sc-1"].BoundRules = nil
					gw.mu.Unlock()
				},
				Config: sidecarListenerConfig("appdb", "postgres"),
			},
			{
				Config: sidecarListenerConfig("appdb-v2", "postgres"),
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "0"),
			},
		},
	})
}

// TestSidecarResourceIgnoresZeroValues covers the configuration type: a
// document with zero-valued leaves reads back from the gateway without them
// and plans no change.
func TestSidecarResourceIgnoresZeroValues(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	config := testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    log_level = ""
    audit     = {}
    admin     = { listen = "0.0.0.0:19000", tls = false, max_conns = 0 }
    listeners = [
      { name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432", tls = {}, guardrails = { rules = [] } }
    ]
  })
}`
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					// The wire carries what the operator wrote; the gateway prunes.
					checkLastBody(gw.rec, "POST", "/sidecars", map[string]string{
						"configuration": `{"log_level":"","audit":{},"admin":{"listen":"0.0.0.0:19000","tls":false,"max_conns":0},"listeners":[{"name":"appdb","protocol":"postgres","listen":"0.0.0.0:15432","upstream":"db:5432","tls":{},"guardrails":{"rules":[]}}]}`,
					}),
				),
			},
			// Same configuration again: the refresh reads the pruned document
			// and the plan must be empty, or this step fails.
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func TestSidecarResourceRefusesWithoutLicense(t *testing.T) {
	for _, tt := range []struct {
		name    string
		license fakeLicense
		want    string
	}{
		{"oss", fakeLicense{set: true, typ: "oss", valid: true, status: "valid"}, `license is of type "oss"`},
		{"expired", fakeLicense{set: true, typ: "enterprise", valid: false, status: "expired"}, `Enterprise license is expired`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gw := newFakeSidecarGateway(t)
			gw.license = tt.license
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest:               true,
				ProtoV6ProviderFactories: sidecarFactories(gw),
				Steps: []resource.TestStep{
					{
						Config:      sidecarListenerConfig("appdb", "postgres"),
						ExpectError: regexp.MustCompile(`Enterprise License Required[\s\S]*` + regexp.QuoteMeta(tt.want)),
					},
				},
			})
		})
	}
}

func TestSidecarServiceAccountResourceRefusesWithoutLicense(t *testing.T) {
	gw := newFakeRuleGateway("/sidecar-service-accounts", false)
	gw.license = fakeLicense{set: true, typ: "oss", valid: true, status: "valid"}
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_sidecar_service_account" "gke" {
  name            = "gke-eu"
  issuer          = "https://container.googleapis.com/v1/projects/p/locations/europe-west1/clusters/eu"
  audience        = "https://hoop.example.com"
  claim           = "sub"
  subject_pattern = "system:serviceaccount:*:hoop-sidecar"
  name_template   = "gke-eu-{1}"
}`,
				ExpectError: regexp.MustCompile(`hoop_sidecar_service_account needs a valid Enterprise license`),
			},
		},
	})
}

func TestPruneZero(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{"empty strings and zero numbers go", `{"a":"","b":0,"c":"x"}`, `{"c":"x"}`},
		{"false goes, true stays", `{"a":false,"b":true}`, `{"b":true}`},
		{"empty object goes", `{"a":{},"b":{"c":""}}`, `{}`},
		{"empty list goes", `{"a":[]}`, `{}`},
		{"rules keeps its empty list", `{"guardrails":{"rules":[]}}`, `{"guardrails":{"rules":[]}}`},
		{"list items are pruned, not removed", `{"a":[{"x":""},{"y":1}]}`, `{"a":[{},{"y":1}]}`},
		{"null goes", `{"a":null,"b":1}`, `{"b":1}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := canonicalSidecarConfig([]byte(tt.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestWarnOnUnbind(t *testing.T) {
	ctx := t.Context()
	targets := func(ids ...string) types.List {
		items := make([]sidecarTargetModel, 0, len(ids))
		for _, id := range ids {
			items = append(items, sidecarTargetModel{SidecarID: types.StringValue(id), ListenerName: types.StringValue("appdb")})
		}
		v, d := types.ListValueFrom(ctx, sidecarTargetObjectType, items)
		if d.HasError() {
			t.Fatalf("building list: %v", d)
		}
		return v
	}
	null := types.ListNull(sidecarTargetObjectType)
	for _, tt := range []struct {
		name        string
		plan, prior types.List
		want        string
	}{
		{"declared then removed warns with the counts", null, targets("sc-1", "sc-1", "sc-2"), "3 listener(s) on 2 sidecar(s)"},
		{"never declared says nothing", null, null, ""},
		{"still declared says nothing", targets("sc-1"), targets("sc-1"), ""},
		{"declared and now empty says nothing", targets(), targets("sc-1"), ""},
		{"removed with no prior targets says nothing", null, targets(), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			warnOnUnbind(ctx, "hoop_guardrail", "no-delete", tt.plan, tt.prior, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if tt.want == "" {
				if diags.WarningsCount() != 0 {
					t.Fatalf("expected no warning, got %v", diags)
				}
				return
			}
			if diags.WarningsCount() != 1 {
				t.Fatalf("expected one warning, got %v", diags)
			}
			if got := diags[0].Detail(); !strings.Contains(got, tt.want) {
				t.Errorf("warning %q does not mention %q", got, tt.want)
			}
		})
	}
}

// TestJsonencodeForm: a document as Postgres jsonb renders it comes out the way
// jsonencode writes it, so an imported value plans no whitespace-only change.
// jsonencode escapes <, > and & as \u003c, \u003e and \u0026, as Go does.
func TestJsonencodeForm(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{`{"listeners": [{"name": "appdb", "max_conns": 5}], "admin": {"listen": "127.0.0.1:19000"}}`,
			`{"admin":{"listen":"127.0.0.1:19000"},"listeners":[{"max_conns":5,"name":"appdb"}]}`},
		{`{"keys": [{"n": "a<b&c>d", "e": "AQAB", "big": 12345678901234567890}]}`,
			"{\"keys\":[{\"big\":12345678901234567890,\"e\":\"AQAB\",\"n\":\"a\\u003cb\\u0026c\\u003ed\"}]}"},
		{`not json`, `not json`},
	} {
		if got := jsonencodeForm([]byte(tt.in)); got != tt.want {
			t.Errorf("jsonencodeForm(%s):\n got  %s\n want %s", tt.in, got, tt.want)
		}
	}
}
