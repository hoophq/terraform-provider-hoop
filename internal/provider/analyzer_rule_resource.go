// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.Resource                   = &analyzerRuleResource{}
	_ resource.ResourceWithImportState    = &analyzerRuleResource{}
	_ resource.ResourceWithModifyPlan     = &analyzerRuleResource{}
	_ resource.ResourceWithValidateConfig = &analyzerRuleResource{}
)

func NewAnalyzerRuleResource() resource.Resource {
	return &analyzerRuleResource{}
}

type analyzerRuleResourceModel struct {
	ID              types.String         `tfsdk:"id"`
	Name            types.String         `tfsdk:"name"`
	Description     types.String         `tfsdk:"description"`
	ConnectionNames types.List           `tfsdk:"connection_names"`
	RiskEvaluation  types.Object         `tfsdk:"risk_evaluation"`
	CustomPrompt    types.String         `tfsdk:"custom_prompt"`
	Agentic         types.Bool           `tfsdk:"agentic"`
	SidecarSpec     jsontypes.Normalized `tfsdk:"sidecar_spec"`
	SidecarTargets  types.List           `tfsdk:"sidecar_targets"`
	ReviewersGroups types.List           `tfsdk:"reviewers_groups"`
	PendingTTLSec   types.Int64          `tfsdk:"pending_ttl_sec"`
	ApprovalTTLSec  types.Int64          `tfsdk:"approval_ttl_sec"`
}

type riskTierModel struct {
	Action                types.String `tfsdk:"action"`
	AccessRequestRuleName types.String `tfsdk:"access_request_rule_name"`
}

type riskEvaluationModel struct {
	LowRisk    types.Object `tfsdk:"low_risk"`
	MediumRisk types.Object `tfsdk:"medium_risk"`
	HighRisk   types.Object `tfsdk:"high_risk"`
}

var riskTierObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"action":                   types.StringType,
		"access_request_rule_name": types.StringType,
	},
}

var riskEvaluationObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"low_risk":    riskTierObjectType,
		"medium_risk": riskTierObjectType,
		"high_risk":   riskTierObjectType,
	},
}

var riskActionValidator = []validator.String{
	stringvalidator.OneOf("allow_execution", "block_execution", "require_access_request"),
}

// reviewTTLValidator matches the gateway's bounds for pending_ttl_sec and
// approval_ttl_sec. Zero is allowed: it is how a declared limit is cleared.
var reviewTTLValidator = []validator.Int64{
	int64validator.Any(
		int64validator.OneOf(0),
		int64validator.Between(60, 604800),
	),
}

type analyzerRuleResource struct {
	client *hoop.Client
}

func (r *analyzerRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ai_session_analyzer_rule"
}

func riskTierAttribute(level string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: fmt.Sprintf("What the analyzer does with a session it classifies as %s risk.", level),
		Required:    true,
		Attributes: map[string]schema.Attribute{
			"action": schema.StringAttribute{
				Description: "One of `allow_execution`, `block_execution` or `require_access_request`.",
				Required:    true,
				Validators:  riskActionValidator,
			},
			"access_request_rule_name": schema.StringAttribute{
				Description: "The access request rule that supplies the approver groups when the action is `require_access_request`.",
				Optional:    true,
			},
		},
	}
}

func (r *analyzerRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AI session analyzer rule. On the gateway the rule classifies the sessions of " +
			"`connection_names` and applies `risk_evaluation`. On a sidecar fleet, `sidecar_spec` is the analyzer " +
			"block a listener receives and `sidecar_targets` names the listeners; one rule per listener. A sidecar " +
			"needs its own top-level `analyzer` section (provider, model, credential) in `configuration` before an " +
			"analyzer rule can be bound to it.",
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
			"connection_names": schema.ListAttribute{
				Description: "The connections this rule analyzes on the gateway. Leave it out for a rule that only sidecars run.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"risk_evaluation": schema.SingleNestedAttribute{
				Description: "The action for each risk level.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"low_risk":    riskTierAttribute("low"),
					"medium_risk": riskTierAttribute("medium"),
					"high_risk":   riskTierAttribute("high"),
				},
			},
			"custom_prompt": schema.StringAttribute{
				Description: "Extra instructions appended to the default system prompt.",
				Optional:    true,
			},
			"agentic": schema.BoolAttribute{
				Description: "When true the analyzer runs a tool-calling loop over past sessions and resource metadata before classifying.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"sidecar_spec":    sidecarSpecAttribute(),
			"sidecar_targets": sidecarTargetsAttribute(),
			"reviewers_groups": schema.ListAttribute{
				Description: "The groups whose members may release a statement this rule holds for approval. " +
					"Only for a rule whose `sidecar_spec` holds statements (`require_review`) with `approval_rule` set to " +
					"this rule's `name`: the gateway then manages an access request rule under that name. With an " +
					"`approval_rule` naming another rule, set the groups on that `hoop_access_request_rule`. " +
					"With none set the admin group reviews. Leave the attribute out to keep groups set outside Terraform; " +
					"removing a declared value clears them.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"pending_ttl_sec": schema.Int64Attribute{
				Description: "Seconds a held statement's review may wait for a decision, 60 to 604800. " +
					"Same conditions as `reviewers_groups`.",
				Optional:   true,
				Validators: reviewTTLValidator,
			},
			"approval_ttl_sec": schema.Int64Attribute{
				Description: "Seconds an approval lasts, 60 to 604800. Same conditions as `pending_ttl_sec`.",
				Optional:    true,
				Validators:  reviewTTLValidator,
			},
		},
	}
}

