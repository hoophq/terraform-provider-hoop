// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientOptions carries the optional settings of NewClient. The zero value is
// valid and yields a client with the default HTTP transport and automatic
// auth scheme detection.
type ClientOptions struct {
	// HttpClient overrides the transport used to issue requests.
	// Defaults to http.DefaultClient.
	HttpClient HttpClient
	// AuthScheme selects the credential header. Defaults to AuthSchemeAuto.
	AuthScheme AuthScheme
	// ProviderVersion is reported in the User-Agent header.
	ProviderVersion string
}

type Client struct {
	apiURL     string
	token      string
	authScheme AuthScheme
	userAgent  string
	httpClient HttpClient
}

func NewClient(apiURL, token string, opts ClientOptions) (*Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("the API key must not be empty")
	}
	authScheme, err := resolveAuthScheme(opts.AuthScheme, token)
	if err != nil {
		return nil, err
	}
	httpClient := opts.HttpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	version := opts.ProviderVersion
	if version == "" {
		version = "dev"
	}
	return &Client{
		apiURL:     strings.TrimSuffix(apiURL, "/"),
		token:      token,
		authScheme: authScheme,
		userAgent:  "terraform-provider-hoop/" + version,
		httpClient: httpClient,
	}, nil
}

// AuthScheme returns the concrete scheme this client authenticates with. It is
// never AuthSchemeAuto.
func (c *Client) AuthScheme() AuthScheme { return c.authScheme }

// RedactedAPIURL returns the gateway URL stripped of anything that could carry
// a credential. It is the only form of the URL that may be logged; the raw one
// is deliberately unreachable from outside the package.
func (c *Client) RedactedAPIURL() string { return redactURL(c.apiURL) }

// newRequest builds a request against the configured gateway. path is appended
// to the API URL and must start with a slash; a non-nil body is sent as JSON.
//
// This is the single place in the package allowed to construct a request or set
// a header — see TestRequestChokepoint. Before EVL-156 every endpoint set its
// own credential header, and each one of them hardcoded the legacy Api-Key
// header, so organization API keys failed with 401 across the board.
func (c *Client) newRequest(method, path string, body []byte) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.apiURL+path, reader)
	if err != nil {
		// urlErrorReason keeps the api_url out of the message: net/url reports
		// the offending URL verbatim, userinfo included, and this error reaches
		// the operator as a Terraform diagnostic.
		return nil, fmt.Errorf("failed to create request for %s %s, reason=%v",
			method, path, urlErrorReason(err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", c.userAgent)
	c.setAuthHeader(req)
	return req, nil
}

// send issues req and returns the response when its status code is one of
// wantStatus. Any other status becomes an *APIError. On success the caller owns
// closing the response body.
func (c *Client) send(req *http.Request, wantStatus ...int) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if slices.Contains(wantStatus, resp.StatusCode) {
		return resp, nil
	}
	defer resp.Body.Close()
	return nil, c.newAPIError(req, resp)
}

// sendDiscard issues req and drops the response body. It is the variant used by
// endpoints that answer with no content.
func (c *Client) sendDiscard(req *http.Request, wantStatus ...int) error {
	resp, err := c.send(req, wantStatus...)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
