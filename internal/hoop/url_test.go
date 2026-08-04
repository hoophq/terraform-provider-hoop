// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"net/http"
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "an ordinary gateway url is untouched",
			in:   "http://localhost:8009/api",
			want: "http://localhost:8009/api",
		},
		{
			name: "userinfo is dropped",
			in:   "https://svc:s3cret@gw.example.com/api",
			want: "https://gw.example.com/api",
		},
		{
			name: "a username without a password is dropped too",
			in:   "https://svc@gw.example.com/api",
			want: "https://gw.example.com/api",
		},
		{
			name: "query parameters are dropped",
			in:   "https://gw.example.com/api?token=s3cret",
			want: "https://gw.example.com/api",
		},
		{
			name: "the fragment is dropped",
			in:   "https://gw.example.com/api#s3cret",
			want: "https://gw.example.com/api",
		},
		{
			name: "an unparsable url yields the placeholder",
			in:   "https://svc:p%ss3cret@gw.example.com/api",
			want: redactedURLPlaceholder,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := redactURL(tt.in)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "s3cret") {
				t.Errorf("the redacted url still carries the credential: %q", got)
			}
		})
	}
}

func TestRedactedAPIURLNeverExposesTheCredential(t *testing.T) {
	client, err := NewClient("https://svc:s3cret@gw.example.com/api", "hpk_token", ClientOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := client.RedactedAPIURL()
	if got != "https://gw.example.com/api" {
		t.Errorf("got %q, want %q", got, "https://gw.example.com/api")
	}
}

// TestNewRequestErrorKeepsTheURLOut guards the one path where net/url would
// otherwise print the api_url — credentials included — straight into a
// Terraform diagnostic. A control character in a resource name reaches it.
func TestNewRequestErrorKeepsTheURLOut(t *testing.T) {
	client, err := NewClient("https://svc:s3cret@gw.example.com/api", "hpk_token", ClientOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.newRequest(http.MethodGet, "/connections/pg\x7fdemo", nil)
	if err == nil {
		t.Fatal("expected a request construction error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("the error leaks the credential: %q", err)
	}
	if strings.Contains(err.Error(), "gw.example.com") {
		t.Errorf("the error leaks the gateway url: %q", err)
	}
	for _, want := range []string{"GET", "/connections/pg", "invalid control character"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should still name %q, got %q", want, err)
		}
	}
}
