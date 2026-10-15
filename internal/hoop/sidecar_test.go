// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"net/http"
	"testing"
)

// TestGetSidecarReadsBoundRuleNames decodes bound_rules as the gateway writes
// them. The provider's fakes encode this package's own types, so a field the
// gateway spells differently only shows up against a literal payload.
func TestGetSidecarReadsBoundRuleNames(t *testing.T) {
	client, err := NewClient("http://localhost:8009/api", "hpk_token", ClientOptions{
		HttpClient: clientFunc(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, `{
  "id": "sc-1",
  "name": "payments",
  "configuration": {},
  "bound_rules": [
    {"kind": "guardrail", "rule_name": "no-destructive", "listener_name": "appdb"}
  ]
}`), nil
		}),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sc, err := client.GetSidecar("payments")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := SidecarRuleBinding{Kind: "guardrail", Name: "no-destructive", ListenerName: "appdb"}
	if len(sc.BoundRules) != 1 || sc.BoundRules[0] != want {
		t.Fatalf("bound_rules: got %+v, want [%+v]", sc.BoundRules, want)
	}
}
