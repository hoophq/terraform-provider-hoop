// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type DataMaskingRule struct {
	ID                   string                      `json:"id,omitempty"`
	Name                 string                      `json:"name"`
	Description          string                      `json:"description"`
	ScoreThreshold       *float64                    `json:"score_threshold"`
	ConnectionIDs        []string                    `json:"connection_ids"`
	SupportedEntityTypes []SupportedEntityTypesEntry `json:"supported_entity_types"`
	CustomEntityTypes    []CustomEntityTypesEntry    `json:"custom_entity_types"`
}

type SupportedEntityTypesEntry struct {
	Name        string   `json:"name"`
	EntityTypes []string `json:"entity_types"`
}

type CustomEntityTypesEntry struct {
	Name     string   `json:"name"`
	Regex    string   `json:"regex"`
	DenyList []string `json:"deny_list"`
	Score    float64  `json:"score"`
}

func (c *Client) GetDatamaskingRule(resourceID string) (*DataMaskingRule, error) {
	req, err := c.newRequest("GET", "/datamasking-rules/"+resourceID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeDatamaskingRule(resp.Body)
}

func (c *Client) CreateDatamaskingRule(rule DataMaskingRule) (*DataMaskingRule, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data masking rule, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/datamasking-rules", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeDatamaskingRule(resp.Body)
}

func (c *Client) UpdateDatamaskingRule(rule DataMaskingRule) (*DataMaskingRule, error) {
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data masking rule, reason=%v", err)
	}
	req, err := c.newRequest("PUT", "/datamasking-rules/"+rule.ID, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeDatamaskingRule(resp.Body)
}

func (c *Client) DeleteDatamaskingRule(resourceID string) error {
	req, err := c.newRequest("DELETE", "/datamasking-rules/"+resourceID, nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeDatamaskingRule(responseBody io.Reader) (*DataMaskingRule, error) {
	var resource DataMaskingRule
	if err := json.NewDecoder(responseBody).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding data masking rule resource, reason=%v", err)
	}
	return &resource, nil
}
