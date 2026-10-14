// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                   = &accessRequestRuleResource{}
	_ resource.ResourceWithImportState    = &accessRequestRuleResource{}
	_ resource.ResourceWithValidateConfig = &accessRequestRuleResource{}
)

func NewAccessRequestRuleResource() resource.Resource {
	return &accessRequestRuleResource{}
}

const accessTypeSidecar = "sidecar"

type accessRequestRuleResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	AccessType             types.String `tfsdk:"access_type"`
	ConnectionNames        types.List   `tfsdk:"connection_names"`
	Attributes             types.List   `tfsdk:"attributes"`
	ApprovalRequiredGroups types.List   `tfsdk:"approval_required_groups"`
	AllGroupsMustApprove   types.Bool   `tfsdk:"all_groups_must_approve"`
	ReviewersGroups        types.List   `tfsdk:"reviewers_groups"`
	ForceApprovalGroups    types.List   `tfsdk:"force_approval_groups"`
	SkipReviewGroups       types.List   `tfsdk:"skip_review_groups"`
	AccessMaxDuration      types.Int64  `tfsdk:"access_max_duration"`
	MinApprovals           types.Int64  `tfsdk:"min_approvals"`
	PendingTTLSec          types.Int64  `tfsdk:"pending_ttl_sec"`
	ApprovalTTLSec         types.Int64  `tfsdk:"approval_ttl_sec"`
	ManagedBy              types.String `tfsdk:"managed_by"`
}

type accessRequestRuleResource struct {
	client *hoop.Client
}

var accessTypeValidator = []validator.String{
	stringvalidator.OneOf("jit", "command", "jit_command", accessTypeSidecar),
}

func (r *accessRequestRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_request_rule"
}

func groupListAttribute(description string) schema.ListAttribute {
	return schema.ListAttribute{
		Description: description,
		ElementType: types.StringType,
		Optional:    true,
	}
}

func (r *accessRequestRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an access request rule: who approves access, and for how long. With `access_type` " +
			"`jit`, `command` or `jit_command` the rule gates the connections in `connection_names`. With " +
			"`access_type` `sidecar` it gates no connection: it is the policy a sidecar review is filed under, " +
			"named by `approval_rule` in an analyzer rule's `sidecar_spec`, and says who may release a statement " +
			"the sidecar held. An analyzer rule that holds under its own name gets such a rule created and managed " +
			"by the gateway; this resource is for a rule several analyzer rules share.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The unique name of the rule. The gateway addresses the rule by it, so a change replaces the rule.",
				Required:    true,
				Validators:  NonEmptyStringValidator,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "The rule description.",
				Optional:    true,
			},
			"access_type": schema.StringAttribute{
				Description: "One of `jit`, `command`, `jit_command` or `sidecar`. A change replaces the rule.",
				Required:    true,
				Validators:  accessTypeValidator,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"connection_names": groupListAttribute("The connections this rule gates. Required, or `attributes`, " +
				"unless `access_type` is `sidecar`, which refuses it."),
			"attributes": groupListAttribute("Attribute names this rule is associated with."),
			"approval_required_groups": groupListAttribute("User groups whose members need approval. " +
				"Empty makes every requester need approval."),
			"all_groups_must_approve": schema.BoolAttribute{
				Description: "When true, one approval from each of `reviewers_groups` is needed and `min_approvals` is ignored.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"reviewers_groups": groupListAttribute("Groups whose members may approve. At least one for a " +
				"connection rule; for a sidecar rule, empty means the admin group reviews."),
			"force_approval_groups": groupListAttribute("Groups whose members may approve on their own."),
			"skip_review_groups": groupListAttribute("Groups whose members skip the review. Only with an empty " +
				"`approval_required_groups`."),
			"access_max_duration": schema.Int64Attribute{
				Description: "Maximum access duration in seconds, for connection rules.",
				Optional:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"min_approvals": schema.Int64Attribute{
				Description: "Approvals needed before access is granted. At least 1 unless `all_groups_must_approve`.",
				Optional:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"pending_ttl_sec": schema.Int64Attribute{
				Description: "Seconds a sidecar review may wait for a decision, 60 to 604800. Sidecar rules only.",
				Optional:    true,
				Validators:  reviewTTLValidator,
			},
			"approval_ttl_sec": schema.Int64Attribute{
				Description: "Seconds a sidecar approval lasts, 60 to 604800. Sidecar rules only.",
				Optional:    true,
				Validators:  reviewTTLValidator,
			},
			"managed_by": schema.StringAttribute{
				Description: "`hoop` for a protection profile's rule, `ai-session-analyzer` for the rule an analyzer " +
					"rule holds under, empty otherwise. A managed rule accepts changes to its groups and approval " +
					"counts only, and cannot be deleted.",
				Computed: true,
			},
		},
	}
}

