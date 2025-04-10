package provider

import (
	"context"

	"github.com/hoophq/terraform-provider-hoop/client"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"api_url": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"hoop_connection":   resourceConnection(),
			"hoop_access_group": resourceAccessGroup(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"hoop_connection": dataSourceConnection(),
		},
	}

	p.ConfigureContextFunc = providerConfigure
	return p
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Initialize provider logging
	ctx = tflog.SetField(ctx, "provider", "hoop")
	tflog.Info(ctx, "Configuring Hoop provider")

	apiKey := d.Get("api_key").(string)
	apiUrl := d.Get("api_url").(string)

	tflog.Debug(ctx, "Creating Hoop client", map[string]interface{}{
		"api_url": apiUrl,
	})

	c := client.NewClient(apiUrl, apiKey)

	tflog.Info(ctx, "Hoop provider configured successfully")
	return c, diags
}
