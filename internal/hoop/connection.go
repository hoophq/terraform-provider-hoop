// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Connection struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Command             []string          `json:"command"`
	Type                string            `json:"type"`
	SubType             string            `json:"subtype"`
	Secrets             map[string]string `json:"secret"`
	AgentId             string            `json:"agent_id"`
	Reviewers           []string          `json:"reviewers"`
	RedactEnabled       bool              `json:"redact_enabled"`
	RedactTypes         []string          `json:"redact_types"`
	ConnectionTags      map[string]string `json:"connection_tags"`
	AccessModeRunbooks  string            `json:"access_mode_runbooks"`
	AccessModeExec      string            `json:"access_mode_exec"`
	AccessModeConnect   string            `json:"access_mode_connect"`
	AccessSchema        string            `json:"access_schema"`
	GuardRailRules      []string          `json:"guardrail_rules"`
	JiraIssueTemplateID string            `json:"jira_issue_template_id"`
}

func (c *Client) GetConnection(name string) (*Connection, error) {
	apiURL := fmt.Sprintf("%s/connections/%s", c.apiURL, name)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return decodeConnection(resp.Body)
	}
	return nil, validateErr(resp)
}

func (c *Client) CreateConnection(conn Connection) (*Connection, error) {
	body, err := encodeConnection(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection, reason=%v", err)
	}

	apiURL := fmt.Sprintf("%s/connections", c.apiURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		return decodeConnection(resp.Body)
	}
	return nil, validateErr(resp)
}

func (c *Client) UpdateConnection(conn Connection) (*Connection, error) {
	body, err := encodeConnection(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection, reason=%v", err)
	}

	apiURL := fmt.Sprintf("%s/connections/%s", c.apiURL, conn.Name)
	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return decodeConnection(resp.Body)
	}
	return nil, validateErr(resp)
}

func (c *Client) DeleteConnection(name string) error {
	apiURL := fmt.Sprintf("%s/connections/%s", c.apiURL, name)

	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete connection, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return validateErr(resp)
}

func decodeConnection(responseBody io.Reader) (*Connection, error) {
	var conn Connection
	err := json.NewDecoder(responseBody).Decode(&conn)
	if err != nil {
		return nil, fmt.Errorf("failed decoding connection resource, reason=%v", err)
	}
	secrets := map[string]string{}
	for key, val := range conn.Secrets {
		decVal, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to decode secret %q, reason=%v", key, err)
		}
		secrets[key] = string(decVal)
	}
	conn.Secrets = secrets
	return &conn, nil
}

func encodeConnection(conn Connection) ([]byte, error) {
	secrets := map[string]string{}
	for key, val := range conn.Secrets {
		secrets[key] = base64.StdEncoding.EncodeToString([]byte(val))
	}
	conn.Secrets = secrets
	return json.Marshal(conn)
}
