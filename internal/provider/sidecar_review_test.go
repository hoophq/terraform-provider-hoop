// Copyright (c) HashiCorp, Inc.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Each test here pins one PR review finding, so the regression it names
// cannot come back without the test saying which one.

// A JSON null is an object to json.Unmarshal into a map, and a nil map to
// write load_from_disk into. Refused at plan.
func TestSidecarResourceRefusesNullConfiguration(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{{
			Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name           = "payments"
  configuration  = "null"
  load_from_disk = true
}`,
			ExpectError: regexp.MustCompile(`got null`),
		}},
	})
}

// A load_from_disk flip detaches the bound rules on the gateway. The plan
// must not refuse the listener change the flip makes harmless, and the
// bound_rules the apply reads back must not contradict the plan.
func TestSidecarResourceFlipWithBoundRules(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config: sidecarListenerConfig("appdb", "postgres"),
			},
			{
				PreConfig: func() {
					gw.bind(hoop.SidecarRuleBinding{Kind: "guardrail", Name: "no-delete", ListenerName: "appdb"})
				},
				Config: sidecarListenerConfig("appdb", "postgres"),
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "1"),
			},
			// Flip and rename in one apply: the guard stands down, the PATCH
			// detaches, and bound_rules ends empty without an
			// "inconsistent result" from Terraform core.
			{
				Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name           = "payments"
  load_from_disk = true
  configuration = jsonencode({
    listeners = [{ name = "appdb-v2", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" }]
  })
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "load_from_disk", "true"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "0"),
					checkWrites(gw.rec, "/sidecars/sc-1", "PATCH", "PUT"),
				),
			},
		},
	})
}

// A read that cannot see the bindings keeps the last known ones, so the
// listener guard still fires.
func TestSidecarResourceKeepsBindingsWhenUnavailable(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config: sidecarListenerConfig("appdb", "postgres"),
			},
			{
				PreConfig: func() {
					gw.bind(hoop.SidecarRuleBinding{Kind: "guardrail", Name: "no-delete", ListenerName: "appdb"})
				},
				Config: sidecarListenerConfig("appdb", "postgres"),
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "1"),
			},
			{
				PreConfig: func() {
					gw.mu.Lock()
					gw.byID["sc-1"].BoundRules = nil
					gw.byID["sc-1"].BoundRulesUnavailable = true
					gw.mu.Unlock()
				},
				Config:      sidecarListenerConfig("appdb-v2", "postgres"),
				ExpectError: regexp.MustCompile(`Listener Change Breaks Bound Rules`),
			},
		},
	})
}

// Bindings change outside the sidecar resource between plan and apply: a
// rule bound to it destroyed earlier in the same apply, or another workspace
// binding one. The update reads back what the gateway holds, and the plan
// must not have promised the old bindings, or Terraform core fails the apply
// with "Provider produced inconsistent result after apply".
func TestSidecarResourceBindingsChangeDuringApply(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	withReports := testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    listeners = [
      { name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" },
      { name = "reports", protocol = "postgres", listen = "0.0.0.0:15433", upstream = "reports:5432" },
    ]
  })
}`
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config: sidecarListenerConfig("appdb", "postgres"),
			},
			{
				PreConfig: func() {
					gw.bind(hoop.SidecarRuleBinding{Kind: "guardrail", Name: "no-delete", ListenerName: "appdb"})
				},
				Config: sidecarListenerConfig("appdb", "postgres"),
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "1"),
			},
			// The binding goes away after the plan read it.
			{
				PreConfig: func() {
					gw.mu.Lock()
					gw.beforePut = func(sc *hoop.Sidecar) { sc.BoundRules = nil }
					gw.mu.Unlock()
				},
				Config: withReports,
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "0"),
			},
			// A binding arrives after the plan read none.
			{
				PreConfig: func() {
					gw.mu.Lock()
					gw.beforePut = func(sc *hoop.Sidecar) {
						sc.BoundRules = []hoop.SidecarRuleBinding{{Kind: "datamasking", Name: "emails", ListenerName: "appdb"}}
					}
					gw.mu.Unlock()
				},
				Config: sidecarListenerConfig("appdb", "postgres"),
				Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "bound_rules.#", "1"),
			},
		},
	})
}

