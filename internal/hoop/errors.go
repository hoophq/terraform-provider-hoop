// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"fmt"
	"io"
	"net/http"
)

// APIError is returned when the Hoop Gateway answers with an unexpected status
// code.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Payload    string

	// AuthScheme is the credential header the request was sent with. It is what
	// makes the 401/403 hints actionable.
	AuthScheme AuthScheme
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("status=%v, payload=%v (%s %s)",
		e.StatusCode, e.Payload, e.Method, e.Path)
	if hint := e.hint(); hint != "" {
		msg += ". " + hint
	}
	return msg
}

// hint returns operator-facing guidance for the failure modes that are hard to
// diagnose from the gateway payload alone. It is empty for every other status.
func (e *APIError) hint() string {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		if e.AuthScheme == AuthSchemeBearer {
			return "The gateway rejected the organization API key sent as a bearer token. " +
				"Check that the key is still active under Settings > API Keys and that api_url points at the matching gateway. " +
				`If this value is instead the gateway's legacy API_KEY environment variable, set auth_scheme = "api_key" in the provider block.`
		}
		return "The gateway rejected the API key sent in the Api-Key header. " +
			"This header only works with the gateway's legacy API_KEY environment variable, and the value must match it exactly. " +
			`Organization API keys (prefixed hpk_) must be sent as a bearer token — remove auth_scheme = "api_key" to let the provider select the scheme.`
	case http.StatusForbidden:
		if e.AuthScheme == AuthSchemeBearer {
			return "The organization API key authenticated but is not allowed to perform this operation. " +
				"Organization API keys only carry the groups assigned to them: grant the key the admin group under Settings > API Keys to manage provider resources."
		}
	}
	return ""
}

// newAPIError drains resp and builds the error describing it. The caller keeps
// ownership of resp.Body.
func (c *Client) newAPIError(req *http.Request, resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed reading response body, status=%v, reason=%v",
			resp.StatusCode, err)
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		Method:     req.Method,
		Path:       req.URL.Path,
		Payload:    string(data),
		AuthScheme: c.authScheme,
	}
}