// ValidateConfig applies the gateway's rule for a risk tier
// (gateway/api/ai/ai.go validateRiskTier) at plan: require_access_request
// needs access_request_rule_name, every other action refuses it. Whether the
// named rule exists is still the gateway's to say.
func (r *analyzerRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data analyzerRuleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateAnalyzerSidecarSpec(data, &resp.Diagnostics)
	if data.RiskEvaluation.IsNull() || data.RiskEvaluation.IsUnknown() {
		return
	}
	var eval riskEvaluationModel
	resp.Diagnostics.Append(data.RiskEvaluation.As(ctx, &eval, basetypesObjectAsOptions)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for level, obj := range map[string]types.Object{"low_risk": eval.LowRisk, "medium_risk": eval.MediumRisk, "high_risk": eval.HighRisk} {
		if obj.IsNull() || obj.IsUnknown() {
			continue
		}
		var tier riskTierModel
		resp.Diagnostics.Append(obj.As(ctx, &tier, basetypesObjectAsOptions)...)
		if resp.Diagnostics.HasError() || tier.Action.IsUnknown() || tier.AccessRequestRuleName.IsUnknown() {
			continue
		}
		named := !tier.AccessRequestRuleName.IsNull() && tier.AccessRequestRuleName.ValueString() != ""
		switch {
		case tier.Action.ValueString() == "require_access_request" && !named:
			resp.Diagnostics.AddAttributeError(path.Root("risk_evaluation").AtName(level).AtName("access_request_rule_name"),
				"Access Request Rule Required",
				fmt.Sprintf("%s: action require_access_request needs access_request_rule_name, the rule that supplies the approver groups.", level))
		case tier.Action.ValueString() != "require_access_request" && named:
			resp.Diagnostics.AddAttributeError(path.Root("risk_evaluation").AtName(level).AtName("access_request_rule_name"),
				"Access Request Rule Not Allowed",
				fmt.Sprintf("%s: access_request_rule_name is only allowed when action is require_access_request.", level))
		}
	}
}

// analyzerSidecarSpec is the part of a sidecar_spec the plan can check before
// the gateway does.
type analyzerSidecarSpec struct {
	Trigger      json.RawMessage `json:"trigger"`
	HighRisk     string          `json:"high"`
	MediumRisk   string          `json:"medium"`
	LowRisk      string          `json:"low"`
	ApprovalRule string          `json:"approval_rule"`
}

// holds reports whether a risk level keeps a statement for a reviewer, in
// either of the sidecar's spellings.
func (s analyzerSidecarSpec) holds() bool {
	for _, action := range []string{s.HighRisk, s.MediumRisk, s.LowRisk} {
		if action == "require_review" || action == "require_approval" {
			return true
		}
	}
	return false
}

// validateAnalyzerSidecarSpec refuses at plan what the gateway refuses or
// ignores at apply.
//
// reviewers_groups and the TTLs drive the access request rule the gateway
// keeps beside an analyzer rule, and it keeps one only while sidecar_spec
// holds statements and approval_rule names this rule. Otherwise it drops the
// values and the apply ends in an inconsistent result.
func validateAnalyzerSidecarSpec(data analyzerRuleResourceModel, diags *diag.Diagnostics) {
	if data.SidecarSpec.IsUnknown() || data.Name.IsUnknown() {
		return
	}
	var spec analyzerSidecarSpec
	if !data.SidecarSpec.IsNull() {
		if err := json.Unmarshal([]byte(data.SidecarSpec.ValueString()), &spec); err != nil {
			// The attribute's type already refuses a document that does not
			// parse; anything else is a shape the gateway reports.
			return
		}
	}
	if len(spec.Trigger) > 0 && spec.Trigger[0] == '"' {
		diags.AddAttributeError(path.Root("sidecar_spec"), "Invalid Analyzer Trigger",
			"sidecar_spec.trigger is an object that narrows which statements are classified, such as "+
				"{ operations = [\"update\", \"delete\"] }, not a string. Leave it out to classify every statement.")
	}
	if spec.holds() && spec.ApprovalRule == "" {
		diags.AddAttributeError(path.Root("sidecar_spec"), "Approval Rule Required",
			"sidecar_spec holds statements for review (require_review) and names no approval_rule. "+
				"Set approval_rule to this rule's name to have the gateway manage the approval rule from "+
				"reviewers_groups, pending_ttl_sec and approval_ttl_sec, or to the name of a hoop_access_request_rule.")
	}
	ownRule := spec.holds() && spec.ApprovalRule == data.Name.ValueString()
	for _, setting := range []struct {
		name  string
		value attr.Value
	}{
		{"reviewers_groups", data.ReviewersGroups},
		{"pending_ttl_sec", data.PendingTTLSec},
		{"approval_ttl_sec", data.ApprovalTTLSec},
	} {
		name := setting.name
		if setting.value.IsNull() || setting.value.IsUnknown() || ownRule {
			continue
		}
		reason := "sidecar_spec holds no statements for review"
		if spec.holds() && spec.ApprovalRule != "" {
			reason = fmt.Sprintf("sidecar_spec names the approval rule %q, which decides who reviews and for how long", spec.ApprovalRule)
		}
		diags.AddAttributeError(path.Root(name), "Review Setting Has No Effect",
			fmt.Sprintf("%s only applies to the approval rule the gateway manages under this rule's name, and %s. "+
				"Set it on that hoop_access_request_rule, or set sidecar_spec.approval_rule to %q.",
				name, reason, data.Name.ValueString()))
	}
}

func toApiRiskTier(ctx context.Context, obj types.Object) (*hoop.AnalyzerRiskTier, diag.Diagnostics) {
	var tier riskTierModel
	diags := obj.As(ctx, &tier, basetypesObjectAsOptions)
	if diags.HasError() {
		return nil, diags
	}
	out := &hoop.AnalyzerRiskTier{Action: tier.Action.ValueString()}
	if !tier.AccessRequestRuleName.IsNull() && !tier.AccessRequestRuleName.IsUnknown() {
		name := tier.AccessRequestRuleName.ValueString()
		out.AccessRequestRuleName = &name
	}
	return out, diags
}

func fromApiRiskTier(tier *hoop.AnalyzerRiskTier, legacyAction string) types.Object {
	action := legacyAction
	name := types.StringNull()
	if tier != nil {
		action = tier.Action
		if tier.AccessRequestRuleName != nil && *tier.AccessRequestRuleName != "" {
			name = types.StringValue(*tier.AccessRequestRuleName)
		}
	}
	return types.ObjectValueMust(riskTierObjectType.AttrTypes, map[string]attr.Value{
		"action":                   types.StringValue(action),
		"access_request_rule_name": name,
	})
}

// toApiTTL reads an optional review limit. A null plan over a declared prior
// sends 0, which clears the limit; over a null prior it omits the key.
func toApiTTL(plan, prior types.Int64) *int {
	if plan.IsNull() || plan.IsUnknown() {
		if prior.IsNull() {
			return nil
		}
		zero := 0
		return &zero
	}
	v := int(plan.ValueInt64())
	return &v
}

func fromApiTTL(prior types.Int64, value *int) types.Int64 {
	if value == nil || *value == 0 {
		return types.Int64Null()
	}
	if prior.IsNull() {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*value))
}

