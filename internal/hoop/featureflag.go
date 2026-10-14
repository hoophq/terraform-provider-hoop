// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// FeatureFlag is one entry of the gateway's flag catalog with the
// organization's current value. The catalog is compiled into the gateway;
// an organization only stores the flags it changed.
type FeatureFlag struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Default     bool     `json:"default"`
	Stability   string   `json:"stability"`
	Components  []string `json:"components"`
	Enabled     bool     `json:"enabled"`
}

type featureFlagUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

func (c *Client) ListFeatureFlags() ([]FeatureFlag, error) {
	req, err := c.newRequest("GET", "/feature-flags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding feature flags, reason=%v", err)
	}
	return out, nil
}

// GetFeatureFlag reads one flag. The gateway has no per-flag read, so this
// lists and picks. A name the catalog does not carry answers an *APIError
// with status 404, the way a missing resource does elsewhere, so a flag that
// a gateway upgrade retired drops out of state instead of failing the read.
func (c *Client) GetFeatureFlag(name string) (*FeatureFlag, error) {
	flags, err := c.ListFeatureFlags()
	if err != nil {
		return nil, err
	}
	for i := range flags {
		if flags[i].Name == name {
			return &flags[i], nil
		}
	}
	return nil, &APIError{
		StatusCode: http.StatusNotFound,
		Method:     "GET",
		Path:       "/feature-flags/" + name,
		Payload:    fmt.Sprintf(`{"message":"unknown feature flag: %s"}`, name),
		AuthScheme: c.authScheme,
	}
}

// SetFeatureFlag writes the organization's value. The gateway answers 400
// for a name outside its catalog.
func (c *Client) SetFeatureFlag(name string, enabled bool) (*FeatureFlag, error) {
	body, err := json.Marshal(featureFlagUpdateRequest{Enabled: enabled})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal feature flag, reason=%v", err)
	}
	req, err := c.newRequest("PUT", "/feature-flags/"+url.PathEscape(name), body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out FeatureFlag
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding feature flag, reason=%v", err)
	}
	return &out, nil
}
