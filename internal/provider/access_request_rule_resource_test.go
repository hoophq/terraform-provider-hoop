// Copyright (c) HashiCorp, Inc.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccessRequestRuleResource(t *testing.T) {
	gw := newFakeRuleGateway("/access-requests/rules", true)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			// A sidecar rule: no connections, review limits, reviewers.
			{
				Config: testProviderBlock + `
resource "hoop_access_request_rule" "dba" {
  name             = "dba-approvals"
  description      = "DBA leads release held statements"
  access_type      = "sidecar"
  reviewers_groups = ["dba-leads"]
  min_approvals    = 1
  pending_ttl_sec  = 900
  approval_ttl_sec = 600
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_access_request_rule.dba", "id", "rule-1"),
					resource.TestCheckResourceAttr("hoop_access_request_rule.dba", "all_groups_must_approve", "false"),
					resource.TestCheckResourceAttr("hoop_access_request_rule.dba", "managed_by", ""),
					resource.TestCheckNoResourceAttr("hoop_access_request_rule.dba", "connection_names"),
					// The gateway requires every list key on the wire, [] when empty.
					checkLastBody(gw.rec, "POST", "/access-requests/rules", map[string]string{
						"access_type":              `"sidecar"`,
						"connection_names":         `[]`,
						"approval_required_groups": `[]`,
						"reviewers_groups":         `["dba-leads"]`,
						"force_approval_groups":    `[]`,
						"min_approvals":            `1`,
						"pending_ttl_sec":          `900`,
						"approval_ttl_sec":         `600`,
						"managed_by":               "<absent>",
					}),
					resetRecorder(gw.rec),
				),
			},
			// Dropping a limit clears it with 0; absent would keep it.
			{
				Config: testProviderBlock + `
resource "hoop_access_request_rule" "dba" {
  name             = "dba-approvals"
  description      = "DBA leads release held statements"
  access_type      = "sidecar"
  reviewers_groups = ["dba-leads", "sre"]
  min_approvals    = 1
  approval_ttl_sec = 600
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("hoop_access_request_rule.dba", "pending_ttl_sec"),
					resource.TestCheckResourceAttr("hoop_access_request_rule.dba", "reviewers_groups.#", "2"),
					checkLastBody(gw.rec, "PUT", "/access-requests/rules/dba-approvals", map[string]string{
						"pending_ttl_sec":  `0`,
						"approval_ttl_sec": `600`,
					}),
				),
			},
			{
				ResourceName:      "hoop_access_request_rule.dba",
				ImportState:       true,
				ImportStateId:     "dba-approvals",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccessRequestRuleResourceConnectionRule(t *testing.T) {
	gw := newFakeRuleGateway("/access-requests/rules", true)
	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: true,
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
		},
		Steps: []resource.TestStep{
			{
				Config: testProviderBlock + `
resource "hoop_access_request_rule" "prod" {
  name                     = "prod-jit"
  access_type              = "jit"
  connection_names         = ["pg-prod"]
  approval_required_groups = ["developers"]
  reviewers_groups         = ["sre"]
  force_approval_groups    = ["admin"]
  access_max_duration      = 3600
  min_approvals            = 2
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hoop_access_request_rule.prod", "connection_names.0", "pg-prod"),
					resource.TestCheckResourceAttr("hoop_access_request_rule.prod", "access_max_duration", "3600"),
					checkLastBody(gw.rec, "POST", "/access-requests/rules", map[string]string{
						"access_type":         `"jit"`,
						"connection_names":    `["pg-prod"]`,
						"access_max_duration": `3600`,
						"pending_ttl_sec":     "<absent>",
					}),
				),
			},
		},
	})
}

func TestAccessRequestRuleResourceValidation(t *testing.T) {
	gw := newFakeRuleGateway("/access-requests/rules", true)
	for _, tt := range []struct {
		name   string
		config string
		want   string
	}{
		{"sidecar rule with connections", `
  access_type      = "sidecar"
  connection_names = ["pg-prod"]
  reviewers_groups = ["sre"]`, `Sidecar Rule Gates No Connection`},
		{"connection rule with review limits", `
  access_type      = "jit"
  connection_names = ["pg-prod"]
  reviewers_groups = ["sre"]
  min_approvals    = 1
  pending_ttl_sec  = 900`, `Review Limits Are For Sidecar Rules`},
		{"connection rule gating nothing", `
  access_type      = "command"
  connection_names = []
  attributes       = []
  reviewers_groups = ["sre"]
  min_approvals    = 1`, `Connection Rule Gates Nothing`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resource.ParallelTest(t, resource.TestCase{
				IsUnitTest: true,
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"hoop": providerserver.NewProtocol6WithError(New("test", clientFunc(gw.handle))()),
				},
				Steps: []resource.TestStep{{
					Config:      testProviderBlock + "\nresource \"hoop_access_request_rule\" \"r\" {\n  name = \"r\"" + tt.config + "\n}",
					ExpectError: regexp.MustCompile(tt.want),
				}},
			})
		})
	}
}
