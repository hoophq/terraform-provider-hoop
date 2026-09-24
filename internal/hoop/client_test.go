// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type clientFunc func(req *http.Request) (*http.Response, error)

func (f clientFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func testResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestNewClientRejectsAnEmptyToken(t *testing.T) {
	for _, token := range []string{"", "   "} {
		if _, err := NewClient("http://localhost:8009/api", token, ClientOptions{}); err == nil {
			t.Errorf("expected an error for token %q", token)
		}
	}
}

func TestNewClientRejectsAnUnknownAuthScheme(t *testing.T) {
	_, err := NewClient("http://localhost:8009/api", "xapi-hash", ClientOptions{AuthScheme: "basic"})
	if err == nil {
		t.Fatal("expected an error for an unknown auth scheme")
	}
	if !strings.Contains(err.Error(), "basic") {
		t.Errorf("the error should name the rejected scheme, got %q", err)
	}
}

func TestNewClientTrimsTheToken(t *testing.T) {
	client, err := NewClient("http://localhost:8009/api", "  hpk_token\n", ClientOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.AuthScheme() != AuthSchemeBearer {
		t.Fatalf("got scheme %q, want %q", client.AuthScheme(), AuthSchemeBearer)
	}
	req, err := client.newRequest("GET", "/connections", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer hpk_token" {
		t.Errorf("Authorization header: got %q, want %q", got, "Bearer hpk_token")
	}
}

func TestNewRequest(t *testing.T) {
	client, err := NewClient("http://localhost:8009/api/", "xapi-hash", ClientOptions{ProviderVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("without a body", func(t *testing.T) {
		req, err := client.newRequest("DELETE", "/connections/pgdemo", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := req.URL.String(), "http://localhost:8009/api/connections/pgdemo"; got != want {
			t.Errorf("url: got %q, want %q", got, want)
		}
		if got := req.Header.Get("Content-Type"); got != "" {
			t.Errorf("a bodyless request must not declare a content type, got %q", got)
		}
		if got, want := req.Header.Get("User-Agent"), "terraform-provider-hoop/1.2.3"; got != want {
			t.Errorf("user agent: got %q, want %q", got, want)
		}
	})

	t.Run("with a body", func(t *testing.T) {
		req, err := client.newRequest("POST", "/connections", []byte(`{"name":"pgdemo"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := req.Header.Get("Content-Type"), "application/json"; got != want {
			t.Errorf("content type: got %q, want %q", got, want)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := string(body), `{"name":"pgdemo"}`; got != want {
			t.Errorf("body: got %q, want %q", got, want)
		}
	})
}

func TestSendUnexpectedStatusYieldsAnAPIError(t *testing.T) {
	client, err := NewClient("http://localhost:8009/api", "hpk_token", ClientOptions{
		HttpClient: clientFunc(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusUnauthorized, `{"message":"access denied"}`), nil
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetConnection("pgdemo")
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.Method != "GET" || apiErr.Path != "/api/connections/pgdemo" {
		t.Errorf("got %s %s, want GET /api/connections/pgdemo", apiErr.Method, apiErr.Path)
	}
	if apiErr.AuthScheme != AuthSchemeBearer {
		t.Errorf("auth scheme: got %q, want %q", apiErr.AuthScheme, AuthSchemeBearer)
	}
	if !strings.HasPrefix(apiErr.Error(), `status=401, payload={"message":"access denied"}`) {
		t.Errorf("the message should keep the status/payload prefix, got %q", apiErr.Error())
	}
}

func TestAPIErrorHints(t *testing.T) {
	for _, tt := range []struct {
		name       string
		statusCode int
		scheme     AuthScheme
		wantHint   string
	}{
		{
			name:       "401 with a bearer token points at the organization api key",
			statusCode: http.StatusUnauthorized,
			scheme:     AuthSchemeBearer,
			wantHint:   "organization API key sent as a bearer token",
		},
		{
			name:       "401 with the legacy header points at the gateway API_KEY",
			statusCode: http.StatusUnauthorized,
			scheme:     AuthSchemeAPIKey,
			wantHint:   "legacy API_KEY environment variable",
		},
		{
			name:       "403 with a bearer token points at the key groups",
			statusCode: http.StatusForbidden,
			scheme:     AuthSchemeBearer,
			wantHint:   "grant the key the admin group",
		},
		{
			name:       "403 with the legacy header has no hint",
			statusCode: http.StatusForbidden,
			scheme:     AuthSchemeAPIKey,
		},
		{
			name:       "404 has no hint",
			statusCode: http.StatusNotFound,
			scheme:     AuthSchemeBearer,
		},
		{
			name:       "500 has no hint",
			statusCode: http.StatusInternalServerError,
			scheme:     AuthSchemeBearer,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{
				StatusCode: tt.statusCode,
				Method:     "GET",
				Path:       "/api/connections",
				Payload:    `{"message":"nope"}`,
				AuthScheme: tt.scheme,
			}
			hint := err.hint()
			if tt.wantHint == "" {
				if hint != "" {
					t.Errorf("expected no hint, got %q", hint)
				}
				return
			}
			if !strings.Contains(hint, tt.wantHint) {
				t.Errorf("hint %q does not mention %q", hint, tt.wantHint)
			}
			if !strings.Contains(err.Error(), hint) {
				t.Errorf("the error message should carry the hint, got %q", err.Error())
			}
		})
	}
}

// Fixtures shared by the endpoint table. The git URL is hashed into the request
// path by the runbook endpoints, so the expectation derives it the same way.
const (
	testAPIURL     = "http://localhost:8009/api"
	testAgentName  = "terraform-agent"
	testConnName   = "pgdemo"
	testUserEmail  = "john@hoop.dev"
	testPluginName = "runbooks"
	testConnID     = "conn-id"
	testRuleID     = "rule-id"
	testGitURL     = "https://github.com/hoophq/runbooks.git"
)

func testRunbookRepoID() string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(testGitURL)).String()
}

// endpointCall drives one *Client method and spells out every request it must
// issue, in order.
//
// The expectation is written per endpoint rather than counted in aggregate. A
// method that gives up before reaching its own request — as UpdateUser does
// when the GetUser it chains fails — still contributes one request to a total,
// so a bare count cannot tell "the PUT went out" from "the PUT never happened".
type endpointCall struct {
	// method is the *Client method name, cross-checked against the type by
	// TestEveryExportedEndpointIsCovered.
	method string
	call   func(*Client)
	wants  []string
}

func clientEndpoints() []endpointCall {
	repoID := testRunbookRepoID()
	return []endpointCall{
		{"GetAgent", func(c *Client) { _, _ = c.GetAgent(testAgentName) },
			[]string{"GET /api/agents/" + testAgentName}},
		{"CreateAgent", func(c *Client) { _, _ = c.CreateAgent(testAgentName, "standard") },
			[]string{"POST /api/agents", "GET /api/agents/" + testAgentName}},
		{"DeleteAgent", func(c *Client) { _ = c.DeleteAgent(testAgentName) },
			[]string{"DELETE /api/agents/" + testAgentName}},

		{"GetConnection", func(c *Client) { _, _ = c.GetConnection(testConnName) },
			[]string{"GET /api/connections/" + testConnName}},
		{"CreateConnection", func(c *Client) { _, _ = c.CreateConnection(Connection{Name: testConnName}) },
			[]string{"POST /api/connections"}},
		{"UpdateConnection", func(c *Client) { _, _ = c.UpdateConnection(Connection{Name: testConnName}) },
			[]string{"PUT /api/connections/" + testConnName}},
		{"DeleteConnection", func(c *Client) { _ = c.DeleteConnection(testConnName) },
			[]string{"DELETE /api/connections/" + testConnName}},

		{"GetUser", func(c *Client) { _, _ = c.GetUser(testUserEmail) },
			[]string{"GET /api/users/" + testUserEmail}},
		{"CreateUser", func(c *Client) { _, _ = c.CreateUser(testUserEmail, "active", nil) },
			[]string{"POST /api/users"}},
		// UpdateUser reads the user before writing it back.
		{"UpdateUser", func(c *Client) { _, _ = c.UpdateUser(testUserEmail, "active", nil) },
			[]string{"GET /api/users/" + testUserEmail, "PUT /api/users/" + testUserEmail}},
		{"DeleteUser", func(c *Client) { _ = c.DeleteUser(testUserEmail) },
			[]string{"DELETE /api/users/" + testUserEmail}},

		{"GetPlugin", func(c *Client) { _, _ = c.GetPlugin(testPluginName) },
			[]string{"GET /api/plugins/" + testPluginName}},
		{"GetPluginConfig", func(c *Client) { _, _ = c.GetPluginConfig(testPluginName) },
			[]string{"GET /api/plugins/" + testPluginName}},
		// CreatePluginConfig stops at its existence check: GetPlugin never
		// reports a missing plugin, so the upsert behind it is unreachable.
		{"CreatePluginConfig", func(c *Client) { _, _ = c.CreatePluginConfig(testPluginName, nil) },
			[]string{"GET /api/plugins/" + testPluginName}},
		{"UpdatePluginConfig", func(c *Client) { _, _ = c.UpdatePluginConfig(testPluginName, nil) },
			[]string{"GET /api/plugins/" + testPluginName, "PUT /api/plugins/" + testPluginName + "/config"}},
		{"DeletePluginConfig", func(c *Client) { _ = c.DeletePluginConfig(testPluginName) },
			[]string{"GET /api/plugins/" + testPluginName, "PUT /api/plugins/" + testPluginName + "/config"}},
		{"upsertPluginConfig", func(c *Client) { _, _ = c.upsertPluginConfig(testPluginName, nil) },
			[]string{"PUT /api/plugins/" + testPluginName + "/config"}},

		{"GetPluginConnection", func(c *Client) { _, _ = c.GetPluginConnection(testPluginName, testConnID) },
			[]string{"GET /api/plugins/" + testPluginName + "/conn/" + testConnID}},
		{"UpsertPluginConnection", func(c *Client) { _, _ = c.UpsertPluginConnection(testPluginName, testConnID, nil) },
			[]string{"PUT /api/plugins/" + testPluginName + "/conn/" + testConnID}},
		{"DeletePluginConnection", func(c *Client) { _ = c.DeletePluginConnection(testPluginName, testConnID) },
			[]string{"DELETE /api/plugins/" + testPluginName + "/conn/" + testConnID}},

		{"GetDatamaskingRule", func(c *Client) { _, _ = c.GetDatamaskingRule(testRuleID) },
			[]string{"GET /api/datamasking-rules/" + testRuleID}},
		{"CreateDatamaskingRule", func(c *Client) { _, _ = c.CreateDatamaskingRule(DataMaskingRule{}) },
			[]string{"POST /api/datamasking-rules"}},
		{"UpdateDatamaskingRule", func(c *Client) { _, _ = c.UpdateDatamaskingRule(DataMaskingRule{ID: testRuleID}) },
			[]string{"PUT /api/datamasking-rules/" + testRuleID}},
		{"DeleteDatamaskingRule", func(c *Client) { _ = c.DeleteDatamaskingRule(testRuleID) },
			[]string{"DELETE /api/datamasking-rules/" + testRuleID}},

		{"GetRunbookConfigByURL", func(c *Client) { _, _ = c.GetRunbookConfigByURL(testGitURL) },
			[]string{"GET /api/runbooks/configurations"}},
		{"CreateRunbookRepo", func(c *Client) { _, _ = c.CreateRunbookRepo(RunbookRepo{GitURL: testGitURL}) },
			[]string{"POST /api/runbooks/configurations"}},
		{"UpdateRunbookRepoByID", func(c *Client) { _, _ = c.UpdateRunbookRepoByID(RunbookRepo{GitURL: testGitURL}) },
			[]string{"PUT /api/runbooks/configurations/" + repoID}},
		{"DeleteRunbookRepoByID", func(c *Client) { _ = c.DeleteRunbookRepoByID(testGitURL) },
			[]string{"DELETE /api/runbooks/configurations/" + repoID}},

		{"GetRunbookRuleByID", func(c *Client) { _, _ = c.GetRunbookRuleByID(testRuleID) },
			[]string{"GET /api/runbooks/rules/" + testRuleID}},
		{"CreateRunbookRule", func(c *Client) { _, _ = c.CreateRunbookRule(RunbookRule{}) },
			[]string{"POST /api/runbooks/rules"}},
		{"UpdateRunbookRuleByID", func(c *Client) { _, _ = c.UpdateRunbookRuleByID(RunbookRule{ID: testRuleID}) },
			[]string{"PUT /api/runbooks/rules/" + testRuleID}},
		{"DeleteRunbookRuleByID", func(c *Client) { _ = c.DeleteRunbookRuleByID(testRuleID) },
			[]string{"DELETE /api/runbooks/rules/" + testRuleID}},
	}
}

// endpointSuccessStatus answers each request with the status its endpoint
// treats as success, so a chained endpoint reaches the request it exists for
// instead of bailing out on the first one.
func endpointSuccessStatus(method string) int {
	switch method {
	case "POST":
		return http.StatusCreated
	case "DELETE":
		return http.StatusNoContent
	default:
		return http.StatusOK
	}
}

// TestEveryEndpointSendsTheResolvedCredential walks the whole client surface to
// prove no endpoint was missed when request construction was centralised.
func TestEveryEndpointSendsTheResolvedCredential(t *testing.T) {
	for _, tt := range []struct {
		name           string
		token          string
		wantAPIKey     string
		wantAuthHeader string
	}{
		{
			name:           "organization api key",
			token:          "hpk_token",
			wantAuthHeader: "Bearer hpk_token",
		},
		{
			name:       "legacy api key",
			token:      "xapi-hash",
			wantAPIKey: "xapi-hash",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, endpoint := range clientEndpoints() {
				t.Run(endpoint.method, func(t *testing.T) {
					var got []string
					transport := clientFunc(func(req *http.Request) (*http.Response, error) {
						got = append(got, req.Method+" "+req.URL.Path)
						if v := req.Header.Get("Api-Key"); v != tt.wantAPIKey {
							t.Errorf("%s %s: Api-Key header: got %q, want %q", req.Method, req.URL.Path, v, tt.wantAPIKey)
						}
						if v := req.Header.Get("Authorization"); v != tt.wantAuthHeader {
							t.Errorf("%s %s: Authorization header: got %q, want %q", req.Method, req.URL.Path, v, tt.wantAuthHeader)
						}
						return testResponse(endpointSuccessStatus(req.Method), "{}"), nil
					})

					client, err := NewClient(testAPIURL, tt.token, ClientOptions{HttpClient: transport})
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					endpoint.call(client)

					if !slices.Equal(got, endpoint.wants) {
						t.Errorf("requests issued: got %v, want %v", got, endpoint.wants)
					}
				})
			}
		})
	}
}

// TestEveryExportedEndpointIsCovered fails when a method is added to the client
// without an entry in clientEndpoints, so credential coverage cannot quietly
// fall behind the API surface.
func TestEveryExportedEndpointIsCovered(t *testing.T) {
	// Methods that deliberately issue no request.
	noRequest := map[string]bool{
		"AuthScheme":     true,
		"RedactedAPIURL": true,
	}

	covered := map[string]bool{}
	for _, endpoint := range clientEndpoints() {
		covered[endpoint.method] = true
	}

	clientType := reflect.TypeOf(&Client{})
	for i := range clientType.NumMethod() {
		name := clientType.Method(i).Name
		if noRequest[name] || covered[name] {
			continue
		}
		t.Errorf("exported client method %s has no entry in clientEndpoints", name)
	}
}
