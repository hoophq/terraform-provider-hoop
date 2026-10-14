// Copyright (c) HashiCorp, Inc.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const analyzerRiskEvaluation = `
  risk_evaluation = {
    low_risk    = { action = "allow_execution" }
    medium_risk = { action = "allow_execution" }
    high_risk   = { action = "require_access_request", access_request_rule_name = "prod-approvals" }
  }`

func TestAnalyzerRuleResource(t *testing.T) {
	gw := newFakeRuleGateway("/ai/session-analyzer/rules", true)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_ai_session_analyzer_rule" "prod" {
  name             = "prod-analysis"
  connection_names = ["pg-prod"]` + analyzerRiskEvaluation + `
  sidecar_spec     = jsonencode({ trigger = "statement", high = "block" })
  sidecar_targets  = [{ sidecar_id = "sc-1", listener_name = "appdb" }]
  reviewers_groups = ["dba-leads"]
  pending_ttl_sec  = 900
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_ai_session_analyzer_rule.prod", "id", "rule-1"),
					resource.TestCheckResourceAttr("hoop_ai_session_analyzer_rule.prod", "agentic", "false"),
					resource.TestCheckResourceAttr("hoop_ai_session_analyzer_rule.prod", "risk_evaluation.high_risk.access_request_rule_name", "prod-approvals"),
					resource.TestCheckNoResourceAttr("hoop_ai_session_analyzer_rule.prod", "risk_evaluation.low_risk.access_request_rule_name"),
					resource.TestCheckResourceAttr("hoop_ai_session_analyzer_rule.prod", "pending_ttl_sec", "900"),
					checkLastBody(gw.rec, "POST", "/ai/session-analyzer/rules", map[string]string{
						"risk_evaluation":  `{"low_risk":{"action":"allow_execution"},"medium_risk":{"action":"allow_execution"},"high_risk":{"action":"require_access_request","access_request_rule_name":"prod-approvals"}}`,
						"reviewers_groups": `["dba-leads"]`,
						"pending_ttl_sec":  `900`,
						"approval_ttl_sec": "<absent>",
						"sidecar_targets":  `[{"sidecar_id":"sc-1","listener_name":"appdb"}]`,
					}),
					resetRecorder(gw.rec),
				),
			},
			// Dropping reviewers_groups and pending_ttl_sec clears both: [] for
			// the groups, 0 for the limit. The targets stay, so the key stays
			// off the wire.
			{
				Config: testProviderBlock + `
resource "hoop_ai_session_analyzer_rule" "prod" {
  name             = "prod-analysis"
  connection_names = ["pg-prod"]` + analyzerRiskEvaluation + `
  sidecar_spec     = jsonencode({ trigger = "statement", high = "block" })
  sidecar_targets  = [{ sidecar_id = "sc-1", listener_name = "appdb" }]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_ai_session_analyzer_rule.prod", "reviewers_groups"),
					resource.TestCheckNoResourceAttr("hoop_ai_session_analyzer_rule.prod", "pending_ttl_sec"),
					checkLastBody(gw.rec, "PUT", "/ai/session-analyzer/rules/prod-analysis", map[string]string{
						"reviewers_groups": `[]`,
						"pending_ttl_sec":  `0`,
						"sidecar_targets":  `[{"sidecar_id":"sc-1","listener_name":"appdb"}]`,
					}),
				),
			},
			{
				ResourceName:      "hoop_ai_session_analyzer_rule.prod",
				ImportState:       true,
				ImportStateId:     "prod-analysis",
				ImportStateVerify: true,
				// Undeclared on import, declared in the configuration above.
				ImportStateVerifyIgnore: []string{"sidecar_spec", "sidecar_targets"},
			},
		},
	})
}
