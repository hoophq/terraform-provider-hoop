// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type PluginConnection struct {
	ID           string   `json:"id"`
	PluginID     string   `json:"plugin_id"`
	ConnectionID string   `json:"connection_id"`
	Config       []string `json:"config"`
}

func (c *Client) GetPluginConnection(pluginName, connectionID string) (*PluginConnection, error) {
	apiURL := fmt.Sprintf("%s/plugins/%s/conn/%s", c.apiURL, pluginName, connectionID)
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
		var resource PluginConnection
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding connection resource, reason=%v", err)
		}
		if resource.Config == nil {
			resource.Config = []string{}
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) UpsertPluginConnection(pluginName, connectionID string, config []string) (*PluginConnection, error) {
	apiURL := fmt.Sprintf("%s/plugins/%s/conn/%s", c.apiURL, pluginName, connectionID)
	jsonData, err := json.Marshal(map[string]any{"config": config})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin connection, reason=%v", err)
	}
	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(jsonData))
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
	if resp.StatusCode == http.StatusOK {
		var resource PluginConnection
		if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
			return nil, fmt.Errorf("failed decoding connection resource, reason=%v", err)
		}
		if resource.Config == nil {
			resource.Config = []string{}
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) DeletePluginConnection(pluginName, connectionID string) error {
	apiURL := fmt.Sprintf("%s/plugins/%s/conn/%s", c.apiURL, pluginName, connectionID)
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request, reason=%v", err)
	}
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
