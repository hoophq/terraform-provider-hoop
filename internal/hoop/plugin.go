// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	apiURL := c.apiURL + "/plugins/" + name
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
		var resource Plugin
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding plugin resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) GetPluginConfig(pluginName string) (*PluginConfig, error) {
	apiURL := fmt.Sprintf("%s/plugins/%s", c.apiURL, pluginName)
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
		var resource Plugin
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
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
	return nil, validateErr(resp)
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
	apiURL := fmt.Sprintf("%s/plugins/%s/config", c.apiURL, pluginName)
	newConfig := map[string]string{}
	for key, val := range config {
		newConfig[key] = base64.StdEncoding.EncodeToString([]byte(val))
	}
	configJSON, err := json.Marshal(newConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plugin config, reason=%v", err)
	}

	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(configJSON))
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
		var resource Plugin
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
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
	return nil, validateErr(resp)
}