// Removing a listener that has Slack channels: the gateway prunes them with
// the configuration write and refuses an entry for a listener it no longer
// has, so the provider must not send one.
func TestSidecarResourceRemovesListenerWithChannels(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	two := testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    listeners = [
      { name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" },
      { name = "reports", protocol = "postgres", listen = "0.0.0.0:15433", upstream = "reports:5432" },
    ]
  })
  slack_channels = { appdb = ["C001"], reports = ["C002"] }
}`
	one := testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    listeners = [{ name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" }]
  })
  slack_channels = { appdb = ["C001"] }
}`
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config: two,
				Check:  resetRecorder(gw.rec),
			},
			{
				Config: one,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "slack_channels.%", "1"),
					checkLastBody(gw.rec, "PUT", "/slack-channels", map[string]string{
						"listeners": `[{"name":"appdb","channels":["C001"]}]`,
					}),
				),
			},
		},
	})
}

// A channel write that fails after the sidecar is created rolls the sidecar
// back, so the next apply creates it whole instead of replacing a tainted one
// and rotating its token.
func TestSidecarResourceRollsBackOnChannelFailure(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	gw.failSlack = true
	config := testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    listeners = [{ name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" }]
  })
  slack_channels = { appdb = ["C001"] }
}`
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`The sidecar was removed again`),
			},
			// The second create gets a fresh id: the first one is gone.
			{
				PreConfig: func() { gw.failSlack = false },
				Config:    config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "id", "sc-2"),
					resource.TestCheckResourceAttr("hoop_sidecar.payments", "token", "hsc_sc-2"),
				),
			},
		},
	})
}

func TestSidecarResourceRefusesBadSlackChannels(t *testing.T) {
	for _, tt := range []struct {
		name, channels, want string
	}{
		{"empty channel list", `{ appdb = [] }`, `at least 1`},
		{"unknown listener", `{ nope = ["C001"] }`, `Slack Channels Name No Listener`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gw := newFakeSidecarGateway(t)
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest:               true,
				ProtoV6ProviderFactories: sidecarFactories(gw),
				Steps: []resource.TestStep{{
					Config: testProviderBlock + `
resource "hoop_sidecar" "payments" {
  name = "payments"
  configuration = jsonencode({
    listeners = [{ name = "appdb", protocol = "postgres", listen = "0.0.0.0:15432", upstream = "db:5432" }]
  })
  slack_channels = ` + tt.channels + `
}`,
					ExpectError: regexp.MustCompile(tt.want),
				}},
			})
		})
	}
}

// A gateway that answers /serverinfo without license_info is left to decide
// at apply.
func TestSidecarResourcePlansWhenLicenseUnknown(t *testing.T) {
	gw := newFakeSidecarGateway(t)
	gw.license = fakeLicense{none: true}
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: sidecarFactories(gw),
		Steps: []resource.TestStep{{
			Config: sidecarListenerConfig("appdb", "postgres"),
			Check:  resource.TestCheckResourceAttr("hoop_sidecar.payments", "id", "sc-1"),
		}},
	})
}

func TestAnalyzerRuleResourceTierValidation(t *testing.T) {
	gw := newFakeRuleGateway("/ai/session-analyzer/rules", true)
	for _, tt := range []struct {
		name, tier, want string
	}{
		{"require_access_request without a rule", `{ action = "require_access_request" }`, `Access Request Rule Required`},
		{"block_execution with a rule", `{ action = "block_execution", access_request_rule_name = "x" }`, `Access Request Rule Not Allowed`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest: true,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
				},
				Steps: []resource.TestStep{{
					Config: testProviderBlock + `
resource "hoop_ai_session_analyzer_rule" "r" {
  name = "r"
  risk_evaluation = {
    low_risk    = { action = "allow_execution" }
    medium_risk = { action = "allow_execution" }
    high_risk   = ` + tt.tier + `
  }
}`,
					ExpectError: regexp.MustCompile(tt.want),
				}},
			})
		})
	}
}
