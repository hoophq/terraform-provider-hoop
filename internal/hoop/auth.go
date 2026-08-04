// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"fmt"
	"net/http"
	"strings"
)

// AuthScheme selects which credential header the client sends to the gateway.
type AuthScheme string

const (
	// AuthSchemeAuto derives the header from the credential format: organization
	// API keys are sent as a bearer token, anything else keeps the legacy
	// Api-Key header. It is the default.
	AuthSchemeAuto AuthScheme = "auto"
	// AuthSchemeBearer always sends `Authorization: Bearer <api_key>`.
	AuthSchemeBearer AuthScheme = "bearer"
	// AuthSchemeAPIKey always sends the legacy `Api-Key: <api_key>` header.
	AuthSchemeAPIKey AuthScheme = "api_key"
)

// AuthSchemes lists every accepted AuthScheme value, in documentation order.
var AuthSchemes = []string{
	string(AuthSchemeAuto),
	string(AuthSchemeBearer),
	string(AuthSchemeAPIKey),
}

// orgAPIKeyPrefix is the fixed prefix of gateway-minted organization API keys.
const orgAPIKeyPrefix = "hpk_"

// isOrgAPIKey reports whether token is a gateway-minted organization API key.
//
// Only this closed family switches to bearer auth. The legacy credential is an
// arbitrary operator-chosen string that the gateway compares byte for byte
// against its own API_KEY setting, so it has no recognizable shape — treating
// anything but an hpk_ key as legacy keeps every existing configuration on the
// exact header it works with today.
func isOrgAPIKey(token string) bool {
	return strings.HasPrefix(token, orgAPIKeyPrefix)
}

// resolveAuthScheme turns the configured scheme into the concrete scheme used
// on the wire. It never returns AuthSchemeAuto.
func resolveAuthScheme(scheme AuthScheme, token string) (AuthScheme, error) {
	switch scheme {
	case AuthSchemeBearer, AuthSchemeAPIKey:
		return scheme, nil
	case "", AuthSchemeAuto:
		if isOrgAPIKey(token) {
			return AuthSchemeBearer, nil
		}
		return AuthSchemeAPIKey, nil
	}
	return "", fmt.Errorf("unknown auth scheme %q, must be one of: %s",
		scheme, strings.Join(AuthSchemes, ", "))
}

// setAuthHeader sets the credential header on req.
//
// The two headers are mutually exclusive by design. The gateway's auth
// middleware branches on the *presence* of the Api-Key header and never falls
// through to the bearer branch, so sending both makes an organization API key
// fail with 401 even though the bearer token is valid.
func (c *Client) setAuthHeader(req *http.Request) {
	if c.authScheme == AuthSchemeAPIKey {
		req.Header.Del("Authorization")
		req.Header.Set("Api-Key", c.token)
		return
	}
	req.Header.Del("Api-Key")
	req.Header.Set("Authorization", "Bearer "+c.token)
}
