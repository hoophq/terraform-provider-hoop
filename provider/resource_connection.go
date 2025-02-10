package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"terraform-provider-hoop/client"
	"terraform-provider-hoop/internal"
	"terraform-provider-hoop/models"

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
	c := m.(*client.Client)

	// Process and validate secrets
	secrets, err := validateAndParseSecrets(d)
	if err != nil {
		return diag.FromErr(fmt.Errorf("invalid credentials: %v", err))
	}

	// Get access mode with defaults if not provided
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
		Name:    d.Get("name").(string),
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
		Tags:           getListWithDefault(d, "tags"),
	}

	if v, ok := d.GetOk("jira_template_id"); ok {
		connection.JiraIssueTemplateID = v.(string)
	}

	err = c.CreateConnection(ctx, connection)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(connection.Name)

	return resourceConnectionRead(ctx, d, m)
}

func resourceConnectionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	c := m.(*client.Client)

	connection, err := c.GetConnection(ctx, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("type", connection.Type); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("subtype", connection.Subtype); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("agent_id", connection.AgentID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("secrets", connection.Secret); err != nil {
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
		return diag.FromErr(err)
	}

	if err := d.Set("access_schema", connection.AccessSchema == "enabled"); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("datamasking", connection.RedactEnabled); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("redact_types", connection.RedactTypes); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("review_groups", connection.Reviewers); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("guardrails", connection.GuardrailRules); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("jira_template_id", connection.JiraIssueTemplateID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("tags", connection.Tags); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func resourceConnectionUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*client.Client)

	// Step 1: Get current connection
	existingConnection, err := c.GetConnection(ctx, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading connection for update: %v", err))
	}

	// Step 2: Build new connection based on the existing one
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
	}

	// Step 3: Update fields that have changed
	if d.HasChange("agent_id") {
		connection.AgentID = d.Get("agent_id").(string)
	}

	if d.HasChange("secrets") {
		parsedSecrets, err := validateAndParseSecrets(d)
		if err != nil {
			return diag.FromErr(fmt.Errorf("invalid credentials: %v", err))
		}
		connection.Secret = parsedSecrets
	}

	if d.HasChange("access_mode") {
		accessMode := getAccessMode(d)
		connection.AccessModeRunbooks = convertBoolToEnabled(accessMode["runbook"].(bool))
		connection.AccessModeExec = convertBoolToEnabled(accessMode["web"].(bool))
		connection.AccessModeConnect = convertBoolToEnabled(accessMode["native"].(bool))
	}

	if d.HasChange("access_schema") {
		connection.AccessSchema = convertBoolToEnabled(d.Get("access_schema").(bool))
	}

	if d.HasChange("datamasking") {
		connection.RedactEnabled = d.Get("datamasking").(bool)
	}

	if d.HasChange("redact_types") {
		connection.RedactTypes = getListWithDefault(d, "redact_types")
	}

	if d.HasChange("review_groups") {
		connection.Reviewers = getListWithDefault(d, "review_groups")
	}

	if d.HasChange("guardrails") {
		connection.GuardrailRules = getListWithDefault(d, "guardrails")
	}

	if d.HasChange("jira_template_id") {
		connection.JiraIssueTemplateID = d.Get("jira_template_id").(string)
	}

	if d.HasChange("tags") {
		connection.Tags = getListWithDefault(d, "tags")
	}

	// Step 4: Update the connection
	err = c.UpdateConnection(ctx, connection)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error updating connection: %v", err))
	}

	// Read the connection again to ensure state consistency
	return resourceConnectionRead(ctx, d, m)
}

func resourceConnectionDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	c := m.(*client.Client)

	// Get connection name from ID
	connectionName := d.Id()

	// Log deletion attempt
	log.Printf("[INFO] Attempting to delete connection %s", connectionName)

	// Delete connection
	if err := c.DeleteConnection(ctx, connectionName); err != nil {
		// Check if the resource is already gone
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[DEBUG] Connection %s was not found - considering delete successful", connectionName)
			return diags
		}

		return diag.FromErr(fmt.Errorf("error deleting connection %s: %v", connectionName, err))
	}

	// Log successful deletion
	log.Printf("[INFO] Successfully deleted connection %s", connectionName)

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
