// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// GuardRailRule is a guardrail as the gateway reads and writes it. Input and
// Output hold the gateway's own rule documents, {"rules": [...]}. SidecarSpec
// holds the same rule in the sidecar's vocabulary, the guardrails block a
// listener receives.
type GuardRailRule struct {
	ID            string          `json:"id,omitempty"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Input         json.RawMessage `json:"input"`
	Output        json.RawMessage `json:"output"`
	ConnectionIDs []string        `json:"connection_ids"`
	Attributes    []string        `json:"attributes"`
	SidecarSpec   json.RawMessage `json:"sidecar_spec,omitempty"`
	// SidecarTargets follows the gateway's three-way reading; see the type.
	SidecarTargets SidecarTargets `json:"sidecar_targets,omitempty"`
}

func guardrailPath(id string) string {
	return "/guardrails/" + url.PathEscape(id)
}

func (c *Client) CreateGuardRailRule(rule GuardRailRule) (*GuardRailRule, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal guardrail rule, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/guardrails", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeGuardRailRule(resp.Body)
}

func (c *Client) GetGuardRailRule(id string) (*GuardRailRule, error) {
	req, err := c.newRequest("GET", guardrailPath(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeGuardRailRule(resp.Body)
}

func (c *Client) UpdateGuardRailRule(rule GuardRailRule) (*GuardRailRule, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal guardrail rule, reason=%v", err)
	}
	req, err := c.newRequest("PUT", guardrailPath(rule.ID), body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeGuardRailRule(resp.Body)
}

func (c *Client) DeleteGuardRailRule(id string) error {
	req, err := c.newRequest("DELETE", guardrailPath(id), nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeGuardRailRule(body io.Reader) (*GuardRailRule, error) {
	var out GuardRailRule
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding guardrail rule resource, reason=%v", err)
	}
	return &out, nil
}
