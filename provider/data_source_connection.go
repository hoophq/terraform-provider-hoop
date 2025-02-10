package provider

import (
	"context"
	"terraform-provider-hoop/internal"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceConnection() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceConnectionRead,
		Schema:      internal.CommonConnectionSchema(false),
	}
}

func dataSourceConnectionRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	connectionName := d.Get("name").(string)
	d.SetId(connectionName)

	return resourceConnectionRead(ctx, d, m)
}
