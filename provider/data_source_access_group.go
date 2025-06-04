package provider

import (
	"context"

	"github.com/hoophq/terraform-provider-hoop/client"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAccessGroup() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAccessGroupRead,
		Schema: map[string]*schema.Schema{
			"group": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the access group to lookup",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Description of the access group",
			},
			"connections": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of connection names that this group can access",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourceAccessGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "data_source", "access_group")

	groupName := d.Get("group").(string)
	ctx = tflog.SetField(ctx, "group_name", groupName)

	tflog.Info(ctx, "Reading access group data source")
	c := m.(*client.Client)

	accessGroup, err := c.GetAccessGroup(ctx, groupName)
	if err != nil {
		tflog.Error(ctx, "Failed to get access group from API", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	// Set the ID to the group name
	d.SetId(accessGroup.Name)

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

	tflog.Info(ctx, "Successfully read access group data source")
	return diags
}
