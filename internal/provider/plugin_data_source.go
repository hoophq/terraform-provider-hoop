// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &pluginDataSource{}
	_ datasource.DataSourceWithConfigure = &pluginDataSource{}
)

// NewPluginpluginDataSource is a helper function to simplify the provider implementation.
func NewPluginpluginDataSource() datasource.DataSource {
	return &pluginDataSource{}

}

type pluginDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

// pluginDataSource is the data source implementation.
type pluginDataSource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (d *pluginDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin"
}

// Schema defines the schema for the data source.
func (d *pluginDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *pluginDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state pluginDataSourceModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plugin, err := d.client.GetPlugin(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Plugin",
			fmt.Sprintf("failed reading plugin %q, reason=%v", state.Name.String(), err),
		)
		return
	}
	state.ID = types.StringValue(plugin.ID)
	state.Name = types.StringValue(plugin.Name)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *pluginDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*hoop.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *hoop.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
	d.client = client
}