func (m *analyzerRuleResourceModel) toApi(ctx context.Context, prior *analyzerRuleResourceModel) (hoop.AnalyzerRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	rule := hoop.AnalyzerRule{
		ID:      m.ID.ValueString(),
		Name:    m.Name.ValueString(),
		Agentic: m.Agentic.ValueBool(),
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		s := m.Description.ValueString()
		rule.Description = &s
	}
	if !m.CustomPrompt.IsNull() && !m.CustomPrompt.IsUnknown() {
		s := m.CustomPrompt.ValueString()
		rule.CustomPrompt = &s
	}
	var d diag.Diagnostics
	rule.ConnectionNames, d = toApiStringList(ctx, m.ConnectionNames)
	diags.Append(d...)

	var eval riskEvaluationModel
	diags.Append(m.RiskEvaluation.As(ctx, &eval, basetypesObjectAsOptions)...)
	if diags.HasError() {
		return rule, diags
	}
	rule.RiskEvaluation.LowRisk, d = toApiRiskTier(ctx, eval.LowRisk)
	diags.Append(d...)
	rule.RiskEvaluation.MediumRisk, d = toApiRiskTier(ctx, eval.MediumRisk)
	diags.Append(d...)
	rule.RiskEvaluation.HighRisk, d = toApiRiskTier(ctx, eval.HighRisk)
	diags.Append(d...)

	priorModel := analyzerRuleResourceModel{
		SidecarSpec:     jsontypes.NewNormalizedNull(),
		SidecarTargets:  types.ListNull(sidecarTargetObjectType),
		ReviewersGroups: types.ListNull(types.StringType),
		PendingTTLSec:   types.Int64Null(),
		ApprovalTTLSec:  types.Int64Null(),
	}
	if prior != nil {
		priorModel = *prior
	}
	rule.SidecarSpec = toApiJSON(m.SidecarSpec, priorModel.SidecarSpec)
	rule.SidecarTargets, d = toApiSidecarTargets(ctx, m.SidecarTargets, priorModel.SidecarTargets)
	diags.Append(d...)
	rule.ReviewersGroups, d = toApiOwnedStringList(ctx, m.ReviewersGroups, priorModel.ReviewersGroups)
	diags.Append(d...)
	rule.PendingTTLSec = toApiTTL(m.PendingTTLSec, priorModel.PendingTTLSec)
	rule.ApprovalTTLSec = toApiTTL(m.ApprovalTTLSec, priorModel.ApprovalTTLSec)
	return rule, diags
}

