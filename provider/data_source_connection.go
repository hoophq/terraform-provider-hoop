package provider

import (
	"context"

	"github.com/hoophq/terraform-provider-hoop/client"
	"github.com/hoophq/terraform-provider-hoop/internal"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceConnection() *schema.Resource {
	connectionSchema := internal.CommonConnectionSchema(false)

	// Override name to make it required and not computed
	connectionSchema["name"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "The name of the connection to lookup",
	}

	return &schema.Resource{
		ReadContext: dataSourceConnectionRead,
		Schema:      connectionSchema,
	}
}

func dataSourceConnectionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "data_source", "connection")

	connectionName := d.Get("name").(string)
	ctx = tflog.SetField(ctx, "connection_name", connectionName)

	tflog.Info(ctx, "Reading connection data source")
	c := m.(*client.Client)

	connection, err := c.GetConnection(ctx, connectionName)
	if err != nil {
		tflog.Error(ctx, "Failed to get connection from API", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	d.SetId(connection.ID)

	if err := d.Set("id", connection.ID); err != nil {
		tflog.Error(ctx, "Error setting id", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	if err := d.Set("name", connection.Name); err != nil {
		tflog.Error(ctx, "Error setting name", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

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

	if err := d.Set("tags", connection.Tags); err != nil {
		tflog.Error(ctx, "Error setting tags", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully read connection data source")
	return diags
}
