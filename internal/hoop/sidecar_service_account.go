// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SidecarServiceAccount maps platform-issued identity tokens to sidecars. A
// sidecar presenting a token from Issuer, for Audience, whose Claim matches
// SubjectPattern reaches the sidecar NameTemplate names, and the control plane
// creates that sidecar on the first handshake.
type SidecarServiceAccount struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	Issuer         string `json:"issuer"`
	Audience       string `json:"audience"`
	Claim          string `json:"claim"`
	SubjectPattern string `json:"subject_pattern"`
	NameTemplate   string `json:"name_template"`
	// JWKS is a static key set for an issuer the control plane cannot reach.
	// Nil means OIDC discovery on the issuer.
	JWKS                  json.RawMessage `json:"jwks,omitempty"`
	AllowAnySubject       bool            `json:"allow_any_subject"`
	AdoptExistingSidecars bool            `json:"adopt_existing_sidecars"`
}

func sidecarServiceAccountPath(id string) string {
	return "/sidecar-service-accounts/" + url.PathEscape(id)
}

func (c *Client) CreateSidecarServiceAccount(sa SidecarServiceAccount) (*SidecarServiceAccount, error) {
	body, err := json.Marshal(sa)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sidecar service account, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/sidecar-service-accounts", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSidecarServiceAccount(resp.Body)
}

func (c *Client) GetSidecarServiceAccount(id string) (*SidecarServiceAccount, error) {
	req, err := c.newRequest("GET", sidecarServiceAccountPath(id), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSidecarServiceAccount(resp.Body)
}

// UpdateSidecarServiceAccount replaces the mapping. The sidecars it already
// reached keep their names; a token the new pattern no longer matches stops
// authenticating.
func (c *Client) UpdateSidecarServiceAccount(sa SidecarServiceAccount) (*SidecarServiceAccount, error) {
	body, err := json.Marshal(sa)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sidecar service account, reason=%v", err)
	}
	req, err := c.newRequest("PUT", sidecarServiceAccountPath(sa.ID), body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSidecarServiceAccount(resp.Body)
}

func (c *Client) DeleteSidecarServiceAccount(id string) error {
	req, err := c.newRequest("DELETE", sidecarServiceAccountPath(id), nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeSidecarServiceAccount(body io.Reader) (*SidecarServiceAccount, error) {
	var out SidecarServiceAccount
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding sidecar service account resource, reason=%v", err)
	}
	return &out, nil
}
