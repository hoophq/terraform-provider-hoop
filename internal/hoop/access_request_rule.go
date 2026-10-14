// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// AccessRequestRule decides who approves access. With access_type jit,
// command or jit_command it gates the connections it names. With access_type
// sidecar it gates nothing on the gateway: it is the policy a sidecar review
// is filed under, named by an analyzer rule's approval_rule, and says who may
// release a statement the sidecar held.
//
// The gateway requires the four list fields to be present on every write, so
// the client sends [] for an empty one and never null.
type AccessRequestRule struct {
	ID                     string   `json:"id,omitempty"`
	Name                   string   `json:"name"`
	Description            *string  `json:"description"`
	AccessType             string   `json:"access_type"`
	ConnectionNames        []string `json:"connection_names"`
	Attributes             []string `json:"attributes"`
	ApprovalRequiredGroups []string `json:"approval_required_groups"`
	AllGroupsMustApprove   bool     `json:"all_groups_must_approve"`
	ReviewersGroups        []string `json:"reviewers_groups"`
	ForceApprovalGroups    []string `json:"force_approval_groups"`
	SkipReviewGroups       []string `json:"skip_review_groups"`
	AccessMaxDuration      *int     `json:"access_max_duration,omitempty"`
	MinApprovals           *int     `json:"min_approvals,omitempty"`
	// PendingTTLSec and ApprovalTTLSec apply to sidecar rules only. On
	// update, absent keeps and 0 clears. The gateway never answers them on
	// a connection rule.
	PendingTTLSec  *int `json:"pending_ttl_sec,omitempty"`
	ApprovalTTLSec *int `json:"approval_ttl_sec,omitempty"`
	// ManagedBy is read-only: "hoop" for a protection profile's rule,
	// "ai-session-analyzer" for the rule an analyzer rule holds under. A
	// managed rule accepts changes to its groups and approval counts only,
	// and refuses deletion.
	ManagedBy *string `json:"managed_by,omitempty"`
}

func accessRequestRulePath(name string) string {
	return "/access-requests/rules/" + url.PathEscape(name)
}

// nonNil keeps the gateway's required-list validation satisfied.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (r *AccessRequestRule) prepare() {
	r.ConnectionNames = nonNil(r.ConnectionNames)
	r.Attributes = nonNil(r.Attributes)
	r.ApprovalRequiredGroups = nonNil(r.ApprovalRequiredGroups)
	r.ReviewersGroups = nonNil(r.ReviewersGroups)
	r.ForceApprovalGroups = nonNil(r.ForceApprovalGroups)
	r.SkipReviewGroups = nonNil(r.SkipReviewGroups)
	r.ManagedBy = nil
}

func (c *Client) CreateAccessRequestRule(rule AccessRequestRule) (*AccessRequestRule, error) {
	rule.prepare()
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal access request rule, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/access-requests/rules", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAccessRequestRule(resp.Body)
}

func (c *Client) GetAccessRequestRule(name string) (*AccessRequestRule, error) {
	req, err := c.newRequest("GET", accessRequestRulePath(name), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAccessRequestRule(resp.Body)
}

func (c *Client) UpdateAccessRequestRule(rule AccessRequestRule) (*AccessRequestRule, error) {
	rule.prepare()
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal access request rule, reason=%v", err)
	}
	req, err := c.newRequest("PUT", accessRequestRulePath(rule.Name), body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAccessRequestRule(resp.Body)
}

func (c *Client) DeleteAccessRequestRule(name string) error {
	req, err := c.newRequest("DELETE", accessRequestRulePath(name), nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeAccessRequestRule(body io.Reader) (*AccessRequestRule, error) {
	var out AccessRequestRule
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding access request rule resource, reason=%v", err)
	}
	return &out, nil
}
