package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hoophq/terraform-provider-hoop/models"
)

type Client struct {
	ApiKey     string
	ApiUrl     string
	HttpClient *http.Client
}

func NewClient(apiUrl, apiKey string) *Client {
	return &Client{
		ApiKey:     apiKey,
		ApiUrl:     apiUrl,
		HttpClient: &http.Client{},
	}
}

func (c *Client) GetConnection(ctx context.Context, name string) (*models.Connection, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/connections/%s", c.ApiUrl, name), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("connection not found")
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var connection models.Connection
	if err := json.NewDecoder(resp.Body).Decode(&connection); err != nil {
		return nil, err
	}

	return &connection, nil
}

func (c *Client) DeleteConnection(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/connections/%s", c.ApiUrl, name), nil)
	if err != nil {
		return fmt.Errorf("error creating delete request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making delete request: %v", err)
	}
	defer resp.Body.Close()

	// API returns 204 on successful deletion
	if resp.StatusCode == 204 {
		return nil
	}

	// Handle specific error cases
	switch resp.StatusCode {
	case 404:
		return fmt.Errorf("connection %s not found", name)
	case 403:
		return fmt.Errorf("not authorized to delete connection %s", name)
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

func (c *Client) CreateConnection(ctx context.Context, conn *models.Connection) error {
	jsonData, err := json.Marshal(conn)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/connections", c.ApiUrl), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) UpdateConnection(ctx context.Context, conn *models.Connection) error {
	jsonData, err := json.Marshal(conn)
	if err != nil {
		return fmt.Errorf("error marshaling connection for update: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/connections/%s", c.ApiUrl, conn.Name), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating update request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
