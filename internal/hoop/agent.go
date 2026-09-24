// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	agentCreateReadAttempts   = 3
	agentCreateReadRetryDelay = 100 * time.Millisecond
)

type Agent struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Mode     string            `json:"mode"`
	Token    string            `json:"token"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata"`
}

type AgentCreateRequest struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type AgentCreateResponse struct {
	Token string `json:"token"`
}

func (c *Client) GetAgent(nameOrID string) (*Agent, error) {
	req, err := c.newRequest("GET", "/agents/"+nameOrID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeAgent(resp.Body)
}

func (c *Client) CreateAgent(name, mode string) (*Agent, error) {
	body, err := json.Marshal(AgentCreateRequest{Name: name, Mode: mode})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent, reason=%v", err)
	}

	req, err := c.newRequest("POST", "/agents", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var created AgentCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		createErr := fmt.Errorf("failed decoding agent create response, reason=%v", err)
		return nil, c.rollbackCreatedAgent(name, createErr)
	}

	agent, err := c.getAgentAfterCreate(name)
	if err != nil {
		createErr := fmt.Errorf("failed reading newly created agent %q: %w", name, err)
		return nil, c.rollbackCreatedAgent(name, createErr)
	}
	agent.Token = created.Token
	return agent, nil
}

func (c *Client) getAgentAfterCreate(name string) (*Agent, error) {
	var lastErr error
	for attempt := 0; attempt < agentCreateReadAttempts; attempt++ {
		agent, err := c.GetAgent(name)
		if err == nil {
			return agent, nil
		}
		lastErr = err
		if !isRetriableAgentRead(err) || attempt == agentCreateReadAttempts-1 {
			break
		}
		time.Sleep(agentCreateReadRetryDelay)
	}
	return nil, lastErr
}

func isRetriableAgentRead(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		// Transport errors are transient from the provider's perspective.
		return true
	}

	return apiErr.StatusCode == http.StatusNotFound ||
		apiErr.StatusCode == http.StatusRequestTimeout ||
		apiErr.StatusCode == http.StatusTooManyRequests ||
		apiErr.StatusCode >= http.StatusInternalServerError
}

func (c *Client) rollbackCreatedAgent(name string, createErr error) error {
	cleanupErr := c.DeleteAgent(name)
	if cleanupErr == nil {
		return createErr
	}

	var apiErr *APIError
	if errors.As(cleanupErr, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return createErr
	}

	return fmt.Errorf("%w; additionally failed to delete newly created agent %q: %v", createErr, name, cleanupErr)
}

func (c *Client) DeleteAgent(nameOrID string) error {
	req, err := c.newRequest("DELETE", "/agents/"+nameOrID, nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeAgent(responseBody io.Reader) (*Agent, error) {
	var agent Agent
	if err := json.NewDecoder(responseBody).Decode(&agent); err != nil {
		return nil, fmt.Errorf("failed decoding agent resource, reason=%v", err)
	}
	return &agent, nil
}
