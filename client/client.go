package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	tflog.Debug(ctx, "Getting connection", map[string]interface{}{
		"name": name,
	})

	url := fmt.Sprintf("%s/connections/%s", c.ApiUrl, name)
	tflog.Trace(ctx, "Creating GET request", map[string]interface{}{
		"url": url,
	})

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create GET request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return nil, fmt.Errorf("failed to create GET request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	tflog.Trace(ctx, "Sending GET request", map[string]interface{}{
		"url": url,
	})
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		tflog.Error(ctx, "Failed to execute GET request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return nil, fmt.Errorf("failed to execute GET request: %v", err)
	}
	defer resp.Body.Close()

	tflog.Debug(ctx, "Received API response", map[string]interface{}{
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode == 404 {
		tflog.Info(ctx, "Connection not found", map[string]interface{}{
			"name": name,
		})
		return nil, fmt.Errorf("connection not found")
	}

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)
		tflog.Error(ctx, "API returned error", map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        responseBody,
		})
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	var connection models.Connection
	if err := json.NewDecoder(resp.Body).Decode(&connection); err != nil {
		tflog.Error(ctx, "Failed to decode API response", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	tflog.Info(ctx, "Connection retrieved successfully", map[string]interface{}{
		"name": name,
		"type": connection.Type,
	})

	return &connection, nil
}

func (c *Client) DeleteConnection(ctx context.Context, name string) error {
	tflog.Debug(ctx, "Deleting connection", map[string]interface{}{
		"name": name,
	})

	url := fmt.Sprintf("%s/connections/%s", c.ApiUrl, name)
	tflog.Trace(ctx, "Creating DELETE request", map[string]interface{}{
		"url": url,
	})

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		tflog.Error(ctx, "Failed to create DELETE request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return fmt.Errorf("error creating delete request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	tflog.Trace(ctx, "Sending DELETE request", map[string]interface{}{
		"url": url,
	})
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		tflog.Error(ctx, "Failed to execute DELETE request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return fmt.Errorf("error making delete request: %v", err)
	}
	defer resp.Body.Close()

	tflog.Debug(ctx, "Received API response for delete", map[string]interface{}{
		"status_code": resp.StatusCode,
	})

	// API returns 204 on successful deletion
	if resp.StatusCode == 204 {
		tflog.Info(ctx, "Connection deleted successfully", map[string]interface{}{
			"name": name,
		})
		return nil
	}

	// Handle specific error cases
	switch resp.StatusCode {
	case 404:
		tflog.Info(ctx, "Connection not found for deletion", map[string]interface{}{
			"name": name,
		})
		return fmt.Errorf("connection %s not found", name)
	case 403:
		tflog.Error(ctx, "Not authorized to delete connection", map[string]interface{}{
			"name": name,
		})
		return fmt.Errorf("not authorized to delete connection %s", name)
	default:
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)
		tflog.Error(ctx, "API returned error for delete", map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        responseBody,
		})
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}
}

func (c *Client) CreateConnection(ctx context.Context, conn *models.Connection) error {
	tflog.Debug(ctx, "Creating connection", map[string]interface{}{
		"name":     conn.Name,
		"type":     conn.Type,
		"subtype":  conn.Subtype,
		"agent_id": conn.AgentID,
	})

	jsonData, err := json.Marshal(conn)
	if err != nil {
		tflog.Error(ctx, "Failed to marshal connection data", map[string]interface{}{
			"error": err.Error(),
			"name":  conn.Name,
		})
		return fmt.Errorf("failed to marshal connection data: %v", err)
	}

	url := fmt.Sprintf("%s/connections", c.ApiUrl)
	tflog.Trace(ctx, "Creating POST request", map[string]interface{}{
		"url": url,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		tflog.Error(ctx, "Failed to create POST request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return fmt.Errorf("failed to create POST request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	tflog.Trace(ctx, "Sending POST request", map[string]interface{}{
		"url": url,
	})
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		tflog.Error(ctx, "Failed to execute POST request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return fmt.Errorf("failed to execute POST request: %v", err)
	}
	defer resp.Body.Close()

	tflog.Debug(ctx, "Received API response for create", map[string]interface{}{
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)
		tflog.Error(ctx, "API returned error for create", map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        responseBody,
		})
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	tflog.Info(ctx, "Connection created successfully", map[string]interface{}{
		"name": conn.Name,
	})

	return nil
}

func (c *Client) UpdateConnection(ctx context.Context, conn *models.Connection) error {
	tflog.Debug(ctx, "Updating connection", map[string]interface{}{
		"name":     conn.Name,
		"type":     conn.Type,
		"subtype":  conn.Subtype,
		"agent_id": conn.AgentID,
	})

	jsonData, err := json.Marshal(conn)
	if err != nil {
		tflog.Error(ctx, "Failed to marshal connection data for update", map[string]interface{}{
			"error": err.Error(),
			"name":  conn.Name,
		})
		return fmt.Errorf("error marshaling connection for update: %v", err)
	}

	url := fmt.Sprintf("%s/connections/%s", c.ApiUrl, conn.Name)
	tflog.Trace(ctx, "Creating PUT request", map[string]interface{}{
		"url": url,
	})

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		tflog.Error(ctx, "Failed to create PUT request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return fmt.Errorf("error creating update request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	tflog.Trace(ctx, "Sending PUT request", map[string]interface{}{
		"url": url,
	})
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		tflog.Error(ctx, "Failed to execute PUT request", map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		})
		return fmt.Errorf("error making update request: %v", err)
	}
	defer resp.Body.Close()

	tflog.Debug(ctx, "Received API response for update", map[string]interface{}{
		"status_code": resp.StatusCode,
	})

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)
		tflog.Error(ctx, "API returned error for update", map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        responseBody,
		})
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	tflog.Info(ctx, "Connection updated successfully", map[string]interface{}{
		"name": conn.Name,
	})

	return nil
}
