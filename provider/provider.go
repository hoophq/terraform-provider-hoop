package provider

import (
	"context"
	"terraform-provider-hoop/client"

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
			"hoop_connection": resourceConnection(),
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

	apiKey := d.Get("api_key").(string)
	apiUrl := d.Get("api_url").(string)

	c := client.NewClient(apiUrl, apiKey)

	return c, diags
}
