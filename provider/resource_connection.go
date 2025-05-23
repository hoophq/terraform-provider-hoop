package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hoophq/terraform-provider-hoop/client"
	"github.com/hoophq/terraform-provider-hoop/internal"
	"github.com/hoophq/terraform-provider-hoop/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceConnection() *schema.Resource {
	baseSchema := internal.CommonConnectionSchema(true)

	// Add resource-specific schema elements
	baseSchema["datamasking"] = &schema.Schema{
		Type:     schema.TypeBool,
		Optional: true,
		Default:  false,
	}
	baseSchema["redact_types"] = &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	}
	baseSchema["review_groups"] = &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	}
	baseSchema["guardrails"] = &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
	}
	baseSchema["jira_template_id"] = &schema.Schema{
		Type:     schema.TypeString,
		Optional: true,
	}

	return &schema.Resource{
		CreateContext: resourceConnectionCreate,
		ReadContext:   resourceConnectionRead,
		UpdateContext: resourceConnectionUpdate,
		DeleteContext: resourceConnectionDelete,
		Schema:        baseSchema,
	}
}

func resourceConnectionCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "connection")
	connectionName := d.Get("name").(string)
	ctx = tflog.SetField(ctx, "connection_name", connectionName)

	tflog.Info(ctx, "Creating connection resource")
	c := m.(*client.Client)

	// Process and validate secrets
	tflog.Debug(ctx, "Validating connection credentials")
	secrets, err := validateAndParseSecrets(d)
	if err != nil {
		tflog.Error(ctx, "Invalid credentials", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("invalid credentials: %v", err))
	}

	// Get access mode with defaults if not provided
	tflog.Debug(ctx, "Processing access mode settings")

	// Verificar se access_mode foi explicitamente definido
	var accessMode map[string]interface{}
	accessModeRaw, accessModeSpecified := d.GetOk("access_mode")

	if accessModeSpecified && len(accessModeRaw.([]interface{})) > 0 {
		// Se especificado, use os valores definidos pelo usuário
		accessMode = accessModeRaw.([]interface{})[0].(map[string]interface{})
		tflog.Debug(ctx, "Using explicitly defined access_mode", map[string]interface{}{
			"runbook": accessMode["runbook"],
			"web":     accessMode["web"],
			"native":  accessMode["native"],
		})
	} else {
		// Caso contrário, use os valores padrão
		accessMode = map[string]interface{}{
			"runbook": true,
			"web":     true,
			"native":  true,
		}
		tflog.Debug(ctx, "Using default access_mode values", map[string]interface{}{
			"is_specified": accessModeSpecified,
		})
	}

	connType := d.Get("type").(string)
	subtype := d.Get("subtype").(string)

	connection := &models.Connection{
		Name:    connectionName,
		Type:    connType,
		Subtype: subtype,
		AgentID: d.Get("agent_id").(string),
		Secret:  secrets,

		// Access modes with defaults
		AccessModeRunbooks: convertBoolToEnabled(accessMode["runbook"].(bool)),
		AccessModeExec:     convertBoolToEnabled(accessMode["web"].(bool)),
		AccessModeConnect:  convertBoolToEnabled(accessMode["native"].(bool)),

		// Other fields with defaults
		AccessSchema:   convertBoolToEnabled(d.Get("access_schema").(bool)),
		RedactEnabled:  d.Get("datamasking").(bool),
		RedactTypes:    getListWithDefault(d, "redact_types"),
		Reviewers:      getListWithDefault(d, "review_groups"),
		GuardrailRules: getListWithDefault(d, "guardrails"),
		Tags:           getConnectionTagsFromResourceData(d),
	}

	// Add command for custom connections
	if connType == "custom" {
		if v, ok := d.GetOk("command"); ok {
			connection.Command = convertToStringArray(v.([]interface{}))
		}
	}

	if v, ok := d.GetOk("jira_template_id"); ok {
		connection.JiraIssueTemplateID = v.(string)
	}

	tflog.Debug(ctx, "Prepared connection model", map[string]interface{}{
		"type":     connection.Type,
		"subtype":  connection.Subtype,
		"agent_id": connection.AgentID,
	})

	conn, err := c.CreateConnection(ctx, connection)
	if err != nil {
		tflog.Error(ctx, "Failed to create connection", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully created connection, setting ID in state")
	d.SetId(conn.ID)

	return resourceConnectionRead(ctx, d, m)
}

func resourceConnectionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "connection")
	ctx = tflog.SetField(ctx, "connection_name", d.Id())

	tflog.Info(ctx, "Reading connection resource")
	c := m.(*client.Client)

	connection, err := c.GetConnection(ctx, d.Id())
	if err != nil {
		tflog.Error(ctx, "Failed to get connection from API", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "Setting connection attributes in state")

	// Set all attributes in state
	if err := d.Set("type", connection.Type); err != nil {
		tflog.Error(ctx, "Error setting type", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("subtype", connection.Subtype); err != nil {
		tflog.Error(ctx, "Error setting subtype", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("agent_id", connection.AgentID); err != nil {
		tflog.Error(ctx, "Error setting agent_id", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	// Set command for custom connections
	if connection.Type == "custom" {
		if err := setArrayFieldInState(d, "command", connection.Command); err != nil {
			tflog.Error(ctx, "Error setting command", map[string]interface{}{
				"error": err.Error(),
				"value": connection.Command,
			})
			return diag.FromErr(err)
		}
	}

	// Careful handling of secrets to avoid unnecessary diff
	if oldSecrets, ok := d.GetOk("secrets"); ok {
		// If we have existing secrets in state, we're careful about updating them
		oldSecretsMap := oldSecrets.(map[string]interface{})
		newSecretsMap := make(map[string]interface{})

		// Process API secrets into format for state
		for key, value := range connection.Secret {
			// Extract pure key without the envvar: prefix
			cleanKey := key
			if strings.HasPrefix(key, "envvar:") {
				cleanKey = strings.TrimPrefix(key, "envvar:")
				cleanKey = strings.ToLower(cleanKey)
			}

			// If we have an old value, prefer it unless the API value is clearly different
			if oldValue, exists := oldSecretsMap[cleanKey]; exists {
				// Try to decode both old and new to compare actual values
				oldValueStr := fmt.Sprint(oldValue)

				// If we can't meaningfully compare, just use the old value to avoid triggering a diff
				newSecretsMap[cleanKey] = oldValueStr
			} else {
				// For new keys, add them
				newSecretsMap[cleanKey] = value
			}
		}

		// Set processed secrets back to state
		if err := d.Set("secrets", newSecretsMap); err != nil {
			tflog.Error(ctx, "Error setting secrets", map[string]interface{}{
				"error": err.Error(),
			})
			return diag.FromErr(err)
		}
	} else {
		// For initial reads, just set the secrets as is
		if err := d.Set("secrets", connection.Secret); err != nil {
			tflog.Error(ctx, "Error setting secrets", map[string]interface{}{
				"error": err.Error(),
			})
			return diag.FromErr(err)
		}
	}

	// Handle access_mode carefully to avoid unnecessary diffs
	// Only set access_mode if it was explicitly specified in the config
	_, accessModeSpecified := d.GetOk("access_mode")

	if accessModeSpecified {
		accessMode := []interface{}{
			map[string]interface{}{
				"runbook": connection.AccessModeRunbooks == "enabled",
				"web":     connection.AccessModeExec == "enabled",
				"native":  connection.AccessModeConnect == "enabled",
			},
		}
		if err := d.Set("access_mode", accessMode); err != nil {
			tflog.Error(ctx, "Error setting access_mode", map[string]interface{}{
				"error": err.Error(),
			})
			return diag.FromErr(err)
		}
	} else {
		// If access_mode wasn't specified in the config, don't set it in the state
		// This prevents Terraform from showing a diff for default values
		tflog.Debug(ctx, "access_mode not explicitly specified in config, skipping state update")
	}

	// Set boolean conversions
	if err := d.Set("access_schema", connection.AccessSchema == "enabled"); err != nil {
		tflog.Error(ctx, "Error setting access_schema", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	// Set remaining fields
	if err := d.Set("datamasking", connection.RedactEnabled); err != nil {
		tflog.Error(ctx, "Error setting datamasking", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := setArrayFieldInState(d, "redact_types", connection.RedactTypes); err != nil {
		tflog.Error(ctx, "Error setting redact_types", map[string]interface{}{
			"error": err.Error(),
			"value": connection.RedactTypes,
		})
		return diag.FromErr(err)
	}
	if err := setArrayFieldInState(d, "review_groups", connection.Reviewers); err != nil {
		tflog.Error(ctx, "Error setting review_groups", map[string]interface{}{
			"error": err.Error(),
			"value": connection.Reviewers,
		})
		return diag.FromErr(err)
	}
	if err := setArrayFieldInState(d, "guardrails", connection.GuardrailRules); err != nil {
		tflog.Error(ctx, "Error setting guardrails", map[string]interface{}{
			"error": err.Error(),
			"value": connection.GuardrailRules,
		})
		return diag.FromErr(err)
	}
	if err := d.Set("jira_template_id", connection.JiraIssueTemplateID); err != nil {
		tflog.Error(ctx, "Error setting jira_template_id", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("tags", connection.Tags); err != nil {
		tflog.Error(ctx, "Error setting tags", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully read connection resource")
	return diags
}

func resourceConnectionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "connection")
	ctx = tflog.SetField(ctx, "connection_name", d.Id())

	tflog.Info(ctx, "Updating connection resource")
	c := m.(*client.Client)

	// Step 1: Get current connection
	tflog.Debug(ctx, "Retrieving current connection state")
	existingConnection, err := c.GetConnection(ctx, d.Id())
	if err != nil {
		tflog.Error(ctx, "Failed to get existing connection", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error reading connection for update: %v", err))
	}

	// Step 2: Build new connection based on the existing one
	tflog.Debug(ctx, "Building updated connection model")
	connection := &models.Connection{
		// Immutable fields
		Name:    existingConnection.Name,
		Type:    existingConnection.Type,
		Subtype: existingConnection.Subtype,

		// Default to existing values
		AgentID:             existingConnection.AgentID,
		Secret:              existingConnection.Secret,
		AccessModeRunbooks:  existingConnection.AccessModeRunbooks,
		AccessModeExec:      existingConnection.AccessModeExec,
		AccessModeConnect:   existingConnection.AccessModeConnect,
		AccessSchema:        existingConnection.AccessSchema,
		RedactEnabled:       existingConnection.RedactEnabled,
		RedactTypes:         existingConnection.RedactTypes,
		Reviewers:           existingConnection.Reviewers,
		GuardrailRules:      existingConnection.GuardrailRules,
		JiraIssueTemplateID: existingConnection.JiraIssueTemplateID,
		Tags:                existingConnection.Tags,
		Command:             existingConnection.Command,
	}

	// Step 3: Update fields that have changed
	var changedFields []string
	if d.HasChange("agent_id") {
		connection.AgentID = d.Get("agent_id").(string)
		changedFields = append(changedFields, "agent_id")
	}

	// Update command for custom connections
	if connection.Type == "custom" && d.HasChange("command") {
		if v, ok := d.GetOk("command"); ok {
			connection.Command = convertToStringArray(v.([]interface{}))
		} else {
			connection.Command = []string{}
		}
		changedFields = append(changedFields, "command")
	}

	// Carefully handle secrets changes - we only want to update if there's a real change
	if d.HasChange("secrets") {
		oldSecretsRaw, newSecretsRaw := d.GetChange("secrets")
		oldSecrets := oldSecretsRaw.(map[string]interface{})
		newSecrets := newSecretsRaw.(map[string]interface{})

		// Check if there's a real change in content
		realChange := false

		// Check if keys were added or removed
		if len(oldSecrets) != len(newSecrets) {
			realChange = true
		} else {
			// Check if values were changed
			for key, newValue := range newSecrets {
				if oldValue, exists := oldSecrets[key]; !exists || oldValue != newValue {
					realChange = true
					break
				}
			}
		}

		if realChange {
			tflog.Debug(ctx, "Validating updated credentials")
			parsedSecrets, err := validateAndParseSecrets(d)
			if err != nil {
				tflog.Error(ctx, "Invalid credentials in update", map[string]interface{}{
					"error": err.Error(),
				})
				return diag.FromErr(fmt.Errorf("invalid credentials: %v", err))
			}
			connection.Secret = parsedSecrets
			changedFields = append(changedFields, "secrets")
		} else {
			tflog.Debug(ctx, "No real change detected in secrets, keeping existing values")
		}
	}

	// Verifique cuidadosamente as alterações no access_mode
	if d.HasChange("access_mode") {
		// Verifica se o access_mode está explicitamente definido na configuração
		_, explicitlyDefined := d.GetOk("access_mode")

		if explicitlyDefined {
			// Se explicitamente definido, atualiza com os novos valores
			accessMode := getAccessMode(d)
			connection.AccessModeRunbooks = convertBoolToEnabled(accessMode["runbook"].(bool))
			connection.AccessModeExec = convertBoolToEnabled(accessMode["web"].(bool))
			connection.AccessModeConnect = convertBoolToEnabled(accessMode["native"].(bool))
			changedFields = append(changedFields, "access_mode")
		} else {
			// Se não estiver explicitamente definido, não considere como uma mudança real
			tflog.Debug(ctx, "access_mode não está explicitamente definido na configuração, ignorando mudança aparente")
		}
	}

	if d.HasChange("access_schema") {
		connection.AccessSchema = convertBoolToEnabled(d.Get("access_schema").(bool))
		changedFields = append(changedFields, "access_schema")
	}

	if d.HasChange("datamasking") {
		connection.RedactEnabled = d.Get("datamasking").(bool)
		changedFields = append(changedFields, "datamasking")
	}

	if d.HasChange("redact_types") {
		oldTypes, newTypes := d.GetChange("redact_types")
		oldTypesList := convertToStringArray(oldTypes.([]interface{}))
		newTypesList := convertToStringArray(newTypes.([]interface{}))

		tflog.Debug(ctx, "Redact types changed", map[string]interface{}{
			"connection_name": connection.Name,
			"old_types":       oldTypesList,
			"new_types":       newTypesList,
		})

		connection.RedactTypes = newTypesList
		changedFields = append(changedFields, "redact_types")
	}

	if d.HasChange("review_groups") {
		oldGroups, newGroups := d.GetChange("review_groups")
		oldGroupsList := convertToStringArray(oldGroups.([]interface{}))
		newGroupsList := convertToStringArray(newGroups.([]interface{}))

		// Log detalhado para troubleshooting
		tflog.Debug(ctx, "Review groups changed", map[string]interface{}{
			"connection_name": connection.Name,
			"old_groups":      oldGroupsList,
			"new_groups":      newGroupsList,
		})

		// Garante que array vazio seja transmitido corretamente para a API
		connection.Reviewers = newGroupsList
		changedFields = append(changedFields, "review_groups")
	}

	if d.HasChange("guardrails") {
		oldRules, newRules := d.GetChange("guardrails")
		oldRulesList := convertToStringArray(oldRules.([]interface{}))
		newRulesList := convertToStringArray(newRules.([]interface{}))

		tflog.Debug(ctx, "Guardrail rules changed", map[string]interface{}{
			"connection_name": connection.Name,
			"old_rules":       oldRulesList,
			"new_rules":       newRulesList,
		})

		connection.GuardrailRules = newRulesList
		changedFields = append(changedFields, "guardrails")
	}

	if d.HasChange("jira_template_id") {
		connection.JiraIssueTemplateID = d.Get("jira_template_id").(string)
		changedFields = append(changedFields, "jira_template_id")
	}

	if d.HasChange("tags") {
		connection.Tags = getConnectionTagsFromResourceData(d)
		changedFields = append(changedFields, "tags")
	}

	tflog.Info(ctx, "Updating connection with changed fields", map[string]interface{}{
		"changed_fields": changedFields,
	})

	// Step 4: Update the connection
	err = c.UpdateConnection(ctx, connection)
	if err != nil {
		tflog.Error(ctx, "Failed to update connection", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error updating connection: %v", err))
	}

	tflog.Info(ctx, "Connection updated successfully")

	// Read the connection again to ensure state consistency
	return resourceConnectionRead(ctx, d, m)
}

func resourceConnectionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "connection")

	// Get connection name from ID
	connectionName := d.Get("name").(string)
	ctx = tflog.SetField(ctx, "connection_name", connectionName)

	tflog.Info(ctx, "Deleting connection resource")
	c := m.(*client.Client)

	// Delete connection
	if err := c.DeleteConnection(ctx, connectionName); err != nil {
		// Check if the resource is already gone
		if strings.Contains(err.Error(), "not found") {
			tflog.Debug(ctx, "Connection not found - considering delete successful", map[string]interface{}{
				"name": connectionName,
			})
			return diags
		}

		tflog.Error(ctx, "Failed to delete connection", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error deleting connection %s: %v", connectionName, err))
	}

	tflog.Info(ctx, "Connection deleted successfully")

	// Clear the ID from state
	d.SetId("")

	return diags
}

func getAccessMode(d *schema.ResourceData) map[string]interface{} {
	// Verificar se access_mode foi explicitamente definido na configuração
	v, explicitlySet := d.GetOk("access_mode")

	// Se foi explicitamente definido e tem elementos, use esses valores
	if explicitlySet && len(v.([]interface{})) > 0 {
		return v.([]interface{})[0].(map[string]interface{})
	}

	// Se não foi explicitamente definido, use os valores padrão
	return map[string]interface{}{
		"runbook": true,
		"web":     true,
		"native":  true,
	}
}

func convertToStringArray(arr []interface{}) []string {
	result := make([]string, len(arr))
	for i, v := range arr {
		result[i] = fmt.Sprint(v)
	}
	return result
}

// Funções auxiliares existentes
func convertBoolToEnabled(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func validateAndParseSecrets(d *schema.ResourceData) (map[string]string, error) {
	connType := d.Get("type").(string)
	subtype := d.Get("subtype").(string)
	secretsRaw := d.Get("secrets").(map[string]interface{})

	// Validate credentials for database connections
	if connType == "database" {
		if err := internal.ValidateCredentials(secretsRaw, subtype); err != nil {
			return nil, err
		}
	}
	// For custom connections, no validation needed

	// Convert to the format expected by the API
	result := make(map[string]string)

	// Check if we're updating an existing resource
	isUpdate := d.Id() != ""

	// If we're updating and the API might return encoded values, we need to be careful
	// about re-encoding already encoded values. For new resources, we always encode.
	for key, value := range secretsRaw {
		valueStr := fmt.Sprint(value)

		var finalKey string

		// Check if the key already has a prefix
		if strings.HasPrefix(strings.ToLower(key), "envvar:") {
			// Keep the prefix, but uppercase the key
			prefix := "envvar:"
			cleanKey := strings.TrimPrefix(strings.ToLower(key), prefix)
			finalKey = prefix + strings.ToUpper(cleanKey)
		} else if strings.HasPrefix(strings.ToLower(key), "filesystem:") {
			// Keep the prefix, but uppercase the key
			prefix := "filesystem:"
			cleanKey := strings.TrimPrefix(strings.ToLower(key), prefix)
			finalKey = prefix + strings.ToUpper(cleanKey)
		} else {
			// No prefix, add envvar: prefix and uppercase the key
			finalKey = fmt.Sprintf("envvar:%s", strings.ToUpper(key))
		}

		// Check if this is already a valid base64 string (for updates)
		if isUpdate {
			// Try to decode the string as base64
			if decoded, err := base64.StdEncoding.DecodeString(valueStr); err == nil && len(decoded) > 0 {
				// If it decodes successfully, it's already encoded - use as is
				result[finalKey] = valueStr
				continue
			}
		}

		// Otherwise encode it
		result[finalKey] = base64.StdEncoding.EncodeToString([]byte(valueStr))
	}

	return result, nil
}

// Função mais robusta para tratar listas, especialmente para review_groups
func getListWithDefault(d *schema.ResourceData, key string) []string {
	// Verifica se a chave está presente no ResourceData
	raw, exists := d.GetOk(key)

	// Adicione log detalhado para troubleshooting
	tflog.Debug(context.Background(), fmt.Sprintf("getListWithDefault: key=%s, exists=%t", key, exists))

	if exists {
		// Se existe, converte para string array
		result := convertToStringArray(raw.([]interface{}))
		tflog.Debug(context.Background(), fmt.Sprintf("getListWithDefault: converted result=%v", result))
		return result
	}

	// Importante: Se a chave não existe, mas está definida como um array vazio,
	// precisamos retornar um array vazio, não null
	// Usamos hasChange para verificar se houve uma mudança explícita para um array vazio
	oldRaw, newRaw := d.GetChange(key)
	if len(oldRaw.([]interface{})) > 0 && len(newRaw.([]interface{})) == 0 {
		tflog.Debug(context.Background(), fmt.Sprintf("getListWithDefault: detected explicit empty array for key=%s", key))
		return []string{} // Retorna array vazio explicitamente
	}

	// Por padrão, retorna um array vazio
	return []string{}
}

func getConnectionTagsFromResourceData(d *schema.ResourceData) map[string]string {
	if v, ok := d.GetOk("tags"); ok {
		result := make(map[string]string)
		for key, value := range v.(map[string]interface{}) {
			result[key] = value.(string)
		}
		return result
	}
	return make(map[string]string)
}

// Melhor função para lidar com arrays explicitamente vazios vs. null
func setArrayFieldInState(d *schema.ResourceData, key string, value []string) error {
	// Se o campo não está definido no config mas está recebendo um valor vazio da API,
	// não atualizamos o estado, evitando mostrar mudanças falsas
	if !d.HasChange(key) && len(value) == 0 {
		// Verifica se o valor atual também é vazio
		current, ok := d.GetOk(key)
		if !ok || (ok && len(current.([]interface{})) == 0) {
			// Ambos são vazios, não precisa atualizar
			return nil
		}
	}

	// Registra as alterações para facilitar o debugging
	tflog.Debug(context.Background(), fmt.Sprintf("Setting %s in state: %v", key, value))
	return d.Set(key, value)
}