// ValidateConfig refuses at plan what the gateway refuses at write, with the
// attribute named: connection names on a sidecar rule, review limits on a
// connection rule, and a connection rule that gates nothing.
func (r *accessRequestRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data accessRequestRuleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.AccessType.IsUnknown() || data.AccessType.IsNull() {
		return
	}
	sidecar := data.AccessType.ValueString() == accessTypeSidecar
	known := func(l types.List) bool { return !l.IsNull() && !l.IsUnknown() }
	switch {
	case sidecar && known(data.ConnectionNames) && len(data.ConnectionNames.Elements()) > 0:
		resp.Diagnostics.AddAttributeError(path.Root("connection_names"), "Sidecar Rule Gates No Connection",
			"A rule with access_type sidecar is named by analyzer rules through approval_rule and refuses connection_names.")
	case !sidecar && (!data.PendingTTLSec.IsNull() || !data.ApprovalTTLSec.IsNull()):
		resp.Diagnostics.AddAttributeError(path.Root("pending_ttl_sec"), "Review Limits Are For Sidecar Rules",
			"pending_ttl_sec and approval_ttl_sec apply to access_type sidecar. The gateway drops them on a connection rule, and the plan would never converge.")
	case !sidecar && known(data.ConnectionNames) && known(data.Attributes) &&
		len(data.ConnectionNames.Elements()) == 0 && len(data.Attributes.Elements()) == 0:
		resp.Diagnostics.AddAttributeError(path.Root("connection_names"), "Connection Rule Gates Nothing",
			"A rule with access_type jit, command or jit_command needs connection_names or attributes.")
	}
}

func (m *accessRequestRuleResourceModel) toApi(ctx context.Context, prior *accessRequestRuleResourceModel) (hoop.AccessRequestRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	rule := hoop.AccessRequestRule{
		ID:                   m.ID.ValueString(),
		Name:                 m.Name.ValueString(),
		AccessType:           m.AccessType.ValueString(),
		AllGroupsMustApprove: m.AllGroupsMustApprove.ValueBool(),
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		s := m.Description.ValueString()
		rule.Description = &s
	}
	var d diag.Diagnostics
	rule.ConnectionNames, d = toApiStringList(ctx, m.ConnectionNames)
	diags.Append(d...)
	rule.Attributes, d = toApiStringList(ctx, m.Attributes)
	diags.Append(d...)
	rule.ApprovalRequiredGroups, d = toApiStringList(ctx, m.ApprovalRequiredGroups)
	diags.Append(d...)
	rule.ReviewersGroups, d = toApiStringList(ctx, m.ReviewersGroups)
	diags.Append(d...)
	rule.ForceApprovalGroups, d = toApiStringList(ctx, m.ForceApprovalGroups)
	diags.Append(d...)
	rule.SkipReviewGroups, d = toApiStringList(ctx, m.SkipReviewGroups)
	diags.Append(d...)
	rule.AccessMaxDuration = toApiInt(m.AccessMaxDuration)
	rule.MinApprovals = toApiInt(m.MinApprovals)

	priorPending, priorApproval := types.Int64Null(), types.Int64Null()
	if prior != nil {
		priorPending, priorApproval = prior.PendingTTLSec, prior.ApprovalTTLSec
	}
	rule.PendingTTLSec = toApiTTL(m.PendingTTLSec, priorPending)
	rule.ApprovalTTLSec = toApiTTL(m.ApprovalTTLSec, priorApproval)
	return rule, diags
}

