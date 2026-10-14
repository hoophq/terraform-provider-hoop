// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
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
	_ resource.Resource                = &guardrailResource{}
	_ resource.ResourceWithImportState = &guardrailResource{}
)

func NewGuardrailResource() resource.Resource {
	return &guardrailResource{}
}

type guardrailResourceModel struct {
	ID             types.String         `tfsdk:"id"`
	Name           types.String         `tfsdk:"name"`
	Description    types.String         `tfsdk:"description"`
	Input          jsontypes.Normalized `tfsdk:"input"`
	Output         jsontypes.Normalized `tfsdk:"output"`
	ConnectionIDs  types.List           `tfsdk:"connection_ids"`
	Attributes     types.List           `tfsdk:"attributes"`
	SidecarSpec    jsontypes.Normalized `tfsdk:"sidecar_spec"`
	SidecarTargets types.List           `tfsdk:"sidecar_targets"`
}

type guardrailResource struct {
	client *hoop.Client
}

// emptyRules is what the gateway stores for a direction with no rules. A
// resource that leaves input or output out sends it, and reads it back as
// null.
const emptyRules = `{"rules":[]}`

func (r *guardrailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guardrail"
}

func (r *guardrailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a guardrail rule. On the gateway, `input` and `output` inspect the sessions of the " +
			"connections in `connection_ids`. On a sidecar fleet, `sidecar_spec` is the rule in the sidecar's " +
			"vocabulary and `sidecar_targets` names the listeners that enforce it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The unique name of the rule.",
				Required:    true,
				Validators:  NonEmptyStringValidator,
			},
			"description": schema.StringAttribute{
				Description: "The rule description.",
				Optional:    true,
			},
			"input": jsonAttribute("Rules applied to what the user sends, as a JSON object " +
				"`{\"rules\": [{\"type\": \"deny_words_list\", \"words\": [...]}]}`. " +
				"Omit it for a rule with no input checks."),
			"output": jsonAttribute("Rules applied to what the connection returns, in the same shape as `input`. " +
				"Omit it for a rule with no output checks."),
			"connection_ids": schema.ListAttribute{
				Description: "The connections this rule applies to on the gateway.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"attributes": schema.ListAttribute{
				Description: "Attribute names this rule is associated with.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"sidecar_spec":    sidecarSpecAttribute(),
			"sidecar_targets": sidecarTargetsAttribute(),
		},
	}
}

func (m *guardrailResourceModel) toApi(ctx context.Context, prior *guardrailResourceModel) (hoop.GuardRailRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	rule := hoop.GuardRailRule{
		ID:          m.ID.ValueString(),
		Name:        m.Name.ValueString(),
		Description: m.Description.ValueString(),
		Input:       toApiJSONOr(m.Input, emptyRules),
		Output:      toApiJSONOr(m.Output, emptyRules),
	}
	var d diag.Diagnostics
	rule.ConnectionIDs, d = toApiStringList(ctx, m.ConnectionIDs)
	diags.Append(d...)
	rule.Attributes, d = toApiStringList(ctx, m.Attributes)
	diags.Append(d...)

	priorSpec := jsontypes.NewNormalizedNull()
	priorTargets := types.ListNull(sidecarTargetObjectType)
	if prior != nil {
		priorSpec = prior.SidecarSpec
		priorTargets = prior.SidecarTargets
	}
	rule.SidecarSpec = toApiJSON(m.SidecarSpec, priorSpec)
	rule.SidecarTargets, d = toApiSidecarTargets(ctx, m.SidecarTargets, priorTargets)
	diags.Append(d...)
	return rule, diags
}

func (m *guardrailResourceModel) fromApi(ctx context.Context, rule *hoop.GuardRailRule) diag.Diagnostics {
	var diags diag.Diagnostics
	var d diag.Diagnostics
	m.ID = types.StringValue(rule.ID)
	m.Name = types.StringValue(rule.Name)
	m.Description = fromApiString(m.Description, rule.Description)
	m.Input, d = fromApiJSONOr(ctx, m.Input, rule.Input, emptyRules)
	diags.Append(d...)
	m.Output, d = fromApiJSONOr(ctx, m.Output, rule.Output, emptyRules)
	diags.Append(d...)
	m.ConnectionIDs, d = fromApiStringList(ctx, m.ConnectionIDs, rule.ConnectionIDs)
	diags.Append(d...)
	m.Attributes, d = fromApiStringList(ctx, m.Attributes, rule.Attributes)
	diags.Append(d...)
	m.SidecarSpec = fromApiJSON(m.SidecarSpec, rule.SidecarSpec)
	m.SidecarTargets, d = fromApiSidecarTargets(ctx, m.SidecarTargets, hoop.TargetsOf(rule.SidecarTargets))
	diags.Append(d...)
	return diags
}

func (r *guardrailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan guardrailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := plan.toApi(ctx, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.CreateGuardRailRule(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Guardrail",
			fmt.Sprintf("Failed to create guardrail %q: %v", plan.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(plan.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *guardrailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state guardrailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.GetGuardRailRule(state.ID.ValueString())
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Guardrail",
			fmt.Sprintf("Failed reading guardrail with ID %q: %v", state.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(state.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *guardrailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state guardrailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := plan.toApi(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload.ID = state.ID.ValueString()
	rule, err := r.client.UpdateGuardRailRule(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Guardrail",
			fmt.Sprintf("Failed to update guardrail %q: %v", plan.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(plan.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *guardrailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state guardrailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteGuardRailRule(state.ID.ValueString())
	if err != nil && !hoop.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Guardrail",
			fmt.Sprintf("Failed to delete guardrail %q: %v", state.Name.ValueString(), err))
	}
}

func (r *guardrailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *guardrailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
