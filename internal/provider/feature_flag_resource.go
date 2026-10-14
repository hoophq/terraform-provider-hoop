// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                = &featureFlagResource{}
	_ resource.ResourceWithImportState = &featureFlagResource{}
)

func NewFeatureFlagResource() resource.Resource {
	return &featureFlagResource{}
}

type featureFlagResourceModel struct {
	Name        types.String `tfsdk:"name"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Description types.String `tfsdk:"description"`
	Default     types.Bool   `tfsdk:"default"`
	Stability   types.String `tfsdk:"stability"`
	Components  types.List   `tfsdk:"components"`
}

type featureFlagResource struct {
	client *hoop.Client
}

func (r *featureFlagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_feature_flag"
}

func (r *featureFlagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sets one of the gateway's feature flags for the organization, the switches under " +
			"Settings → Experimental. The flag must exist in the gateway's catalog; `GET /api/feature-flags` lists it. " +
			"For a sidecar fleet, `experimental.sidecar_session_events` decides whether sidecar traffic is recorded " +
			"as sessions; it is off by default. Destroying the resource puts the flag back to its catalog default.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The flag name, such as `experimental.sidecar_session_events`.",
				Required:    true,
				Validators:  NonEmptyStringValidator,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "The value for the organization.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "What the flag does, from the catalog.",
				Computed:    true,
			},
			"default": schema.BoolAttribute{
				Description: "The catalog default, restored on destroy.",
				Computed:    true,
			},
			"stability": schema.StringAttribute{
				Description: "`experimental` or `beta`.",
				Computed:    true,
			},
			"components": schema.ListAttribute{
				Description: "The binaries that read the flag: `gateway`, `agent`, `client`.",
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (m *featureFlagResourceModel) fromApi(ctx context.Context, f *hoop.FeatureFlag) diag.Diagnostics {
	m.Name = types.StringValue(f.Name)
	m.Enabled = types.BoolValue(f.Enabled)
	m.Description = types.StringValue(f.Description)
	m.Default = types.BoolValue(f.Default)
	m.Stability = types.StringValue(f.Stability)
	components := f.Components
	if components == nil {
		components = []string{}
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, components)
	m.Components = list
	return diags
}

func (r *featureFlagResource) set(ctx context.Context, model *featureFlagResourceModel, verb string) diag.Diagnostics {
	var diags diag.Diagnostics
	f, err := r.client.SetFeatureFlag(model.Name.ValueString(), model.Enabled.ValueBool())
	if err != nil {
		diags.AddError(fmt.Sprintf("Error %s Feature Flag", verb),
			fmt.Sprintf("Failed to set feature flag %q: %v", model.Name.ValueString(), err))
		return diags
	}
	diags.Append(model.fromApi(ctx, f)...)
	return diags
}

func (r *featureFlagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan featureFlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.set(ctx, &plan, "Creating")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *featureFlagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state featureFlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	f, err := r.client.GetFeatureFlag(state.Name.ValueString())
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Feature Flag",
			fmt.Sprintf("Failed reading feature flag %q: %v", state.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(state.fromApi(ctx, f)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *featureFlagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan featureFlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.set(ctx, &plan, "Updating")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete restores the catalog default. The gateway has no per-org "unset";
// a flag row at its default is the same as no row.
func (r *featureFlagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state featureFlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.SetFeatureFlag(state.Name.ValueString(), state.Default.ValueBool())
	if err != nil && !hoop.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Resetting Feature Flag",
			fmt.Sprintf("Failed to restore the default of feature flag %q: %v", state.Name.ValueString(), err))
	}
}

func (r *featureFlagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *featureFlagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*hoop.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *hoop.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}
