// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ServerAuthConfig is the gateway's one authentication row. PUT replaces the
// whole row: oidc_config and saml_config are rebuilt from the body, so a key
// left out is stored empty. rollout_api_key is not carried: sending it starts
// an api key rollout, which is not this client's to do.
type ServerAuthConfig struct {
	AuthMethod                  string                `json:"auth_method"`
	OidcConfig                  *ServerAuthOidcConfig `json:"oidc_config"`
	SamlConfig                  *ServerAuthSamlConfig `json:"saml_config"`
	ProviderName                string                `json:"provider_name"`
	ApiKey                      *string               `json:"api_key,omitempty"`
	WebappUsersManagementStatus string                `json:"webapp_users_management_status"`
	AdminRoleName               string                `json:"admin_role_name"`
	AuditorRoleName             string                `json:"auditor_role_name"`
}

type ServerAuthOidcConfig struct {
	IssuerURL           string   `json:"issuer_url"`
	ClientID            string   `json:"client_id"`
	ClientSecret        string   `json:"client_secret"`
	ClientAssertionFile string   `json:"client_assertion_file,omitempty"`
	Scopes              []string `json:"scopes"`
	Audience            string   `json:"audience"`
	GroupsClaim         string   `json:"groups_claim"`
}

type ServerAuthSamlConfig struct {
	IdpMetadataURL   string                          `json:"idp_metadata_url"`
	GroupsClaim      string                          `json:"groups_claim"`
	ResolvedMetadata *ServerAuthSamlResolvedMetadata `json:"resolved_metadata,omitempty"`
}

// ServerAuthSamlResolvedMetadata is the gateway's snapshot of the identity
// provider the metadata URL resolved to on the last write. Read only.
type ServerAuthSamlResolvedMetadata struct {
	EntityID             string `json:"entity_id"`
	SsoURL               string `json:"sso_url"`
	CertificateExpiresAt string `json:"certificate_expires_at"`
	ResolvedAt           string `json:"resolved_at"`
}

// GetServerAuthConfig reads the row. With no row stored the gateway answers
// the configuration it runs from its environment, in the same shape.
func (c *Client) GetServerAuthConfig() (*ServerAuthConfig, error) {
	req, err := c.newRequest("GET", "/serverconfig/auth", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out ServerAuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding server auth config, reason=%v", err)
	}
	return &out, nil
}

// UpdateServerAuthConfig replaces the row and returns what the gateway stored.
func (c *Client) UpdateServerAuthConfig(cfg *ServerAuthConfig) (*ServerAuthConfig, error) {
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server auth config, reason=%v", err)
	}
	req, err := c.newRequest("PUT", "/serverconfig/auth", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out ServerAuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding server auth config, reason=%v", err)
	}
	return &out, nil
}
