// Copyright (c) HashiCorp, Inc.

package provider

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const guardrailSpec = `jsonencode({
    rules = [{ name = "no-delete", type = "operation", operations = ["delete", "drop"], message = "refused" }]
  })`

// TestGuardrailResourceRefusesNamelessTarget: the gateway binds per listener
// and answers 422 to a target without one; the schema says so at plan.
func TestGuardrailResourceRefusesNamelessTarget(t *testing.T) {
	gw := newFakeRuleGateway("/guardrails", false)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name            = "no-delete"
  sidecar_spec    = ` + guardrailSpec + `
  sidecar_targets = [{ sidecar_id = "sc-1" }]
}`,
				ExpectError: regexp.MustCompile(`listener_name`),
			},
			{
				Config: testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name            = "no-delete"
  sidecar_spec    = ` + guardrailSpec + `
  sidecar_targets = [{ sidecar_id = "sc-1", listener_name = "" }]
}`,
				ExpectError: regexp.MustCompile(`listener_name`),
			},
		},
	})
}

func TestGuardrailResource(t *testing.T) {
	gw := newFakeRuleGateway("/guardrails", false)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			// Declared targets go out as a list, one listener each.
			{
				Config: testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name         = "no-delete"
  description  = "no destructive statements"
  input        = jsonencode({ rules = [{ type = "deny_words_list", words = ["DELETE"] }] })
  sidecar_spec = ` + guardrailSpec + `
  sidecar_targets = [
    { sidecar_id = "sc-1", listener_name = "appdb" },
    { sidecar_id = "sc-2", listener_name = "reports" },
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "id", "rule-1"),
					resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "sidecar_targets.#", "2"),
					resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "sidecar_targets.1.listener_name", "reports"),
					resource.TestCheckNoResourceAttr("hoop_guardrail.no_delete", "output"),
					checkLastBody(gw.rec, "POST", "/guardrails", map[string]string{
						"sidecar_targets": `[{"sidecar_id":"sc-1","listener_name":"appdb"},{"sidecar_id":"sc-2","listener_name":"reports"}]`,
						"sidecar_spec":    `{"rules":[{"name":"no-delete","type":"operation","operations":["delete","drop"],"message":"refused"}]}`,
						"input":           `{"rules":[{"type":"deny_words_list","words":["DELETE"]}]}`,
						"output":          `{"rules":[]}`,
						"connection_ids":  `[]`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// Removing both attributes sends the values that clear them. An
			// absent key would leave the fleet enforcing a rule Terraform
			// no longer describes.
			{
				Config: testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name        = "no-delete"
  description = "no destructive statements"
  input       = jsonencode({ rules = [{ type = "deny_words_list", words = ["DELETE"] }] })
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_guardrail.no_delete", "sidecar_targets"),
					resource.TestCheckNoResourceAttr("hoop_guardrail.no_delete", "sidecar_spec"),
					checkLastBody(gw.rec, "PUT", "/guardrails/rule-1", map[string]string{
						"sidecar_targets": `[]`,
						"sidecar_spec":    `null`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// With neither attribute declared, a write says nothing about
			// sidecars: the keys stay off the wire.
			{
				Config: testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name        = "no-delete"
  description = "no destructive statements, reworded"
  input       = jsonencode({ rules = [{ type = "deny_words_list", words = ["DELETE"] }] })
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkLastBody(gw.rec, "PUT", "/guardrails/rule-1", map[string]string{
						"sidecar_targets": "<absent>",
						"sidecar_spec":    "<absent>",
					}),
					resetRecorder(gw.rec),
				),
			},
			// A binding made outside Terraform, on a rule whose configuration
			// never declared sidecar_targets, is not drift.
			{
				PreConfig: func() {
					gw.mu.Lock()
					defer gw.mu.Unlock()
					gw.rules["rule-1"]["sidecar_targets"] = json.RawMessage(`[{"sidecar_id":"sc-9","listener_name":"appdb"}]`)
				},
				Config: testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name        = "no-delete"
  description = "no destructive statements, reworded"
  input       = jsonencode({ rules = [{ type = "deny_words_list", words = ["DELETE"] }] })
}`,
				PlanOnly: true,
			},
			{
				ResourceName:      "hoop_guardrail.no_delete",
				ImportState:       true,
				ImportStateId:     "rule-1",
				ImportStateVerify: true,
			},
		},
	})
}

// TestGuardrailResourceKeepsTargetOrder: the gateway answers bindings sorted
// by sidecar id and listener. A configuration in any other order must apply
// and then plan clean, including a target added in the middle and a reorder.
func TestGuardrailResourceKeepsTargetOrder(t *testing.T) {
	gw := newFakeRuleGateway("/guardrails", false)
	config := func(targets string) string {
		return testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name            = "no-delete"
  sidecar_spec    = ` + guardrailSpec + `
  sidecar_targets = [` + targets + `]
}`
	}
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: config(`
    { sidecar_id = "sc-2", listener_name = "orders" },
    { sidecar_id = "sc-1", listener_name = "appdb" },`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "sidecar_targets.0.sidecar_id", "sc-2"),
					resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "sidecar_targets.1.sidecar_id", "sc-1"),
				),
			},
			{
				Config: config(`
    { sidecar_id = "sc-2", listener_name = "orders" },
    { sidecar_id = "sc-1", listener_name = "reports" },
    { sidecar_id = "sc-1", listener_name = "appdb" },`),
				Check: resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "sidecar_targets.1.listener_name", "reports"),
			},
			{
				Config: config(`
    { sidecar_id = "sc-1", listener_name = "reports" },
    { sidecar_id = "sc-1", listener_name = "appdb" },
    { sidecar_id = "sc-2", listener_name = "orders" },`),
				Check: resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "sidecar_targets.2.sidecar_id", "sc-2"),
			},
		},
	})
}