func (m *analyzerRuleResourceModel) fromApi(ctx context.Context, rule *hoop.AnalyzerRule) diag.Diagnostics {
	var diags diag.Diagnostics
	var d diag.Diagnostics
	m.ID = types.StringValue(rule.ID)
	m.Name = types.StringValue(rule.Name)
	m.Description = fromApiString(m.Description, derefString(rule.Description))
	m.CustomPrompt = fromApiString(m.CustomPrompt, derefString(rule.CustomPrompt))
	m.Agentic = types.BoolValue(rule.Agentic)
	m.ConnectionNames, d = fromApiStringList(ctx, m.ConnectionNames, rule.ConnectionNames)
	diags.Append(d...)

	eval := rule.RiskEvaluation
	m.RiskEvaluation = types.ObjectValueMust(riskEvaluationObjectType.AttrTypes, map[string]attr.Value{
		"low_risk":    fromApiRiskTier(eval.LowRisk, eval.LowRiskAction),
		"medium_risk": fromApiRiskTier(eval.MediumRisk, eval.MediumRiskAction),
		"high_risk":   fromApiRiskTier(eval.HighRisk, eval.HighRiskAction),
	})

	m.SidecarSpec = fromApiJSON(m.SidecarSpec, rule.SidecarSpec)
	m.SidecarTargets, d = fromApiSidecarTargets(ctx, m.SidecarTargets, hoop.TargetsOf(rule.SidecarTargets))
	diags.Append(d...)
	m.ReviewersGroups, d = fromApiOwnedStringList(ctx, m.ReviewersGroups, rule.ReviewersGroups)
	diags.Append(d...)
	m.PendingTTLSec = fromApiTTL(m.PendingTTLSec, rule.PendingTTLSec)
	m.ApprovalTTLSec = fromApiTTL(m.ApprovalTTLSec, rule.ApprovalTTLSec)
	return diags
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *analyzerRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan analyzerRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, diags := plan.toApi(ctx, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.CreateAnalyzerRule(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating AI Session Analyzer Rule",
			fmt.Sprintf("Failed to create analyzer rule %q: %v", plan.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(plan.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *analyzerRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state analyzerRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.GetAnalyzerRule(state.Name.ValueString())
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading AI Session Analyzer Rule",
			fmt.Sprintf("Failed reading analyzer rule %q: %v", state.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(state.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *analyzerRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state analyzerRuleResourceModel
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
	rule, err := r.client.UpdateAnalyzerRule(payload)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating AI Session Analyzer Rule",
			fmt.Sprintf("Failed to update analyzer rule %q: %v", plan.Name.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(plan.fromApi(ctx, rule)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *analyzerRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state analyzerRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteAnalyzerRule(state.Name.ValueString())
	if err != nil && !hoop.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting AI Session Analyzer Rule",
			fmt.Sprintf("Failed to delete analyzer rule %q: %v", state.Name.ValueString(), err))
	}
}

// ImportState takes the rule name, which is how the gateway addresses it.
// ModifyPlan refuses a write that would leave a bound rule without a sidecar
// spec, and warns when a declared sidecar_targets is removed, since the write
// that follows unbinds the rule from every sidecar it named.
func (r *analyzerRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan analyzerRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if req.State.Raw.IsNull() {
		refuseBoundRuleWithoutSpec(plan.SidecarSpec, jsontypes.NewNormalizedNull(), true, plan.SidecarTargets, &resp.Diagnostics)
		return
	}
	var state analyzerRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	refuseBoundRuleWithoutSpec(plan.SidecarSpec, state.SidecarSpec, false, plan.SidecarTargets, &resp.Diagnostics)
	warnOnUnbind(ctx, "hoop_ai_session_analyzer_rule", state.Name.ValueString(), plan.SidecarTargets, state.SidecarTargets, &resp.Diagnostics)
}

func (r *analyzerRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *analyzerRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
