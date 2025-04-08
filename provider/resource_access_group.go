package provider

import (
	"context"
	"fmt"

	"github.com/hoophq/terraform-provider-hoop/client"
	"github.com/hoophq/terraform-provider-hoop/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAccessGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAccessGroupCreate,
		ReadContext:   resourceAccessGroupRead,
		UpdateContext: resourceAccessGroupUpdate,
		DeleteContext: resourceAccessGroupDelete,
		Schema: map[string]*schema.Schema{
			"group": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The group of users that will have access to the connections",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the access group",
			},
			"connections": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "List of connection names that this group can access",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceAccessGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	// var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "access_group")
	groupName := d.Get("group").(string)
	ctx = tflog.SetField(ctx, "group_name", groupName)

	tflog.Info(ctx, "Creating access group resource")
	c := m.(*client.Client)

	// Build the access group model
	accessGroup := &models.AccessGroup{
		Name:        groupName,
		Description: d.Get("description").(string),
		Connections: expandStringList(d.Get("connections").([]interface{})),
	}

	tflog.Debug(ctx, "Prepared access group model", map[string]interface{}{
		"name":        accessGroup.Name,
		"connections": accessGroup.Connections,
	})

	// Create the access group
	err := c.CreateAccessGroup(ctx, accessGroup)
	if err != nil {
		tflog.Error(ctx, "Failed to create access group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully created access group, setting ID in state")
	d.SetId(accessGroup.Name)

	return resourceAccessGroupRead(ctx, d, m)
}

func resourceAccessGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "access_group")
	ctx = tflog.SetField(ctx, "group_name", d.Id())

	tflog.Info(ctx, "Reading access group resource")
	c := m.(*client.Client)

	accessGroup, err := c.GetAccessGroup(ctx, d.Id())
	if err != nil {
		tflog.Error(ctx, "Failed to get access group from API", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "Setting access group attributes in state")

	// Set all attributes in state
	if err := d.Set("group", accessGroup.Name); err != nil {
		tflog.Error(ctx, "Error setting group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	if err := d.Set("description", accessGroup.Description); err != nil {
		tflog.Error(ctx, "Error setting description", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	if err := d.Set("connections", accessGroup.Connections); err != nil {
		tflog.Error(ctx, "Error setting connections", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Info(ctx, "Successfully read access group resource")
	return diags
}

func resourceAccessGroupUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "access_group")
	ctx = tflog.SetField(ctx, "group_name", d.Id())

	tflog.Info(ctx, "Updating access group resource")
	c := m.(*client.Client)

	// Build the access group model
	accessGroup := &models.AccessGroup{
		Name:        d.Id(),
		Description: d.Get("description").(string),
		Connections: expandStringList(d.Get("connections").([]interface{})),
	}

	// Record which fields are changing
	var changedFields []string
	if d.HasChange("description") {
		changedFields = append(changedFields, "description")
	}
	if d.HasChange("connections") {
		changedFields = append(changedFields, "connections")
	}

	tflog.Info(ctx, "Updating access group with changed fields", map[string]interface{}{
		"changed_fields": changedFields,
	})

	// Update the access group
	err := c.UpdateAccessGroup(ctx, accessGroup)
	if err != nil {
		tflog.Error(ctx, "Failed to update access group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error updating access group: %v", err))
	}

	tflog.Info(ctx, "Access group updated successfully")

	// Read the access group again to ensure state consistency
	return resourceAccessGroupRead(ctx, d, m)
}

func resourceAccessGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "access_group")

	// Get access group name from ID
	groupName := d.Id()
	ctx = tflog.SetField(ctx, "group_name", groupName)

	tflog.Info(ctx, "Deleting access group resource")
	c := m.(*client.Client)

	// Delete access group
	if err := c.DeleteAccessGroup(ctx, groupName); err != nil {
		tflog.Error(ctx, "Failed to delete access group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error deleting access group %s: %v", groupName, err))
	}

	tflog.Info(ctx, "Access group deleted successfully")

	// Clear the ID from state
	d.SetId("")

	return diags
}

// expandStringList converte uma lista de interface{} em uma lista de string
func expandStringList(list []interface{}) []string {
	vs := make([]string, 0, len(list))
	for _, v := range list {
		val, ok := v.(string)
		if ok && val != "" {
			vs = append(vs, val)
		}
	}
	return vs
}
