// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Plugin struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
