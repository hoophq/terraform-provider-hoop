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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &userResource{}
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

// NewUserResource is a helper function to simplify the provider implementation.
func NewUserResource() resource.Resource {
	return &userResource{}
}

// userResourceModel maps the data source schema data.
type userResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Email  types.String `tfsdk:"email"`
	Groups types.List   `tfsdk:"groups"`
	Status types.String `tfsdk:"status"`
}

// userResource is the data source implementation.
type userResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

// Schema defines the schema for the data source.
func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage user and group resources. Do not use this terraform resource when managing groups via Identity Provider. Make sure to work with this resource only with the gateway version 1.39.1 and onwards.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Description: "The email address of the user.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"groups": schema.ListAttribute{
				Description: "Groups the user belongs to.",
				Required:    true,
				ElementType: types.StringType,
			},
			"status": schema.StringAttribute{
				Description: "The status of the user. Accepted values are: `active`, `inactive`.",
				Required:    true,
				Validators:  UserStatusValidator,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentState userResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(currentState.Email.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading User",
			fmt.Sprintf("Failed reading user %q: %v", currentState.Email.ValueString(), err),
		)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("Read user %q with ID %q, groups=%#v", user.Email, user.ID, user.Groups))

	currentState.ID = types.StringValue(user.ID)
	currentState.Status = types.StringValue(user.Status)
	currentState.Groups, diags = types.ListValueFrom(ctx, types.StringType, user.Groups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Groups",
			fmt.Sprintf("Failed to convert groups: %v", diags),
		)
		return
	}

	tflog.Info(ctx, fmt.Sprintf("Read user %q, elements=%#v", user.Email, currentState.Groups.Elements()))

	diags = resp.State.Set(ctx, currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	userGroups, diags := convertListToStringSlice(ctx, plan.Groups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Groups to Slice",
			fmt.Sprintf("Failed to convert groups to slice: %v", diags),
		)
		return
	}

	userResp, err := r.client.CreateUser(plan.Email.ValueString(), plan.Status.ValueString(), userGroups)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating User",
			fmt.Sprintf("Failed to create user: %v", err),
		)
		return
	}
	plan.ID = types.StringValue(userResp.ID)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan userResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	userGroups, diags := convertListToStringSlice(ctx, plan.Groups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Groups to Slice",
			fmt.Sprintf("Failed to convert groups to slice: %v", diags),
		)
		return
	}
	userResp, err := r.client.UpdateUser(plan.Email.ValueString(), plan.Status.ValueString(), userGroups)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating User",
			fmt.Sprintf("Failed to update user: %v", err),
		)
		return
	}
	plan.Status = types.StringValue(userResp.Status)
	plan.Groups, diags = types.ListValueFrom(ctx, types.StringType, userResp.Groups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Groups",
			fmt.Sprintf("Failed to convert groups: %v", diags),
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
func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteUser(state.Email.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting User",
			fmt.Sprintf("Failed to delete user %q: %v", state.Email.ValueString(), err),
		)
		return
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("email"), req, resp)
}

// Configure adds the provider configured client to the data source.
func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
