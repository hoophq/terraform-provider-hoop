package hoop

import (
	"bytes"
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
	apiURL := fmt.Sprintf("%s/runbooks/rules/%s", c.apiURL, id)
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
		var resource RunbookRule
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding runbooks configuration resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) CreateRunbookRule(rule RunbookRule) (*RunbookRule, error) {
	return c.doRunbookRuleRequestWithBody("", rule)
}

func (c *Client) UpdateRunbookRuleByID(rule RunbookRule) (*RunbookRule, error) {
	return c.doRunbookRuleRequestWithBody(rule.ID, rule)
}

func (c *Client) DeleteRunbookRuleByID(id string) error {
	apiURL := fmt.Sprintf("%s/runbooks/rules/%s", c.apiURL, id)
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return validateErr(resp)
}

func (c *Client) doRunbookRuleRequestWithBody(id string, rule RunbookRule) (*RunbookRule, error) {
	method := "POST"
	apiURL := fmt.Sprintf("%s/runbooks/rules", c.apiURL)
	if id != "" {
		method = "PUT"
		apiURL = fmt.Sprintf("%s/%s", apiURL, id)
	}
	jsonData, err := json.Marshal(rule)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runbook rule, reason=%v", err)
	}
	req, err := http.NewRequest(method, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var resource RunbookRule
		if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
			return nil, fmt.Errorf("failed decoding runbook rule resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}