// A link made from hoop_connection's guardrail_rules shows up in the
// guardrail's connection_ids. A guardrail that never declared the attribute
// must not read it as drift, and its writes must carry the link back: the
// gateway replaces a guardrail's connections on every write and reads an
// absent key as none. Declaring the attribute takes the links over, and
// removing it unlinks them.
func TestGuardrailResourceKeepsConnectionsLinkedElsewhere(t *testing.T) {
	gw := newFakeRuleGateway("/guardrails", false)
	config := func(extra string) string {
		return testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name  = "no-delete"
  input = jsonencode({ rules = [{ type = "deny_words_list", words = ["DELETE"] }] })
` + extra + `
}`
	}
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: config(``),
				Check:  resetRecorder(gw.rec),
			},
			// hoop_connection links the guardrail: not drift.
			{
				PreConfig: func() {
					gw.mu.Lock()
					defer gw.mu.Unlock()
					gw.rules["rule-1"]["connection_ids"] = json.RawMessage(`["conn-1"]`)
				},
				Config:   config(``),
				PlanOnly: true,
			},
			// An unrelated change keeps the link.
			{
				Config: config(`  description = "reworded"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_guardrail.no_delete", "connection_ids"),
					checkLastBody(gw.rec, "PUT", "/guardrails/rule-1", map[string]string{
						"connection_ids": `["conn-1"]`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// Declared, Terraform owns the links.
			{
				Config: config(`  connection_ids = ["conn-2"]`),
				Check: checkLastBody(gw.rec, "PUT", "/guardrails/rule-1", map[string]string{
					"connection_ids": `["conn-2"]`,
				}),
			},
			// Removing the declared value unlinks.
			{
				Config: config(``),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_guardrail.no_delete", "connection_ids"),
					checkLastBody(gw.rec, "PUT", "/guardrails/rule-1", map[string]string{
						"connection_ids": `[]`,
					}),
				),
			},
		},
	})
}

// The gateway answers attribute names sorted on a read and in the written
// order on a write. Written in any order, they apply and then plan clean.
func TestGuardrailResourceAttributesAnyOrder(t *testing.T) {
	gw := newFakeRuleGateway("/guardrails", false)
	config := func(attributes string) string {
		return testProviderBlock + `
resource "hoop_guardrail" "no_delete" {
  name       = "no-delete"
  attributes = ` + attributes + `
}`
	}
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: config(`["zeta", "alpha"]`),
				Check:  resource.TestCheckResourceAttr("hoop_guardrail.no_delete", "attributes.#", "2"),
			},
			{
				Config: config(`["zeta", "mid", "alpha"]`),
				Check:  resource.TestCheckTypeSetElemAttr("hoop_guardrail.no_delete", "attributes.*", "mid"),
			},
			// Import cannot tell whether the configuration owns the links,
			// so an imported rule has null attributes.
			{
				ResourceName:            "hoop_guardrail.no_delete",
				ImportState:             true,
				ImportStateId:           "rule-1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
		},
	})
}

