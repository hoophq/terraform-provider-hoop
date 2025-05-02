package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hoophq/terraform-provider-hoop/client"
	"github.com/hoophq/terraform-provider-hoop/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRunbooksPath() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRunbooksPathCreate,
		ReadContext:   resourceRunbooksPathRead,
		UpdateContext: resourceRunbooksPathUpdate,
		DeleteContext: resourceRunbooksPathDelete,
		Schema: map[string]*schema.Schema{
			"connection_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the connection to configure with a runbooks path",
			},
			"connection_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the connection to configure with a runbooks path",
			},
			"path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The path to set for runbooks. Set to empty string to remove the path.",
			},
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func resourceRunbooksPathCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "runbooks_path")
	connectionID := d.Get("connection_id").(string)
	connectionName := d.Get("connection_name").(string)
	path := d.Get("path").(string)

	ctx = tflog.SetField(ctx, "connection_id", connectionID)
	ctx = tflog.SetField(ctx, "connection_name", connectionName)

	tflog.Info(ctx, "Creating runbooks path resource")
	c := m.(*client.Client)

	// Get current runbooks plugin
	plugin, err := c.GetPlugin(ctx, "runbooks")
	if err != nil {
		if err.Error() == "plugin not found" {
			// Plugin doesn't exist yet, create it
			tflog.Info(ctx, "Runbooks plugin not found, creating it")
			plugin = &models.Plugin{
				Name:        "runbooks",
				Connections: []models.PluginConnection{},
				Source:      nil,
				Priority:    0,
			}
		} else {
			tflog.Error(ctx, "Failed to get runbooks plugin", map[string]interface{}{
				"error": err.Error(),
			})
			return diag.FromErr(fmt.Errorf("error getting runbooks plugin: %s", err))
		}
	}

	// Check if connection already exists in plugin
	connectionFound := false
	for i, conn := range plugin.Connections {
		if conn.ID == connectionID {
			connectionFound = true
			if path == "" {
				plugin.Connections[i].Config = nil
			} else {
				plugin.Connections[i].Config = []string{path}
			}
			break
		}
	}

	// If connection wasn't found in the plugin, add it
	if !connectionFound {
		newConn := models.PluginConnection{
			ID:   connectionID,
			Name: connectionName,
		}
		if path != "" {
			newConn.Config = []string{path}
		}
		plugin.Connections = append(plugin.Connections, newConn)
	}

	// Update or create the plugin
	var updateErr error
	if plugin.ID == "" {
		updateErr = c.CreatePlugin(ctx, plugin)
	} else {
		updateErr = c.UpdatePlugin(ctx, plugin)
	}

	if updateErr != nil {
		tflog.Error(ctx, "Failed to update/create runbooks plugin", map[string]interface{}{
			"error": updateErr.Error(),
		})
		return diag.FromErr(fmt.Errorf("error updating runbooks plugin: %s", updateErr))
	}

	// Set the resource ID to a composite of plugin name and connection ID
	d.SetId(fmt.Sprintf("runbooks:%s", connectionID))

	// Set the path value in the state
	if err := d.Set("path", path); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("last_updated", time.Now().Format(time.RFC3339)); err != nil {
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully created runbooks path resource")

	// Call ReadContext to ensure state is consistent but skip setting path to what API returns
	readCtx := context.WithValue(ctx, "skip_path_update", true)
	return resourceRunbooksPathRead(readCtx, d, m)
}

func resourceRunbooksPathRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "runbooks_path")

	connectionID := d.Get("connection_id").(string)
	connectionName := d.Get("connection_name").(string)
	pathInState := d.Get("path").(string)

	ctx = tflog.SetField(ctx, "connection_id", connectionID)
	ctx = tflog.SetField(ctx, "connection_name", connectionName)

	// Check if we should skip updating the path (used during create to preserve the original path)
	skipPathUpdate := false
	if val, ok := ctx.Value("skip_path_update").(bool); ok && val {
		skipPathUpdate = true
		tflog.Debug(ctx, "Will skip updating path value from API due to context flag")
	}

	tflog.Info(ctx, "Reading runbooks path resource")
	c := m.(*client.Client)

	// Get current runbooks plugin
	plugin, err := c.GetPlugin(ctx, "runbooks")
	if err != nil {
		if err.Error() == "plugin not found" {
			// If plugin doesn't exist, the resource is gone
			d.SetId("")
			return diags
		}

		tflog.Error(ctx, "Failed to get runbooks plugin", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error getting runbooks plugin: %s", err))
	}

	// Find the connection in the plugin
	connectionFound := false
	for _, conn := range plugin.Connections {
		if conn.ID == connectionID {
			connectionFound = true
			tflog.Debug(ctx, "Found connection in runbooks plugin", map[string]interface{}{
				"connection_id": connectionID,
			})

			// If we're skipping path updates, don't modify the path in state
			if skipPathUpdate {
				tflog.Debug(ctx, "Skipping path update from API as requested", map[string]interface{}{
					"path_in_state": pathInState,
				})
				break
			}

			// Get its path from API if available
			if len(conn.Config) > 0 && conn.Config[0] != "" {
				tflog.Debug(ctx, "Setting path from API response", map[string]interface{}{
					"path_from_api": conn.Config[0],
					"path_in_state": pathInState,
				})
				if err := d.Set("path", conn.Config[0]); err != nil {
					return diag.FromErr(err)
				}
			} else if pathInState != "" {
				// If API returns empty path but we have a path in state, preserve it
				tflog.Debug(ctx, "Preserving path from state as API returned empty value", map[string]interface{}{
					"path": pathInState,
				})
				if err := d.Set("path", pathInState); err != nil {
					return diag.FromErr(err)
				}
			} else {
				// Both API and state have empty path
				if err := d.Set("path", ""); err != nil {
					return diag.FromErr(err)
				}
			}
			break
		}
	}

	// If connection not found in plugin, the resource is gone
	if !connectionFound {
		tflog.Info(ctx, "Connection not found in runbooks plugin, removing from state", map[string]interface{}{
			"connection_id": connectionID,
		})
		d.SetId("")
	}

	return diags
}

func resourceRunbooksPathUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "runbooks_path")
	connectionID := d.Get("connection_id").(string)
	connectionName := d.Get("connection_name").(string)
	path := d.Get("path").(string)

	ctx = tflog.SetField(ctx, "connection_id", connectionID)
	ctx = tflog.SetField(ctx, "connection_name", connectionName)

	tflog.Info(ctx, "Updating runbooks path resource")
	c := m.(*client.Client)

	// Get current runbooks plugin
	plugin, err := c.GetPlugin(ctx, "runbooks")
	if err != nil {
		tflog.Error(ctx, "Failed to get runbooks plugin", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error getting runbooks plugin: %s", err))
	}

	// Update the plugin with the new path for the connection
	pathFound := false
	for i, conn := range plugin.Connections {
		if conn.ID == connectionID {
			pathFound = true
			if path == "" {
				plugin.Connections[i].Config = nil
			} else {
				plugin.Connections[i].Config = []string{path}
			}
			break
		}
	}

	if !pathFound {
		tflog.Warn(ctx, "Connection not found in plugin during update, will be added", map[string]interface{}{
			"connection_id": connectionID,
		})

		// If connection wasn't found in the plugin, add it
		newConn := models.PluginConnection{
			ID:   connectionID,
			Name: d.Get("connection_name").(string),
		}
		if path != "" {
			newConn.Config = []string{path}
		}
		plugin.Connections = append(plugin.Connections, newConn)
	}

	// Update the plugin
	if err := c.UpdatePlugin(ctx, plugin); err != nil {
		tflog.Error(ctx, "Failed to update runbooks plugin", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error updating runbooks plugin: %s", err))
	}

	// Ensure path is preserved in state
	if err := d.Set("path", path); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("last_updated", time.Now().Format(time.RFC3339)); err != nil {
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully updated runbooks path resource")

	// Call read function with skip_path_update flag
	readCtx := context.WithValue(ctx, "skip_path_update", true)
	return resourceRunbooksPathRead(readCtx, d, m)
}

func resourceRunbooksPathDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "runbooks_path")

	connectionID := d.Get("connection_id").(string)
	ctx = tflog.SetField(ctx, "connection_id", connectionID)

	tflog.Info(ctx, "Deleting runbooks path resource")
	c := m.(*client.Client)

	// Get current runbooks plugin
	plugin, err := c.GetPlugin(ctx, "runbooks")
	if err != nil {
		if err.Error() == "plugin not found" {
			// If plugin doesn't exist, there's nothing to delete
			d.SetId("")
			return diags
		}

		tflog.Error(ctx, "Failed to get runbooks plugin", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error getting runbooks plugin: %s", err))
	}

	// Find and clear the path for the connection
	for i, conn := range plugin.Connections {
		if conn.ID == connectionID {
			tflog.Debug(ctx, "Found connection in runbooks plugin, removing config", map[string]interface{}{
				"connection_id": connectionID,
			})
			plugin.Connections[i].Config = nil
			break
		}
	}

	// Update the plugin
	if err := c.UpdatePlugin(ctx, plugin); err != nil {
		tflog.Error(ctx, "Failed to update runbooks plugin", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error updating runbooks plugin: %s", err))
	}

	// Remove the resource from state
	d.SetId("")

	tflog.Info(ctx, "Successfully deleted runbooks path resource")

	return diags
}
