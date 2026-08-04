// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
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
	req, err := c.newRequest("GET", "/users/"+userEmail, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeUser(resp.Body)
}

func (c *Client) CreateUser(email, status string, groups []string) (*User, error) {
	body, err := json.Marshal(User{
		Email:  email,
		Status: status,
		Groups: groups,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/users", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeUser(resp.Body)
}

func (c *Client) UpdateUser(userEmail, status string, groups []string) (*User, error) {
	user, err := c.GetUser(userEmail)
	if err != nil {
		return nil, err
	}
	user.Status = status
	user.Groups = groups

	body, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user, reason=%v", err)
	}
	req, err := c.newRequest("PUT", "/users/"+userEmail, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeUser(resp.Body)
}

func (c *Client) DeleteUser(userID string) error {
	req, err := c.newRequest("DELETE", "/users/"+userID, nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

func decodeUser(responseBody io.Reader) (*User, error) {
	var resource User
	if err := json.NewDecoder(responseBody).Decode(&resource); err != nil {
		return nil, fmt.Errorf("failed decoding user resource, reason=%v", err)
	}
	return &resource, nil
}
