package provider

import (
	"context"
	"fmt"

	"github.com/hoophq/terraform-provider-hoop/client"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRunbooksPath() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRunbooksPathRead,
		Schema: map[string]*schema.Schema{
			"connection_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the connection to look up runbooks path for",
			},
			"connection_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the connection",
			},
			"path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The configured path for runbooks",
			},
			"last_updated": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp of when the runbooks path was last updated",
			},
		},
	}
}

func dataSourceRunbooksPathRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "data_source", "runbooks_path")

	connectionID := d.Get("connection_id").(string)
	ctx = tflog.SetField(ctx, "connection_id", connectionID)

	tflog.Info(ctx, "Reading runbooks path data source")
	c := m.(*client.Client)

	// Get runbooks plugin
	plugin, err := c.GetPlugin(ctx, "runbooks")
	if err != nil {
		tflog.Error(ctx, "Failed to get runbooks plugin", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error getting runbooks plugin: %s", err))
	}

	// Find the connection in the plugin
	var connectionName string
	var path string
	connectionFound := false

	for _, conn := range plugin.Connections {
		if conn.ID == connectionID {
			connectionFound = true
			connectionName = conn.Name

			if len(conn.Config) > 0 {
				path = conn.Config[0]
			}
			break
		}
	}

	if !connectionFound {
		return diag.FromErr(fmt.Errorf("connection with ID %s not found in runbooks plugin", connectionID))
	}

	// Set the ID to the format used by the resource
	d.SetId(fmt.Sprintf("runbooks:%s", connectionID))

	if err := d.Set("connection_name", connectionName); err != nil {
		tflog.Error(ctx, "Error setting connection_name", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	if err := d.Set("path", path); err != nil {
		tflog.Error(ctx, "Error setting path", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	// Note: last_updated isn't available from the API for data sources
	// but we keep the field for consistency with the resource

	tflog.Info(ctx, "Successfully read runbooks path data source")
	return diags
}
