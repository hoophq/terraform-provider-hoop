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
	var accessMode map[string]interface{}
	if v, ok := d.GetOk("access_mode"); ok && len(v.([]interface{})) > 0 {
		accessMode = v.([]interface{})[0].(map[string]interface{})
	} else {
		accessMode = map[string]interface{}{
			"runbook": true,
			"web":     true,
			"native":  true,
		}
	}

	connection := &models.Connection{
		Name:    connectionName,
		Type:    d.Get("type").(string),
		Subtype: d.Get("subtype").(string),
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
		ConnectionTags: getConnectionTagsFromResourceData(d),
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
	if err := d.Set("secrets", connection.Secret); err != nil {
		tflog.Error(ctx, "Error setting secrets", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

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
	if err := d.Set("redact_types", connection.RedactTypes); err != nil {
		tflog.Error(ctx, "Error setting redact_types", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("review_groups", connection.Reviewers); err != nil {
		tflog.Error(ctx, "Error setting review_groups", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("guardrails", connection.GuardrailRules); err != nil {
		tflog.Error(ctx, "Error setting guardrails", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("jira_template_id", connection.JiraIssueTemplateID); err != nil {
		tflog.Error(ctx, "Error setting jira_template_id", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}
	if err := d.Set("connection_tags", connection.ConnectionTags); err != nil {
		tflog.Error(ctx, "Error setting connection_tags", map[string]interface{}{
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
		ConnectionTags:      existingConnection.ConnectionTags,
	}

	// Step 3: Update fields that have changed
	var changedFields []string
	if d.HasChange("agent_id") {
		connection.AgentID = d.Get("agent_id").(string)
		changedFields = append(changedFields, "agent_id")
	}

	if d.HasChange("secrets") {
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
	}

	if d.HasChange("access_mode") {
		accessMode := getAccessMode(d)
		connection.AccessModeRunbooks = convertBoolToEnabled(accessMode["runbook"].(bool))
		connection.AccessModeExec = convertBoolToEnabled(accessMode["web"].(bool))
		connection.AccessModeConnect = convertBoolToEnabled(accessMode["native"].(bool))
		changedFields = append(changedFields, "access_mode")
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
		connection.RedactTypes = getListWithDefault(d, "redact_types")
		changedFields = append(changedFields, "redact_types")
	}

	if d.HasChange("review_groups") {
		connection.Reviewers = getListWithDefault(d, "review_groups")
		changedFields = append(changedFields, "review_groups")
	}

	if d.HasChange("guardrails") {
		connection.GuardrailRules = getListWithDefault(d, "guardrails")
		changedFields = append(changedFields, "guardrails")
	}

	if d.HasChange("jira_template_id") {
		connection.JiraIssueTemplateID = d.Get("jira_template_id").(string)
		changedFields = append(changedFields, "jira_template_id")
	}

	if d.HasChange("connection_tags") {
		connection.ConnectionTags = getConnectionTagsFromResourceData(d)
		changedFields = append(changedFields, "connection_tags")
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
	connectionName := d.Id()
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
	if v, ok := d.GetOk("access_mode"); ok {
		if len(v.([]interface{})) > 0 {
			return v.([]interface{})[0].(map[string]interface{})
		}
	}
	// Return default values if not set
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
	subtype := d.Get("subtype").(string)
	secretsRaw := d.Get("secrets").(map[string]interface{})

	// Validate credentials if it's a database type
	if d.Get("type").(string) == "database" {
		if err := internal.ValidateCredentials(subtype, secretsRaw); err != nil {
			return nil, err
		}
	}

	// Convert to the format expected by the API
	result := make(map[string]string)
	for key, value := range secretsRaw {
		envvarKey := fmt.Sprintf("envvar:%s", strings.ToUpper(key))
		result[envvarKey] = base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(value)))
	}

	return result, nil
}

func getListWithDefault(d *schema.ResourceData, key string) []string {
	if v, ok := d.GetOk(key); ok {
		return convertToStringArray(v.([]interface{}))
	}
	return []string{}
}

func getConnectionTagsFromResourceData(d *schema.ResourceData) map[string]string {
	if v, ok := d.GetOk("connection_tags"); ok {
		result := make(map[string]string)
		for key, value := range v.(map[string]interface{}) {
			result[key] = value.(string)
		}
		return result
	}
	return make(map[string]string)
}
