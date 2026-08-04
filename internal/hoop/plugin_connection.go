// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type PluginConnection struct {
	ID           string   `json:"id"`
	PluginID     string   `json:"plugin_id"`
	ConnectionID string   `json:"connection_id"`
	Config       []string `json:"config"`
}

func (c *Client) GetPluginConnection(pluginName, connectionID string) (*PluginConnection, error) {
	req, err := c.newRequest("GET", pluginConnectionPath(pluginName, connectionID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePluginConnection(resp.Body)
}

func (c *Client) UpsertPluginConnection(pluginName, connectionID string, config []string) (*PluginConnection, error) {
	jsonData, err := json.Marshal(map[string]any{"config": config})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin connection, reason=%v", err)
	}
	req, err := c.newRequest("PUT", pluginConnectionPath(pluginName, connectionID), jsonData)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePluginConnection(resp.Body)
}

func (c *Client) DeletePluginConnection(pluginName, connectionID string) error {
	req, err := c.newRequest("DELETE", pluginConnectionPath(pluginName, connectionID), nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func pluginConnectionPath(pluginName, connectionID string) string {
	return fmt.Sprintf("/plugins/%s/conn/%s", pluginName, connectionID)
}

func decodePluginConnection(responseBody io.Reader) (*PluginConnection, error) {
	var resource PluginConnection
	if err := json.NewDecoder(responseBody).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding plugin connection resource, reason=%v", err)
	}
	if resource.Config == nil {
		resource.Config = []string{}
	}
	return &resource, nil
}