func (m *accessRequestRuleResourceModel) fromApi(ctx context.Context, rule *hoop.AccessRequestRule) diag.Diagnostics {
	var diags diag.Diagnostics
	var d diag.Diagnostics
	m.ID = types.StringValue(rule.ID)
	m.Name = types.StringValue(rule.Name)
	m.Description = fromApiString(m.Description, derefString(rule.Description))
	m.AccessType = types.StringValue(rule.AccessType)
	m.AllGroupsMustApprove = types.BoolValue(rule.AllGroupsMustApprove)
	m.ConnectionNames, d = fromApiStringList(ctx, m.ConnectionNames, rule.ConnectionNames)
	diags.Append(d...)
	m.Attributes, d = fromApiStringList(ctx, m.Attributes, rule.Attributes)
	diags.Append(d...)
	m.ApprovalRequiredGroups, d = fromApiStringList(ctx, m.ApprovalRequiredGroups, rule.ApprovalRequiredGroups)
	diags.Append(d...)
	m.ReviewersGroups, d = fromApiStringList(ctx, m.ReviewersGroups, rule.ReviewersGroups)
	diags.Append(d...)
	m.ForceApprovalGroups, d = fromApiStringList(ctx, m.ForceApprovalGroups, rule.ForceApprovalGroups)
	diags.Append(d...)
	m.SkipReviewGroups, d = fromApiStringList(ctx, m.SkipReviewGroups, rule.SkipReviewGroups)
	diags.Append(d...)
	m.AccessMaxDuration = fromApiInt(rule.AccessMaxDuration)
	m.MinApprovals = fromApiInt(rule.MinApprovals)
	// This resource owns the whole rule, so a limit the gateway holds is
	// state even when the configuration never set it; the next apply then
	// clears it with 0. The analyzer resource reads the same two fields the
	// other way round, because there the limit sits on a rule the gateway
	// manages for it.
	m.PendingTTLSec = fromApiLimit(rule.PendingTTLSec)
	m.ApprovalTTLSec = fromApiLimit(rule.ApprovalTTLSec)
	m.ManagedBy = types.StringValue(derefString(rule.ManagedBy))
	return diags
}

func toApiInt(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := int(v.ValueInt64())
	return &i
}

// fromApiLimit reads a review limit: absent and 0 both mean "no limit", and
// the gateway stores 0 as absent (services.StoredSidecarReviewTTL).
func fromApiLimit(v *int) types.Int64 {
	if v == nil || *v == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}

func fromApiInt(v *int) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}

func (r *accessRequestRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessRequestRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := plan.toApi(ctx, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.CreateAccessRequestRule(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Access Request Rule",
			fmt.Sprintf("Failed to create access request rule %q: %v", plan.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(plan.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accessRequestRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accessRequestRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.GetAccessRequestRule(state.Name.ValueString())
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Access Request Rule",
			fmt.Sprintf("Failed reading access request rule %q: %v", state.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(state.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accessRequestRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accessRequestRuleResourceModel
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
	rule, err := r.client.UpdateAccessRequestRule(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Access Request Rule",
			fmt.Sprintf("Failed to update access request rule %q: %v", plan.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(plan.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the rule. A managed rule cannot be deleted on the gateway;
// the error says so, and the rule stays in state until it is removed from
// the configuration with `terraform state rm` or the managing object goes.
func (r *accessRequestRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accessRequestRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteAccessRequestRule(state.Name.ValueString())
	if err != nil && !hoop.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Access Request Rule",
			fmt.Sprintf("Failed to delete access request rule %q: %v", state.Name.ValueString(), err))
	}
}

// ImportState takes the rule name, which is how the gateway addresses it.
func (r *accessRequestRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *accessRequestRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
