// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// AnalyzerRiskTier is what the analyzer does with a session it classifies at
// one risk level.
type AnalyzerRiskTier struct {
	Action                string  `json:"action"`
	AccessRequestRuleName *string `json:"access_request_rule_name,omitempty"`
}

// AnalyzerRiskEvaluation names the tier for each risk level. The *_action
// fields are the older spelling; this client reads them from a gateway that
// still answers with them and never sends them.
type AnalyzerRiskEvaluation struct {
	LowRiskAction    string `json:"low_risk_action,omitempty"`
	MediumRiskAction string `json:"medium_risk_action,omitempty"`
	HighRiskAction   string `json:"high_risk_action,omitempty"`

	LowRisk    *AnalyzerRiskTier `json:"low_risk,omitempty"`
	MediumRisk *AnalyzerRiskTier `json:"medium_risk,omitempty"`
	HighRisk   *AnalyzerRiskTier `json:"high_risk,omitempty"`
}

// AnalyzerRule is an AI session analyzer rule. The gateway keys it by name in
// the URL, so a rename is a delete and a create.
type AnalyzerRule struct {
	ID              string                 `json:"id,omitempty"`
	Name            string                 `json:"name"`
	Description     *string                `json:"description"`
	ConnectionNames []string               `json:"connection_names"`
	RiskEvaluation  AnalyzerRiskEvaluation `json:"risk_evaluation"`
	CustomPrompt    *string                `json:"custom_prompt,omitempty"`
	Agentic         bool                   `json:"agentic"`
	SidecarSpec     json.RawMessage        `json:"sidecar_spec,omitempty"`
	// SidecarTargets follows the gateway's three-way reading; see the type.
	SidecarTargets SidecarTargets `json:"sidecar_targets,omitempty"`
	// ReviewersGroups has the same reading: absent keeps the stored groups,
	// [] clears them, and with none set the admin group reviews.
	ReviewersGroups *[]string `json:"reviewers_groups,omitempty"`
	// PendingTTLSec and ApprovalTTLSec: absent keeps, 0 clears. The gateway
	// reads them only while sidecar_spec holds statements for approval.
	PendingTTLSec  *int `json:"pending_ttl_sec,omitempty"`
	ApprovalTTLSec *int `json:"approval_ttl_sec,omitempty"`
}

// analyzerRuleResponse is the read shape. The gateway answers the targets and
// reviewer groups as plain lists, so they decode here and move into the
// request shape afterwards.
type analyzerRuleResponse struct {
	AnalyzerRule
	SidecarTargets  []SidecarRuleTarget `json:"sidecar_targets"`
	ReviewersGroups []string            `json:"reviewers_groups"`
}

func analyzerRulePath(name string) string {
	return "/ai/session-analyzer/rules/" + url.PathEscape(name)
}

func (c *Client) CreateAnalyzerRule(rule AnalyzerRule) (*AnalyzerRule, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analyzer rule, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/ai/session-analyzer/rules", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAnalyzerRule(resp.Body)
}

func (c *Client) GetAnalyzerRule(name string) (*AnalyzerRule, error) {
	req, err := c.newRequest("GET", analyzerRulePath(name), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAnalyzerRule(resp.Body)
}

func (c *Client) UpdateAnalyzerRule(rule AnalyzerRule) (*AnalyzerRule, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analyzer rule, reason=%v", err)
	}
	req, err := c.newRequest("PUT", analyzerRulePath(rule.Name), body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAnalyzerRule(resp.Body)
}

func (c *Client) DeleteAnalyzerRule(name string) error {
	req, err := c.newRequest("DELETE", analyzerRulePath(name), nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeAnalyzerRule(body io.Reader) (*AnalyzerRule, error) {
	var out analyzerRuleResponse
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding analyzer rule resource, reason=%v", err)
	}
	rule := out.AnalyzerRule
	rule.SidecarTargets = NewSidecarTargets(out.SidecarTargets)
	groups := out.ReviewersGroups
	if groups == nil {
		groups = []string{}
	}
	rule.ReviewersGroups = &groups
	return &rule, nil
}
