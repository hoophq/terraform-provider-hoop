// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &pluginConfigResource{}
	_ resource.ResourceWithConfigure   = &pluginConfigResource{}
	_ resource.ResourceWithImportState = &pluginConfigResource{}
)

// NewPluginConfigResource is a helper function to simplify the provider implementation.
func NewPluginConfigResource() resource.Resource {
	return &pluginConfigResource{}
}

// pluginConfigResourceModel maps the data source schema data.
type pluginConfigResourceModel struct {
	ID         types.String `tfsdk:"id"`
	PluginName types.String `tfsdk:"plugin_name"`
	Config     types.Map    `tfsdk:"config"`
}

// pluginConfigResource is the data source implementation.
type pluginConfigResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *pluginConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin_config"
}

// Schema defines the schema for the data source.
func (r *pluginConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Plugin Config resources allows configuring plugin definitions. The supported plugins that accept configurations are: `slack`, and `runbooks`. Make sure to work with this resource only with the gateway version 1.39.1 and onwards.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"plugin_name": schema.StringAttribute{
				Description: "The name of the plugin that this configuration refers to. Accepted values are: `slack`, and `runbooks`.",
				Required:    true,
				Validators:  PluginConfigNameValidator,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config": schema.MapAttribute{
				Description: "A map of generic configuration required for this plugin.",
				Required:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyMapValidator,
				Sensitive:   true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *pluginConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentState pluginConfigResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	pluginConf, err := r.client.GetPluginConfig(currentState.PluginName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Plugin Config",
			fmt.Sprintf("Failed reading plugin %q: %v",
				currentState.PluginName.ValueString(), err),
		)
		return
	}

	if pluginConf == nil {
		resp.Diagnostics.AddError(
			"Plugin Config Not Found",
			fmt.Sprintf("No plugin config found for plugin %q", currentState.PluginName.ValueString()),
		)
		return
	}

	currentState.ID = types.StringValue(pluginConf.ID)
	currentState.Config, diags = types.MapValueFrom(ctx, types.StringType, pluginConf.EnvVars)
	if diags.HasError() {
		return
	}

	diags = resp.State.Set(ctx, currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *pluginConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pluginConfigResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config map[string]string
	diags = plan.Config.ElementsAs(ctx, &config, false)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config to Map",
			fmt.Sprintf("Failed to convert config to map: %v", diags),
		)
		return
	}

	pluginConfig, err := r.client.UpdatePluginConfig(plan.PluginName.ValueString(), config)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Plugin Configuration",
			fmt.Sprintf("Failed to create plugin config: %v", err),
		)
		return
	}

	plan.ID = types.StringValue(pluginConfig.ID)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *pluginConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pluginConfigResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config map[string]string
	diags = plan.Config.ElementsAs(ctx, &config, false)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config to Map",
			fmt.Sprintf("Failed to convert config to map: %v", diags),
		)
		return
	}

	pluginConfig, err := r.client.UpdatePluginConfig(plan.PluginName.ValueString(), config)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Plugin Configuration",
			fmt.Sprintf("Failed to update plugin config: %v", err),
		)
		return
	}

	plan.Config, diags = types.MapValueFrom(ctx, types.StringType, pluginConfig.EnvVars)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Config Map",
			fmt.Sprintf("Failed to convert config map: %v", diags),
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
func (r *pluginConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pluginConfigResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePluginConfig(state.PluginName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Plugin Configuration",
			fmt.Sprintf("Failed to delete plugin config: %v", err),
		)
		return
	}
}

func (r *pluginConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	pluginName := req.ID
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("plugin_name"), pluginName)...)
}

// Configure adds the provider configured client to the data source.
func (r *pluginConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
