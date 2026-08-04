// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
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
			calls := 0
			transport := clientFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if got := req.Header.Get("Api-Key"); got != tt.wantAPIKey {
					t.Errorf("%s %s: Api-Key header: got %q, want %q", req.Method, req.URL.Path, got, tt.wantAPIKey)
				}
				if got := req.Header.Get("Authorization"); got != tt.wantAuthHeader {
					t.Errorf("%s %s: Authorization header: got %q, want %q", req.Method, req.URL.Path, got, tt.wantAuthHeader)
				}
				// Any payload is fine here, the assertions are on the request.
				return testResponse(http.StatusInternalServerError, `{"message":"unused"}`), nil
			})

			client, err := NewClient("http://localhost:8009/api", tt.token, ClientOptions{HttpClient: transport})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			endpoints := []func(){
				func() { _, _ = client.GetConnection("pgdemo") },
				func() { _, _ = client.CreateConnection(Connection{Name: "pgdemo"}) },
				func() { _, _ = client.UpdateConnection(Connection{Name: "pgdemo"}) },
				func() { _ = client.DeleteConnection("pgdemo") },
				func() { _, _ = client.GetUser("john@hoop.dev") },
				func() { _, _ = client.CreateUser("john@hoop.dev", "active", nil) },
				func() { _, _ = client.UpdateUser("john@hoop.dev", "active", nil) },
				func() { _ = client.DeleteUser("john@hoop.dev") },
				func() { _, _ = client.GetPlugin("runbooks") },
				func() { _, _ = client.GetPluginConfig("runbooks") },
				func() { _, _ = client.upsertPluginConfig("runbooks", nil) },
				func() { _, _ = client.GetPluginConnection("runbooks", "id") },
				func() { _, _ = client.UpsertPluginConnection("runbooks", "id", nil) },
				func() { _ = client.DeletePluginConnection("runbooks", "id") },
				func() { _, _ = client.GetDatamaskingRule("id") },
				func() { _, _ = client.CreateDatamaskingRule(DataMaskingRule{}) },
				func() { _, _ = client.UpdateDatamaskingRule(DataMaskingRule{}) },
				func() { _ = client.DeleteDatamaskingRule("id") },
				func() { _, _ = client.GetRunbookConfigByURL("https://github.com/hoophq/runbooks.git") },
				func() { _, _ = client.CreateRunbookRepo(RunbookRepo{}) },
				func() { _, _ = client.UpdateRunbookRepoByID(RunbookRepo{}) },
				func() { _ = client.DeleteRunbookRepoByID("https://github.com/hoophq/runbooks.git") },
				func() { _, _ = client.GetRunbookRuleByID("id") },
				func() { _, _ = client.CreateRunbookRule(RunbookRule{}) },
				func() { _, _ = client.UpdateRunbookRuleByID(RunbookRule{}) },
				func() { _ = client.DeleteRunbookRuleByID("id") },
			}
			for _, call := range endpoints {
				call()
			}

			if calls != len(endpoints) {
				t.Errorf("issued %d requests for %d endpoints", calls, len(endpoints))
			}
		})
	}
}
