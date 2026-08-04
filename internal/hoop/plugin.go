// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Plugin struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Config *PluginConfig `json:"config"`
}

type PluginConfig struct {
	ID      string            `json:"id"`
	EnvVars map[string]string `json:"envvars"`
}

func (c *Client) GetPlugin(name string) (*Plugin, error) {
	req, err := c.newRequest("GET", "/plugins/"+name, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var resource Plugin
	if err := json.NewDecoder(resp.Body).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding plugin resource, reason=%v", err)
	}
	return &resource, nil
}

func (c *Client) GetPluginConfig(pluginName string) (*PluginConfig, error) {
	req, err := c.newRequest("GET", "/plugins/"+pluginName, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePluginConfig(resp.Body)
}

func (c *Client) CreatePluginConfig(pluginName string, config map[string]string) (*PluginConfig, error) {
	pl, err := c.GetPlugin(pluginName)
	if err != nil {
		return nil, fmt.Errorf("failed to validate if plugin %s exists, reason=%v", pluginName, err)
	}
	if pl != nil {
		return nil, fmt.Errorf("plugin %s already exists", pluginName)
	}
	return c.upsertPluginConfig(pluginName, config)
}

func (c *Client) UpdatePluginConfig(pluginName string, config map[string]string) (*PluginConfig, error) {
	pl, err := c.GetPlugin(pluginName)
	if err != nil {
		return nil, fmt.Errorf("failed to validate if plugin %s exists, reason=%v", pluginName, err)
	}
	if pl == nil {
		return nil, fmt.Errorf("plugin %s does not exist", pluginName)
	}
	return c.upsertPluginConfig(pluginName, config)
}

func (c *Client) DeletePluginConfig(pluginName string) error {
	pl, err := c.GetPlugin(pluginName)
	if err != nil {
		return fmt.Errorf("failed to validate if plugin %s exists, reason=%v", pluginName, err)
	}
	if pl == nil {
		return fmt.Errorf("plugin %s does not exist", pluginName)
	}
	_, err = c.upsertPluginConfig(pluginName, nil)
	return err
}

func (c *Client) upsertPluginConfig(pluginName string, config map[string]string) (*PluginConfig, error) {
	newConfig := map[string]string{}
	for key, val := range config {
		newConfig[key] = base64.StdEncoding.EncodeToString([]byte(val))
	}
	configJSON, err := json.Marshal(newConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin config, reason=%v", err)
	}

	req, err := c.newRequest("PUT", "/plugins/"+pluginName+"/config", configJSON)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodePluginConfig(resp.Body)
}

// decodePluginConfig decodes a plugin resource and returns its configuration
// with the environment variable values base64-decoded. It returns a nil config
// when the plugin has none.
func decodePluginConfig(responseBody io.Reader) (*PluginConfig, error) {
	var resource Plugin
	if err := json.NewDecoder(responseBody).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding plugin config resource, reason=%v", err)
	}
	if resource.Config == nil {
		return nil, nil
	}
	pluginConfig := resource.Config
	for key, val := range pluginConfig.EnvVars {
		decoded, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return nil, fmt.Errorf("failed decoding plugin config value for key %s, reason=%v", key, err)
		}
		pluginConfig.EnvVars[key] = string(decoded)
	}
	return pluginConfig, nil
}
