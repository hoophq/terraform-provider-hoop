// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"net/http"
	"testing"
)

func TestResolveAuthScheme(t *testing.T) {
	for _, tt := range []struct {
		name      string
		scheme    AuthScheme
		token     string
		want      AuthScheme
		wantError bool
	}{
		{
			name:   "auto promotes an organization api key to bearer",
			scheme: AuthSchemeAuto,
			token:  "hpk_9Ny3rWQ0Xj0m0iSk3B2s4a5H6cJ8dL1pQ2rT3uV4wX8",
			want:   AuthSchemeBearer,
		},
		{
			name:   "the empty scheme behaves like auto",
			scheme: "",
			token:  "hpk_9Ny3rWQ0Xj0m0iSk3B2s4a5H6cJ8dL1pQ2rT3uV4wX8",
			want:   AuthSchemeBearer,
		},
		{
			name:   "auto keeps a legacy xapi key on the api-key header",
			scheme: AuthSchemeAuto,
			token:  "xapi-hash",
			want:   AuthSchemeAPIKey,
		},
		{
			name:   "auto keeps a legacy org-scoped key on the api-key header",
			scheme: AuthSchemeAuto,
			token:  "orgid|hash",
			want:   AuthSchemeAPIKey,
		},
		{
			// The gateway compares the legacy API_KEY byte for byte, so it can
			// be any operator-chosen string. Those must not be promoted.
			name:   "auto keeps an arbitrary legacy key on the api-key header",
			scheme: AuthSchemeAuto,
			token:  "VuOnc2nUwv8aCRhfQGsp",
			want:   AuthSchemeAPIKey,
		},
		{
			name:   "an explicit api_key scheme overrides detection",
			scheme: AuthSchemeAPIKey,
			token:  "hpk_9Ny3rWQ0Xj0m0iSk3B2s4a5H6cJ8dL1pQ2rT3uV4wX8",
			want:   AuthSchemeAPIKey,
		},
		{
			name:   "an explicit bearer scheme overrides detection",
			scheme: AuthSchemeBearer,
			token:  "VuOnc2nUwv8aCRhfQGsp",
			want:   AuthSchemeBearer,
		},
		{
			name:      "an unknown scheme is rejected",
			scheme:    AuthScheme("basic"),
			token:     "VuOnc2nUwv8aCRhfQGsp",
			wantError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAuthScheme(tt.scheme, tt.token)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected an error, got scheme %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got scheme %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSetAuthHeaderIsMutuallyExclusive is the regression test for EVL-156: the
// gateway auth middleware branches on the presence of the Api-Key header and
// never falls through to the bearer branch, so the two headers can never be
// sent together.
func TestSetAuthHeaderIsMutuallyExclusive(t *testing.T) {
	for _, tt := range []struct {
		name           string
		token          string
		scheme         AuthScheme
		wantAPIKey     string
		wantAuthHeader string
	}{
		{
			name:           "organization api key",
			token:          "hpk_token",
			scheme:         AuthSchemeAuto,
			wantAuthHeader: "Bearer hpk_token",
		},
		{
			name:       "legacy api key",
			token:      "xapi-hash",
			scheme:     AuthSchemeAuto,
			wantAPIKey: "xapi-hash",
		},
		{
			name:       "organization api key forced to the legacy header",
			token:      "hpk_token",
			scheme:     AuthSchemeAPIKey,
			wantAPIKey: "hpk_token",
		},
		{
			name:           "legacy api key forced to bearer",
			token:          "xapi-hash",
			scheme:         AuthSchemeBearer,
			wantAuthHeader: "Bearer xapi-hash",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("http://localhost:8009/api", tt.token, ClientOptions{AuthScheme: tt.scheme})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			req, err := client.newRequest("GET", "/connections", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := req.Header.Get("Api-Key"); got != tt.wantAPIKey {
				t.Errorf("Api-Key header: got %q, want %q", got, tt.wantAPIKey)
			}
			if got := req.Header.Get("Authorization"); got != tt.wantAuthHeader {
				t.Errorf("Authorization header: got %q, want %q", got, tt.wantAuthHeader)
			}
		})
	}
}

// TestSetAuthHeaderClearsTheOtherScheme guards against a request that is reused
// or pre-populated ending up with both credential headers.
func TestSetAuthHeaderClearsTheOtherScheme(t *testing.T) {
	client, err := NewClient("http://localhost:8009/api", "hpk_token", ClientOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, err := http.NewRequest("GET", "http://localhost:8009/api/connections", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req.Header.Set("Api-Key", "stale")
	client.setAuthHeader(req)

	if got := req.Header.Get("Api-Key"); got != "" {
		t.Errorf("Api-Key header should have been removed, got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer hpk_token" {
		t.Errorf("Authorization header: got %q, want %q", got, "Bearer hpk_token")
	}
}
