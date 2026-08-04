// Copyright (c) HashiCorp, Inc.

package hoop

import (
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
	req, err := c.newRequest("GET", "/connections/"+name, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeConnection(resp.Body)
}

func (c *Client) CreateConnection(conn Connection) (*Connection, error) {
	body, err := encodeConnection(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection, reason=%v", err)
	}

	req, err := c.newRequest("POST", "/connections", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeConnection(resp.Body)
}

func (c *Client) UpdateConnection(conn Connection) (*Connection, error) {
	body, err := encodeConnection(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal connection, reason=%v", err)
	}

	req, err := c.newRequest("PUT", "/connections/"+conn.Name, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeConnection(resp.Body)
}

func (c *Client) DeleteConnection(name string) error {
	req, err := c.newRequest("DELETE", "/connections/"+name, nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
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
