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
