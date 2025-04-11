package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
		ApiKey: apiKey,
		ApiUrl: apiUrl,
		HttpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
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

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseBody := string(bodyBytes)

	tflog.Debug(ctx, "Connection response body", map[string]interface{}{
		"body": responseBody,
	})

	var connection models.Connection
	if err := json.Unmarshal(bodyBytes, &connection); err != nil {
		tflog.Error(ctx, "Failed to decode API response", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	tflog.Info(ctx, "Connection retrieved successfully", map[string]interface{}{
		"name": name,
		"type": connection.Type,
		"id":   connection.ID,
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
		"status_code":     resp.StatusCode,
		"status":          resp.Status,
		"request_url":     req.URL.String(),
		"response_url":    resp.Request.URL.String(),
		"location_header": resp.Header.Get("Location"),
		"content_type":    resp.Header.Get("Content-Type"),
		"content_length":  resp.ContentLength,
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

// GetPlugin obtém um plugin pelo nome
func (c *Client) GetPlugin(ctx context.Context, name string) (*models.Plugin, error) {
	tflog.Debug(ctx, "Getting plugin", map[string]interface{}{
		"name": name,
	})

	url := fmt.Sprintf("%s/plugins/%s", c.ApiUrl, name)
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

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseBody := string(bodyBytes)

	tflog.Debug(ctx, "Resposta da API GetPlugin", map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        responseBody,
	})

	if resp.StatusCode == 404 {
		tflog.Info(ctx, "Plugin not found", map[string]interface{}{
			"name": name,
		})
		return nil, fmt.Errorf("plugin not found")
	}

	if resp.StatusCode != 200 {
		tflog.Error(ctx, "API returned error", map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        responseBody,
		})
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	var plugin models.Plugin
	if err := json.Unmarshal(bodyBytes, &plugin); err != nil {
		tflog.Error(ctx, "Failed to decode API response", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	tflog.Info(ctx, "Plugin retrieved successfully", map[string]interface{}{
		"name": name,
	})

	return &plugin, nil
}

// CreatePlugin cria um novo plugin
func (c *Client) CreatePlugin(ctx context.Context, plugin *models.Plugin) error {
	tflog.Debug(ctx, "Creating plugin", map[string]interface{}{
		"name": plugin.Name,
	})

	jsonData, err := json.Marshal(plugin)
	if err != nil {
		tflog.Error(ctx, "Failed to marshal plugin data", map[string]interface{}{
			"error": err.Error(),
			"name":  plugin.Name,
		})
		return fmt.Errorf("failed to marshal plugin data: %v", err)
	}

	url := fmt.Sprintf("%s/plugins", c.ApiUrl)
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

	tflog.Info(ctx, "Plugin created successfully", map[string]interface{}{
		"name": plugin.Name,
	})

	return nil
}

// UpdatePlugin atualiza um plugin existente
func (c *Client) UpdatePlugin(ctx context.Context, plugin *models.Plugin) error {
	tflog.Debug(ctx, "Updating plugin", map[string]interface{}{
		"name": plugin.Name,
	})

	jsonData, err := json.Marshal(plugin)
	if err != nil {
		tflog.Error(ctx, "Failed to marshal plugin data for update", map[string]interface{}{
			"error": err.Error(),
			"name":  plugin.Name,
		})
		return fmt.Errorf("error marshaling plugin for update: %v", err)
	}

	url := fmt.Sprintf("%s/plugins/%s", c.ApiUrl, plugin.Name)
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

	tflog.Info(ctx, "Plugin updated successfully", map[string]interface{}{
		"name": plugin.Name,
	})

	return nil
}

// GetAccessGroup obtém um grupo de acesso pelo nome
func (c *Client) GetAccessGroup(ctx context.Context, group string) (*models.AccessGroup, error) {
	// Verificar grupo
	if group == "" {
		return nil, fmt.Errorf("group name cannot be empty")
	}

	// Obter o plugin de access_control
	plugin, err := c.GetPlugin(ctx, "access_control")
	if err != nil {
		return nil, fmt.Errorf("failed to get access_control plugin: %v", err)
	}

	// Filtra conexões para este grupo
	var connections []string
	for _, conn := range plugin.Connections {
		// A config do plugin access_control contém os grupos que podem acessar a conexão
		if contains(conn.Config, group) {
			connections = append(connections, conn.Name)
		}
	}

	accessGroup := &models.AccessGroup{
		Name:        group,
		Connections: connections,
	}

	return accessGroup, nil
}

// CreateAccessGroup cria um grupo de acesso adicionando as conexões ao plugin access_control
func (c *Client) CreateAccessGroup(ctx context.Context, accessGroup *models.AccessGroup) error {
	// Validar o grupo
	if accessGroup.Name == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	// Filtrar conexões vazias
	var validConnections []string
	for _, conn := range accessGroup.Connections {
		if conn != "" {
			validConnections = append(validConnections, conn)
		}
	}
	accessGroup.Connections = validConnections

	// Se não houver conexões válidas, apenas retorne sem erro
	if len(accessGroup.Connections) == 0 {
		tflog.Warn(ctx, "No valid connections provided for access group, nothing to create")
		return nil
	}

	// Tentar obter o plugin access_control existente
	var plugin models.Plugin
	existingPlugin, err := c.GetPlugin(ctx, "access_control")
	if err == nil {
		// Plugin já existe, usar seus dados
		plugin = *existingPlugin
	} else {
		// Plugin não existe ou erro ao obter, criar novo
		plugin = models.Plugin{
			ID:          generateUUID(),
			Name:        "access_control",
			Connections: []models.PluginConnection{},
			Config:      nil,
			Source:      nil,
			Priority:    0,
			Installed:   true,
		}
	}

	// Preparar as conexões para o plugin
	var pluginConnections []models.PluginConnection

	// Obter a lista de conexões existentes para preservar outras conexões não modificadas
	existingConnectionsMap := make(map[string]models.PluginConnection)
	for _, conn := range plugin.Connections {
		existingConnectionsMap[conn.ID] = conn
	}

	// Adicionar as conexões novas/atualizadas
	for _, connName := range accessGroup.Connections {
		conn, err := c.GetConnection(ctx, connName)
		if err != nil {
			return fmt.Errorf("connection %q does not exist: %v", connName, err)
		}

		// Tentar extrair o ID da conexão
		connID := conn.ID
		if connID == "" {
			return fmt.Errorf("connection %q has no ID", connName)
		}

		// Verificar se esta conexão já existe no plugin
		if existingConn, exists := existingConnectionsMap[connID]; exists {
			// Adicionar o grupo aos grupos existentes, se não estiver lá
			groups := existingConn.Config
			if !contains(groups, accessGroup.Name) {
				groups = append(groups, accessGroup.Name)
			}
			existingConn.Config = groups
			pluginConnections = append(pluginConnections, existingConn)
			delete(existingConnectionsMap, connID)
		} else {
			// Criar nova conexão no plugin
			pluginConn := models.PluginConnection{
				ID:     connID,
				Name:   connName,
				Config: []string{accessGroup.Name},
			}
			pluginConnections = append(pluginConnections, pluginConn)
		}
	}

	// Adicionar de volta as conexões existentes não modificadas
	for _, conn := range existingConnectionsMap {
		pluginConnections = append(pluginConnections, conn)
	}

	plugin.Connections = pluginConnections

	// Fazer o PUT para atualizar/criar o plugin
	jsonData, err := json.Marshal(plugin)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin data: %v", err)
	}

	// Log do JSON para debug
	tflog.Debug(ctx, "Plugin JSON para PUT (CreateAccessGroup)", map[string]interface{}{
		"json_payload": string(jsonData),
	})

	url := fmt.Sprintf("%s/plugins/access_control", c.ApiUrl)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute PUT request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseBody := string(bodyBytes)

	tflog.Debug(ctx, "Resposta da API (CreateAccessGroup)", map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        responseBody,
	})

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	return nil
}

// UpdateAccessGroup atualiza um grupo de acesso
func (c *Client) UpdateAccessGroup(ctx context.Context, accessGroup *models.AccessGroup) error {
	// Validar o grupo
	if accessGroup.Name == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	// Filtrar conexões vazias
	var validConnections []string
	for _, conn := range accessGroup.Connections {
		if conn != "" {
			validConnections = append(validConnections, conn)
		}
	}
	accessGroup.Connections = validConnections

	// Obter o plugin access_control existente
	existingPlugin, err := c.GetPlugin(ctx, "access_control")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// Se o plugin não existir, criar um novo
			return c.CreateAccessGroup(ctx, accessGroup)
		}
		return fmt.Errorf("failed to get access_control plugin: %v", err)
	}

	// Criar uma cópia do plugin para modificação
	plugin := *existingPlugin

	// Preparar as conexões para o plugin
	var pluginConnections []models.PluginConnection

	// Mapear as conexões existentes
	existingConnectionsMap := make(map[string]models.PluginConnection)
	for _, conn := range plugin.Connections {
		existingConnectionsMap[conn.ID] = conn
	}

	// Construir um mapa das conexões que o grupo deve acessar (pelo ID)
	connectionIDsToAccess := make(map[string]string) // id -> name
	for _, connName := range accessGroup.Connections {
		conn, err := c.GetConnection(ctx, connName)
		if err != nil {
			return fmt.Errorf("connection %q does not exist: %v", connName, err)
		}

		connID := conn.ID
		if connID == "" {
			return fmt.Errorf("connection %q has no ID", connName)
		}

		connectionIDsToAccess[connID] = connName
	}

	// Atualizar as conexões existentes
	for id, conn := range existingConnectionsMap {
		if _, shouldAccess := connectionIDsToAccess[id]; shouldAccess {
			// Esta conexão deve ter acesso ao grupo
			if !contains(conn.Config, accessGroup.Name) {
				conn.Config = append(conn.Config, accessGroup.Name)
			}
			delete(connectionIDsToAccess, id) // Remover do mapa para processarmos os novos depois
		} else {
			// Verificar se o grupo deve ser removido desta conexão
			conn.Config = removeString(conn.Config, accessGroup.Name)
		}

		// Só adicionar se ainda tiver alguma config
		if len(conn.Config) > 0 {
			pluginConnections = append(pluginConnections, conn)
		}

	}

	// Adicionar novas conexões que devem ter acesso
	for id, name := range connectionIDsToAccess {
		newConn := models.PluginConnection{
			ID:     id,
			Name:   name,
			Config: []string{accessGroup.Name},
		}
		pluginConnections = append(pluginConnections, newConn)
	}

	plugin.Connections = pluginConnections

	// Fazer o PUT para atualizar o plugin
	jsonData, err := json.Marshal(plugin)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin data: %v", err)
	}

	// Log do JSON para debug
	tflog.Debug(ctx, "Plugin JSON para PUT (UpdateAccessGroup)", map[string]interface{}{
		"json_payload": string(jsonData),
	})

	url := fmt.Sprintf("%s/plugins/access_control", c.ApiUrl)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute PUT request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseBody := string(bodyBytes)

	tflog.Debug(ctx, "Resposta da API (UpdateAccessGroup)", map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        responseBody,
	})

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	return nil
}

