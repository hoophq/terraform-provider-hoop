// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                = &agentResource{}
	_ resource.ResourceWithConfigure   = &agentResource{}
	_ resource.ResourceWithImportState = &agentResource{}
)

func NewAgentResource() resource.Resource {
	return &agentResource{}
}

type agentResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Mode   types.String `tfsdk:"mode"`
	Token  types.String `tfsdk:"token"`
	Status types.String `tfsdk:"status"`
}

type agentResource struct {
	client *hoop.Client
}

func (r *agentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *agentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage an agent resource in Hoop Platform. The generated token is returned only when the agent is created and is stored as sensitive Terraform state. Imported agents cannot recover an existing token because the Hoop API never returns agent secrets after creation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the agent resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the agent resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: AgentNameValidator,
			},
			"mode": schema.StringAttribute{
				Description: "The agent operation mode. Valid values are `standard` or `embedded`.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: AgentModeValidator,
			},
			"token": schema.StringAttribute{
				Description: "The generated agent token in DSN form. Hoop only returns this value at creation time, so Terraform preserves it in sensitive state for downstream resources. This value is unavailable for imported agents.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "The current agent connection status reported by Hoop.",
				Computed:    true,
			},
		},
	}
}

func (r *agentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agentResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	mode := plan.Mode.ValueString()
	if plan.Mode.IsNull() || plan.Mode.IsUnknown() || mode == "" {
		mode = "standard"
	}

	agent, err := r.client.CreateAgent(plan.Name.ValueString(), mode)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Agent",
			fmt.Sprintf("Failed to create agent: %v", err),
		)
		return
	}

	state := agentResourceModel{
		ID:     types.StringValue(agent.ID),
		Name:   types.StringValue(agent.Name),
		Mode:   types.StringValue(agent.Mode),
		Token:  types.StringValue(agent.Token),
		Status: types.StringValue(agent.Status),
	}
	if state.Mode.ValueString() == "" {
		state.Mode = types.StringValue(mode)
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *agentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameOrID := state.ID.ValueString()
	if nameOrID == "" {
		nameOrID = state.Name.ValueString()
	}

	agent, err := r.client.GetAgent(nameOrID)
	if err != nil {
		var apiErr *hoop.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Agent",
			fmt.Sprintf("Failed to read agent: %v", err),
		)
		return
	}

	// The gateway intentionally never returns the agent token after creation.
	// Preserve the sensitive value already held in Terraform state.
	state.ID = types.StringValue(agent.ID)
	state.Name = types.StringValue(agent.Name)
	if agent.Mode == "" {
		state.Mode = types.StringValue("standard")
	} else {
		state.Mode = types.StringValue(agent.Mode)
	}
	state.Status = types.StringValue(agent.Status)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *agentResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	// name and mode both require replacement, and all remaining attributes are
	// computed, so Terraform should never call Update.
}

func (r *agentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agentResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameOrID := state.ID.ValueString()
	if nameOrID == "" {
		nameOrID = state.Name.ValueString()
	}
	if err := r.client.DeleteAgent(nameOrID); err != nil {
		var apiErr *hoop.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Agent",
			fmt.Sprintf("Failed to delete agent: %v", err),
		)
	}
}

func (r *agentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *agentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
