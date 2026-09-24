// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"net/http"
	"strings"
	"testing"
)

func TestCreateAgentRetriesTransientRead(t *testing.T) {
	var getCalls int
	var deleteCalls int

	client, err := NewClient(testAPIURL, "xapi-hash", ClientOptions{
		HttpClient: clientFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodPost && req.URL.Path == "/api/agents":
				return testResponse(http.StatusCreated, `{"token":"grpcs://my-agent:secret@gateway.example:443?mode=standard"}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/agents/my-agent":
				getCalls++
				if getCalls == 1 {
					return testResponse(http.StatusInternalServerError, `{"message":"temporary"}`), nil
				}
				return testResponse(http.StatusOK, `{"id":"agent-id","name":"my-agent","mode":"standard","status":"DISCONNECTED","metadata":{}}`), nil
			case req.Method == http.MethodDelete:
				deleteCalls++
				return testResponse(http.StatusNoContent, ""), nil
			default:
				return testResponse(http.StatusInternalServerError, `{"message":"unexpected request"}`), nil
			}
		}),
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	agent, err := client.CreateAgent("my-agent", "standard")
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if getCalls != 2 {
		t.Fatalf("GET calls: got %d, want 2", getCalls)
	}
	if deleteCalls != 0 {
		t.Fatalf("DELETE calls: got %d, want 0", deleteCalls)
	}
	if agent.ID != "agent-id" {
		t.Fatalf("agent ID: got %q, want %q", agent.ID, "agent-id")
	}
	if !strings.Contains(agent.Token, "secret") {
		t.Fatalf("expected creation token to be preserved, got %q", agent.Token)
	}
}

func TestCreateAgentRollsBackAfterMalformedCreateResponse(t *testing.T) {
	var deleteCalls int

	client, err := NewClient(testAPIURL, "xapi-hash", ClientOptions{
		HttpClient: clientFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodPost && req.URL.Path == "/api/agents":
				return testResponse(http.StatusCreated, `{"token":`), nil
			case req.Method == http.MethodDelete && req.URL.Path == "/api/agents/my-agent":
				deleteCalls++
				return testResponse(http.StatusNoContent, ""), nil
			default:
				return testResponse(http.StatusInternalServerError, `{"message":"unexpected request"}`), nil
			}
		}),
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	_, err = client.CreateAgent("my-agent", "standard")
	if err == nil {
		t.Fatal("expected create to fail")
	}
	if deleteCalls != 1 {
		t.Fatalf("DELETE calls: got %d, want 1", deleteCalls)
	}
}

func TestCreateAgentRollsBackAfterPersistentReadFailure(t *testing.T) {
	var getCalls int
	var deleteCalls int

	client, err := NewClient(testAPIURL, "xapi-hash", ClientOptions{
		HttpClient: clientFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodPost && req.URL.Path == "/api/agents":
				return testResponse(http.StatusCreated, `{"token":"grpcs://my-agent:secret@gateway.example:443?mode=standard"}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/agents/my-agent":
				getCalls++
				return testResponse(http.StatusInternalServerError, `{"message":"temporary"}`), nil
			case req.Method == http.MethodDelete && req.URL.Path == "/api/agents/my-agent":
				deleteCalls++
				return testResponse(http.StatusNoContent, ""), nil
			default:
				return testResponse(http.StatusInternalServerError, `{"message":"unexpected request"}`), nil
			}
		}),
	})
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}

	_, err = client.CreateAgent("my-agent", "standard")
	if err == nil {
		t.Fatal("expected create to fail")
	}
	if getCalls != agentCreateReadAttempts {
		t.Fatalf("GET calls: got %d, want %d", getCalls, agentCreateReadAttempts)
	}
	if deleteCalls != 1 {
		t.Fatalf("DELETE calls: got %d, want 1", deleteCalls)
	}
}