// DeleteAccessGroup remove um grupo de acesso
func (c *Client) DeleteAccessGroup(ctx context.Context, group string) error {
	// Validar grupo
	if group == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	// Obter o plugin access_control existente
	existingPlugin, err := c.GetPlugin(ctx, "access_control")
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// Se o plugin não existir, não há nada para excluir
			return nil
		}
		return fmt.Errorf("failed to get access_control plugin: %v", err)
	}

	// Criar uma cópia do plugin para modificação
	plugin := *existingPlugin

	// Verificar se há alguma conexão que usa este grupo
	hasChanges := false
	var updatedConnections []models.PluginConnection

	for _, conn := range plugin.Connections {
		if contains(conn.Config, group) {
			// Remover o grupo das configurações
			conn.Config = removeString(conn.Config, group)
			hasChanges = true

			// Só adicionar conexões que ainda têm configurações
			if len(conn.Config) > 0 {
				updatedConnections = append(updatedConnections, conn)
			}
		} else {
			// Esta conexão não tem o grupo, mantê-la intacta
			updatedConnections = append(updatedConnections, conn)
		}
	}

	// Se não houve mudanças, não precisa atualizar
	if !hasChanges {
		return nil
	}

	// Atualizar o plugin com as conexões atualizadas
	plugin.Connections = updatedConnections

	// Fazer o PUT para atualizar o plugin
	jsonData, err := json.Marshal(plugin)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin data: %v", err)
	}

	// Log do JSON para debug
	tflog.Debug(ctx, "Plugin JSON para PUT (DeleteAccessGroup)", map[string]interface{}{
		"json_payload": string(jsonData),
	})

	url := fmt.Sprintf("%s/plugins/access_control", c.ApiUrl)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}

	req.Header.Set("Api-Key", c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute PUT request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	responseBody := string(bodyBytes)

	tflog.Debug(ctx, "Resposta da API (DeleteAccessGroup)", map[string]interface{}{
		"status_code": resp.StatusCode,
		"body":        responseBody,
	})

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, responseBody)
	}

	return nil
}

// Função auxiliar para verificar se uma string está em um slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Função auxiliar para remover uma string de um slice
func removeString(slice []string, item string) []string {
	var result []string
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// Função auxiliar para gerar UUID
func generateUUID() string {
	// Um UUID simples para uso no provider
	return "28ba4b85-b4a8-4c55-8f5e-34edc9aa62c8"
}
