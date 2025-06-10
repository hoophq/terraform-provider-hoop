// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &pluginConnectionResource{}
	_ resource.ResourceWithConfigure   = &pluginConnectionResource{}
	_ resource.ResourceWithImportState = &pluginConnectionResource{}
)

// NewPluginpluginConnectionResource is a helper function to simplify the provider implementation.
func NewPluginpluginConnectionResource() resource.Resource {
	return &pluginConnectionResource{}
}

// pluginConnectionResourceModel maps the data source schema data.
type pluginConnectionResourceModel struct {
	PluginName   types.String `tfsdk:"plugin_name"`
	ConnectionID types.String `tfsdk:"connection_id"`
	Config       types.List   `tfsdk:"config"`
}

// pluginConnectionResource is the data source implementation.
type pluginConnectionResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *pluginConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin_connection"
}

// Schema defines the schema for the data source.
func (r *pluginConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Plugin Connection resources allows enabling features specific features to connection which are called plugins. The supported plugins are: `slack`, `webhooks`, `runbooks`, `access_control`.",
		Attributes: map[string]schema.Attribute{
			"plugin_name": schema.StringAttribute{
				Description: "The name of the plugin that this configuration refers to. Accepted values are: `slack`, `webhooks`, `runbooks`, `access_control`.",
				Required:    true,
				Validators:  PluginNameValidator,
			},
			"connection_id": schema.StringAttribute{
				Description: "The unique identifier of the connection.",
				Required:    true,
			},
			"config": schema.ListAttribute{
				Description: "A list of configuration values for the plugin connection. The values depend on the plugin type.",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *pluginConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	tflog.Info(ctx, "running read for plugin connection resource")
	var currentState pluginConnectionResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	pluginConn, err := r.client.GetPluginConnection(
		currentState.PluginName.ValueString(),
		currentState.ConnectionID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Plugin Connection",
			fmt.Sprintf("Failed reading plugin %q with connection id %q: %v",
				currentState.PluginName.ValueString(), currentState.ConnectionID.ValueString(), err),
		)
		return
	}

	currentState.ConnectionID = types.StringValue(pluginConn.ConnectionID)
	currentState.Config, diags = types.ListValueFrom(ctx, types.StringType, pluginConn.Config)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config List",
			fmt.Sprintf("Failed to convert config list: %v", diags),
		)
		return
	}

	diags = resp.State.Set(ctx, currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *pluginConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pluginConnectionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, diags := convertListToStringSlice(ctx, plan.Config)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config List",
			fmt.Sprintf("Failed to convert config list: %v", diags),
		)
		return
	}

	_, err := r.client.UpsertPluginConnection(
		plan.PluginName.ValueString(),
		plan.ConnectionID.ValueString(),
		config)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Plugin Connection",
			fmt.Sprintf("Failed to create plugin connection: %v", err),
		)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *pluginConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "running update for plugin connection resource")
	var plan pluginConnectionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, diags := convertListToStringSlice(ctx, plan.Config)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config List",
			fmt.Sprintf("Failed to convert config list: %v", diags),
		)
		return
	}

	pluginConn, err := r.client.UpsertPluginConnection(
		plan.PluginName.ValueString(),
		plan.ConnectionID.ValueString(),
		config,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Plugin Connection",
			fmt.Sprintf("Failed to update plugin connection: %v", err),
		)
		return
	}

	plan.ConnectionID = types.StringValue(pluginConn.ConnectionID)
	plan.Config, diags = types.ListValueFrom(ctx, types.StringType, pluginConn.Config)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config List",
			fmt.Sprintf("Failed to convert config list: %v", diags),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *pluginConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pluginConnectionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "running delete for plugin connection resource", map[string]interface{}{
		"plugin_name":   state.PluginName.ValueString(),
		"connection_id": state.ConnectionID.ValueString(),
	})
	err := r.client.DeletePluginConnection(state.PluginName.ValueString(), state.ConnectionID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Plugin Connection",
			fmt.Sprintf("Failed to delete plugin connection: %v", err),
		)
		return
	}
}

func (r *pluginConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	pluginName, connectionID, found := strings.Cut(req.ID, "/")
	if !found || pluginName == "" || connectionID == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format 'plugin_name/connection_id'",
		)
		return
	}

	// Set the individual attributes
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("plugin_name"), pluginName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("connection_id"), connectionID)...)
}

// Configure adds the provider configured client to the data source.
func (r *pluginConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
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
	r.client = client
}
