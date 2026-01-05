// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type User struct {
	ID      string   `json:"id,omitempty"`
	Email   string   `json:"email"`
	Status  string   `json:"status"`
	Groups  []string `json:"groups"`
	Name    string   `json:"name"`
	Picture string   `json:"picture"`
	SlackID string   `json:"slack_id"`
}

func (c *Client) GetUser(userEmail string) (*User, error) {
	apiURL := c.apiURL + "/users/" + userEmail
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var resource User
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding user resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) CreateUser(email, status string, groups []string) (*User, error) {
	apiURL := c.apiURL + "/users"
	body, err := json.Marshal(User{
		Email:  email,
		Status: status,
		Groups: groups,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user, reason=%v", err)
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		var resource User
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding user resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) UpdateUser(userEmail, status string, groups []string) (*User, error) {
	user, err := c.GetUser(userEmail)
	if err != nil {
		return nil, err
	}
	user.Status = status
	user.Groups = groups

	apiURL := fmt.Sprintf("%s/users/%s", c.apiURL, userEmail)
	body, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user, reason=%v", err)
	}
	req, err := http.NewRequest("PUT", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update user, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var resource User
		err := json.NewDecoder(resp.Body).Decode(&resource)
		if err != nil {
			return nil, fmt.Errorf("failed decoding user resource, reason=%v", err)
		}
		return &resource, nil
	}
	return nil, validateErr(resp)
}

func (c *Client) DeleteUser(userID string) error {
	apiURL := fmt.Sprintf("%s/users/%s", c.apiURL, userID)
	req, err := http.NewRequest("DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request, reason=%v", err)
	}
	req.Header.Set("Api-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete user, reason=%v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return validateErr(resp)
}
