// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	apiURL := fmt.Sprintf("%s/datamasking-rules/%s", c.apiURL, resourceID)
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
		var resource DataMaskingRule
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding data masking rule resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) CreateDatamaskingRule(rule DataMaskingRule) (*DataMaskingRule, error) {
	apiURL := fmt.Sprintf("%s/datamasking-rules", c.apiURL)
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data masking rule, reason=%v", err)
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create data masking rule, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		var resource DataMaskingRule
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding data masking rule resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) UpdateDatamaskingRule(rule DataMaskingRule) (*DataMaskingRule, error) {
	apiURL := fmt.Sprintf("%s/datamasking-rules/%s", c.apiURL, rule.ID)
	body, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data masking rule, reason=%v", err)
	}
	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update data masking rule, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var resource DataMaskingRule
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding data masking rule resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) DeleteDatamaskingRule(resourceID string) error {
	apiURL := fmt.Sprintf("%s/datamasking-rules/%s", c.apiURL, resourceID)
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete data masking rule, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return validateErr(resp)
}

// func (c *Client) DeleteConnection(name string) error {
// 	apiURL := fmt.Sprintf("%s/connections/%s", c.apiURL, name)

// 	req, err := http.NewRequest("DELETE", apiURL, nil)
// 	if err != nil {
// 		return fmt.Errorf("failed to create request, reason=%v", err)
// 	}
// 	req.Header.Set("Api-Key", c.token)
// 	resp, err := c.httpClient.Do(req)
// 	if err != nil {
// 		return fmt.Errorf("failed to delete connection, reason=%v", err)
// 	}
// 	defer resp.Body.Close()
// 	if resp.StatusCode == http.StatusNoContent {
// 		return nil
// 	}
// 	return validateErr(resp)
// }

// func decodeConnection(responseBody io.Reader) (*Connection, error) {
// 	var conn Connection
// 	err := json.NewDecoder(responseBody).Decode(&conn)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed decoding connection resource, reason=%v", err)
// 	}
// 	secrets := map[string]string{}
// 	for key, val := range conn.Secrets {
// 		decVal, err := base64.StdEncoding.DecodeString(val)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to decode secret %q, reason=%v", key, err)
// 		}
// 		secrets[key] = string(decVal)
// 	}
// 	conn.Secrets = secrets
// 	return &conn, nil
// }

// func encodeConnection(conn Connection) ([]byte, error) {
// 	secrets := map[string]string{}
// 	for key, val := range conn.Secrets {
// 		secrets[key] = base64.StdEncoding.EncodeToString([]byte(val))
// 	}
// 	conn.Secrets = secrets
// 	return json.Marshal(conn)
// }
