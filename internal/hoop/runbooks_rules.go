// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type RunbookRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Connections []string          `json:"connections"`
	UserGroups  []string          `json:"user_groups"`
	Runbooks    []RunbookRuleItem `json:"runbooks"`
}

type RunbookRuleItem struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
}

func (c *Client) GetRunbookRuleByID(id string) (*RunbookRule, error) {
	req, err := c.newRequest("GET", "/runbooks/rules/"+id, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var resource RunbookRule
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding runbooks configuration resource, reason=%v", err)
	}
	return &resource, nil
}

func (c *Client) CreateRunbookRule(rule RunbookRule) (*RunbookRule, error) {
	return c.doRunbookRuleRequestWithBody("", rule)
}

func (c *Client) UpdateRunbookRuleByID(rule RunbookRule) (*RunbookRule, error) {
	return c.doRunbookRuleRequestWithBody(rule.ID, rule)
}

func (c *Client) DeleteRunbookRuleByID(id string) error {
	req, err := c.newRequest("DELETE", "/runbooks/rules/"+id, nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func (c *Client) doRunbookRuleRequestWithBody(id string, rule RunbookRule) (*RunbookRule, error) {
	method := "POST"
	path := "/runbooks/rules"
	if id != "" {
		method = "PUT"
		path += "/" + id
	}
	jsonData, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runbook rule, reason=%v", err)
	}
	req, err := c.newRequest(method, path, jsonData)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var resource RunbookRule
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding runbook rule resource, reason=%v", err)
	}
	return &resource, nil
}