// A rule bound to a listener has to say what it does there. The plan refuses
// the writes the gateway would refuse, on every rule kind: a create with
// targets and no spec, and removing a declared spec while targets stay. A
// spec never declared on an existing rule is kept by the gateway, possibly
// set in the UI, and is not the plan's to refuse.
func TestRuleResourcesRefuseBoundRuleWithoutSpec(t *testing.T) {
	for _, tt := range []struct {
		name, collection, resource, extra string
		keyByName                         bool
	}{
		{"guardrail", "/guardrails", "hoop_guardrail", ``, false},
		{"datamasking", "/datamasking-rules", "hoop_datamasking_rules", `
  description            = "masks"
  score_threshold        = 0.5
  connection_ids         = []
  custom_entity_types    = []
  supported_entity_types = []`, false},
		{"analyzer", "/ai/session-analyzer/rules", "hoop_ai_session_analyzer_rule", analyzerRiskEvaluation, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gw := newFakeRuleGateway(tt.collection, tt.keyByName)
			path := "/rule-1"
			if tt.keyByName {
				path = "/r"
			}
			config := func(body string) string {
				return testProviderBlock + `
resource "` + tt.resource + `" "r" {
  name = "r"` + tt.extra + `
` + body + `
}`
			}
			spec := `  sidecar_spec    = jsonencode({ rules = [{ name = "r" }] })`
			targets := `  sidecar_targets = [{ sidecar_id = "sc-1", listener_name = "appdb" }]`
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest: true,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
				},
				Steps: []resource.TestStep{
					{
						Config:      config(targets),
						ExpectError: regexp.MustCompile(`Bound Rule Needs a Sidecar Spec`),
					},
					{
						Config: config(``),
						Check:  resetRecorder(gw.rec),
					},
					// The spec was set in the UI; Terraform binds the rule
					// without declaring it, and the write leaves it alone.
					{
						PreConfig: func() {
							gw.mu.Lock()
							defer gw.mu.Unlock()
							for _, rule := range gw.rules {
								rule["sidecar_spec"] = json.RawMessage(`{"rules":[{"name":"ui"}]}`)
							}
						},
						Config: config(targets),
						Check: resource.ComposeAggregateTestCheckFunc(
							checkLastBody(gw.rec, "PUT", path, map[string]string{
								"sidecar_spec":    "<absent>",
								"sidecar_targets": `[{"sidecar_id":"sc-1","listener_name":"appdb"}]`,
							}),
						),
					},
					{
						Config: config(spec + "\n" + targets),
					},
					// Removing the declared spec would send the null that clears it.
					{
						Config:      config(targets),
						ExpectError: regexp.MustCompile(`Bound Rule Needs a Sidecar Spec`),
					},
				},
			})
		})
	}
}

// An attribute links rules from its own side. A rule that never declared
// attributes must not read those links as drift, and its writes must carry
// them back, since the gateway replaces a rule's links on every write and
// reads an absent key as none. Declaring the attribute takes the links
// over, and removing it unlinks them.
func TestRuleResourcesKeepAttributesLinkedElsewhere(t *testing.T) {
	for _, tt := range []struct {
		name, collection, resource, body, path string
		keyByName                              bool
	}{
		{"guardrail", "/guardrails", "hoop_guardrail", ``, "/guardrails/rule-1", false},
		{"access request rule", "/access-requests/rules", "hoop_access_request_rule", `
  access_type      = "jit"
  connection_names = ["pg-prod"]
  reviewers_groups = ["sre"]
  min_approvals    = 1`, "/access-requests/rules/r", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gw := newFakeRuleGateway(tt.collection, tt.keyByName)
			config := func(extra string) string {
				return testProviderBlock + `
resource "` + tt.resource + `" "r" {
  name = "r"` + tt.body + `
` + extra + `
}`
			}
			key := "rule-1"
			if tt.keyByName {
				key = "r"
			}
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest: true,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
				},
				Steps: []resource.TestStep{
					{
						Config: config(``),
					},
					{
						PreConfig: func() {
							gw.mu.Lock()
							defer gw.mu.Unlock()
							gw.rules[key]["attributes"] = json.RawMessage(`["pci"]`)
						},
						Config:   config(``),
						PlanOnly: true,
					},
					{
						Config: config(`  description = "reworded"`),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckNoResourceAttr(tt.resource+".r", "attributes"),
							checkLastBody(gw.rec, "PUT", tt.path, map[string]string{"attributes": `["pci"]`}),
						),
					},
					{
						Config: config(`  attributes = ["sox"]`),
						Check:  checkLastBody(gw.rec, "PUT", tt.path, map[string]string{"attributes": `["sox"]`}),
					},
					{
						Config: config(``),
						Check:  checkLastBody(gw.rec, "PUT", tt.path, map[string]string{"attributes": `[]`}),
					},
				},
			})
		})
	}
}
