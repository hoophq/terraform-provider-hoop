// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// basetypesObjectAsOptions is the strict form: a null or unknown object is an
// error rather than a zero struct, so a missing nested block fails loudly.
var basetypesObjectAsOptions = basetypes.ObjectAsOptions{}

// The three rule resources bind to sidecars the same way, and the gateway
// reads their binding fields the same way: a key left out of the request keeps
// what is stored, an empty value clears it. The helpers here give every rule
// resource one reading of "declared" and "not declared".
//
// An attribute the configuration never declares stays null in state and never
// reaches the request, so a rule bound from the control plane UI keeps its
// bindings under Terraform. Declaring the attribute makes Terraform the owner:
// the value goes out on every write, and removing the attribute from the
// configuration sends the empty value that clears it.

// sidecarTargetModel is one sidecar_targets entry.
type sidecarTargetModel struct {
	SidecarID    types.String `tfsdk:"sidecar_id"`
	ListenerName types.String `tfsdk:"listener_name"`
}

var sidecarTargetObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"sidecar_id":    types.StringType,
		"listener_name": types.StringType,
	},
}

const sidecarTargetsDescription = "The sidecar listeners the control plane distributes this rule to. " +
	"Each entry names a sidecar and one of its listeners. " +
	"Leave the attribute out to keep bindings made outside Terraform. " +
	"Set it to `[]` to unbind the rule from every sidecar."

const sidecarSpecDescription = "The rule in the sidecar's own vocabulary, as a JSON object. " +
	"The control plane places this block on each bound listener. " +
	"Leave the attribute out to keep the block stored on the gateway; removing a declared value clears it."

func sidecarTargetsAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Description: sidecarTargetsDescription,
		Optional:    true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"sidecar_id": schema.StringAttribute{
					Description: "The sidecar that enforces the rule.",
					Required:    true,
				},
				"listener_name": schema.StringAttribute{
					Description: "The listener on that sidecar. Required: the gateway binds rules per listener, " +
						"because the listener's protocol decides which rules it can run.",
					Required:   true,
					Validators: NonEmptyStringValidator,
				},
			},
		},
	}
}

func sidecarSpecAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Description: sidecarSpecDescription,
		CustomType:  jsontypes.NormalizedType{},
		Optional:    true,
	}
}

func jsonAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Description: description,
		CustomType:  jsontypes.NormalizedType{},
		Optional:    true,
	}
}

// toApiSidecarTargets reads the planned targets. A null plan with a null prior
// state omits the key. A null plan over a declared prior state is the operator
// removing the attribute, which unbinds the rule everywhere.
func toApiSidecarTargets(ctx context.Context, plan, prior types.List) (hoop.SidecarTargets, diag.Diagnostics) {
	if plan.IsNull() || plan.IsUnknown() {
		if prior.IsNull() {
			return nil, nil
		}
		return hoop.NewSidecarTargets(nil), nil
	}
	var items []sidecarTargetModel
	diags := plan.ElementsAs(ctx, &items, false)
	if diags.HasError() {
		return nil, diags
	}
	targets := make([]hoop.SidecarRuleTarget, 0, len(items))
	for _, item := range items {
		targets = append(targets, hoop.SidecarRuleTarget{
			SidecarID:    item.SidecarID.ValueString(),
			ListenerName: item.ListenerName.ValueString(),
		})
	}
	return hoop.NewSidecarTargets(targets), diags
}

// fromApiSidecarTargets writes the gateway's bindings into state. A prior null
// stays null: the operator never declared the attribute, and the bindings are
// not Terraform's to report.
//
// The gateway answers bindings sorted by sidecar id and listener, not in the
// order they were sent, so they are put back in the prior's order: the plan's
// after a write, the state's on a read. Bindings the prior does not name follow
// in the gateway's order.
func fromApiSidecarTargets(ctx context.Context, prior types.List, targets []hoop.SidecarRuleTarget) (types.List, diag.Diagnostics) {
	if prior.IsNull() {
		return types.ListNull(sidecarTargetObjectType), nil
	}
	var priorItems []sidecarTargetModel
	if !prior.IsUnknown() {
		if diags := prior.ElementsAs(ctx, &priorItems, false); diags.HasError() {
			return types.ListNull(sidecarTargetObjectType), diags
		}
	}
	targets = reorderTargets(priorItems, targets)
	items := make([]sidecarTargetModel, 0, len(targets))
	for _, t := range targets {
		items = append(items, sidecarTargetModel{
			SidecarID:    types.StringValue(t.SidecarID),
			ListenerName: types.StringValue(t.ListenerName),
		})
	}
	return types.ListValueFrom(ctx, sidecarTargetObjectType, items)
}

// reorderTargets returns targets in the order prior names them, followed by
// the ones prior does not name, in their own order. An unknown prior element
// matches nothing.
func reorderTargets(prior []sidecarTargetModel, targets []hoop.SidecarRuleTarget) []hoop.SidecarRuleTarget {
	remaining := make(map[hoop.SidecarRuleTarget]int, len(targets))
	for _, t := range targets {
		remaining[t]++
	}
	ordered := make([]hoop.SidecarRuleTarget, 0, len(targets))
	for _, p := range prior {
		t := hoop.SidecarRuleTarget{SidecarID: p.SidecarID.ValueString(), ListenerName: p.ListenerName.ValueString()}
		if remaining[t] > 0 {
			ordered = append(ordered, t)
			remaining[t]--
		}
	}
	for _, t := range targets {
		if remaining[t] > 0 {
			ordered = append(ordered, t)
			remaining[t]--
		}
	}
	return ordered
}

// toApiJSON reads a JSON attribute for a request. A null plan with a null prior
// omits the key. A null plan over a declared prior sends a JSON null, which the
// gateway reads as "clear it".
func toApiJSON(plan, prior jsontypes.Normalized) json.RawMessage {
	if plan.IsNull() || plan.IsUnknown() {
		if prior.IsNull() {
			return nil
		}
		return json.RawMessage("null")
	}
	return json.RawMessage(plan.ValueString())
}

// toApiJSONOr is toApiJSON for a field the gateway requires: a null plan sends
// fallback instead of omitting the key.
func toApiJSONOr(plan jsontypes.Normalized, fallback string) json.RawMessage {
	if plan.IsNull() || plan.IsUnknown() {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(plan.ValueString())
}

// fromApiJSON writes a JSON document into state. A prior null stays null: the
// operator never declared the attribute, so the document is not Terraform's to
// report.
func fromApiJSON(prior jsontypes.Normalized, raw json.RawMessage) jsontypes.Normalized {
	if len(raw) == 0 || string(raw) == "null" {
		return jsontypes.NewNormalizedNull()
	}
	if prior.IsNull() {
		return jsontypes.NewNormalizedNull()
	}
	return jsontypes.NewNormalizedValue(string(raw))
}

// jsonencodeForm rewrites a JSON document the way Terraform's jsonencode
// writes one: keys sorted, no whitespace, numbers as written. A value read
// with no prior to compare against, as after an import, lands in state in
// that form, so a configuration that builds it with jsonencode plans no
// change. A document that does not parse is returned unchanged.
func jsonencodeForm(raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

// fromApiJSONOr is fromApiJSON for a required field with a fallback. The state
// stays null while the gateway holds the fallback and nothing declared the
// attribute, so a resource that never set the field shows no diff.
func fromApiJSONOr(ctx context.Context, prior jsontypes.Normalized, raw json.RawMessage, fallback string) (jsontypes.Normalized, diag.Diagnostics) {
	if prior.IsNull() {
		if len(raw) == 0 || string(raw) == "null" {
			return jsontypes.NewNormalizedNull(), nil
		}
		equal, diags := jsontypes.NewNormalizedValue(fallback).StringSemanticEquals(ctx, jsontypes.NewNormalizedValue(string(raw)))
		if diags.HasError() {
			return jsontypes.NewNormalizedNull(), diags
		}
		if equal {
			return jsontypes.NewNormalizedNull(), nil
		}
	}
	if len(raw) == 0 || string(raw) == "null" {
		return jsontypes.NewNormalizedValue(fallback), nil
	}
	return jsontypes.NewNormalizedValue(string(raw)), nil
}

// toApiStringList reads an optional list of strings. Null sends an empty list,
// which is what the gateway stores for "none".
func toApiStringList(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	out := []string{}
	if list.IsNull() || list.IsUnknown() {
		return out, nil
	}
	diags := list.ElementsAs(ctx, &out, false)
	return out, diags
}

// fromApiStringList writes a list of strings into state. An empty answer over
// a null prior stays null, so an attribute left out of the configuration does
// not come back as [].
func fromApiStringList(ctx context.Context, prior types.List, values []string) (types.List, diag.Diagnostics) {
	if prior.IsNull() && len(values) == 0 {
		return types.ListNull(types.StringType), nil
	}
	if values == nil {
		values = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, values)
}

// toApiStringSet reads an optional set of strings. Null sends an empty list,
// which is what the gateway stores for "none".
func toApiStringSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	out := []string{}
	if set.IsNull() || set.IsUnknown() {
		return out, nil
	}
	diags := set.ElementsAs(ctx, &out, false)
	return out, diags
}

// fromApiOwnedStringSet writes a set of links into state, null while
// undeclared: the links may have been made from the other side, such as on
// the attribute itself, and reading them in would plan their removal. A set,
// because the gateway answers some lists sorted whatever order they were
// written in (attribute names), and a list would plan that reorder forever.
func fromApiOwnedStringSet(ctx context.Context, prior types.Set, values []string) (types.Set, diag.Diagnostics) {
	if prior.IsNull() {
		return types.SetNull(types.StringType), nil
	}
	if values == nil {
		values = []string{}
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

// orEmpty returns values, or an empty list for nil, so a links field written
// back from a read goes out as [] rather than null.
func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// toApiOwnedStringList reads a list the gateway keeps when the key is absent.
// A null plan over a null prior omits the key; over a declared prior it sends
// [] to clear.
func toApiOwnedStringList(ctx context.Context, plan, prior types.List) (*[]string, diag.Diagnostics) {
	if plan.IsNull() || plan.IsUnknown() {
		if prior.IsNull() {
			return nil, nil
		}
		empty := []string{}
		return &empty, nil
	}
	var out []string
	diags := plan.ElementsAs(ctx, &out, false)
	if out == nil {
		out = []string{}
	}
	return &out, diags
}

// fromApiOwnedStringList writes a kept list into state, null while undeclared.
func fromApiOwnedStringList(ctx context.Context, prior types.List, values *[]string) (types.List, diag.Diagnostics) {
	if prior.IsNull() {
		return types.ListNull(types.StringType), nil
	}
	out := []string{}
	if values != nil && *values != nil {
		out = *values
	}
	return types.ListValueFrom(ctx, types.StringType, out)
}

// fromApiString writes an optional string into state. An empty answer over a
// null prior stays null, so an attribute left out of the configuration does
// not come back as "".
func fromApiString(prior types.String, value string) types.String {
	if prior.IsNull() && value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

// refuseBoundRuleWithoutSpec is the gateway's ValidateSidecarRuleSpec at plan:
// a rule bound to a listener has to say what it does there, and the gateway
// refuses the write that would leave one without a spec. That is a create
// with declared targets and no sidecar_spec, or a declared sidecar_spec
// removed while declared targets stay, which sends the null that clears it.
// A spec never declared on an existing rule is not sent, so the gateway keeps
// what it stores, possibly set in the UI; that case is left to the gateway.
func refuseBoundRuleWithoutSpec(plan, prior jsontypes.Normalized, creating bool, targets types.List, diags *diag.Diagnostics) {
	if !plan.IsNull() || targets.IsNull() || targets.IsUnknown() || len(targets.Elements()) == 0 {
		return
	}
	if !creating && prior.IsNull() {
		return
	}
	diags.AddAttributeError(path.Root("sidecar_spec"), "Bound Rule Needs a Sidecar Spec",
		fmt.Sprintf("sidecar_targets binds this rule to %d listener(s), and sidecar_spec is not set. The gateway "+
			"refuses a bound rule that does not say what it does on its listeners. Set sidecar_spec, or set "+
			"sidecar_targets to [] to unbind the rule in the same change.", len(targets.Elements())))
}

// warnOnUnbind adds a plan warning when a declared sidecar_targets is removed
// from the configuration. The plan already shows the attribute going away;
// this says what that does, because the write sends [] and the gateway
// unbinds the rule from every listener it named.
func warnOnUnbind(ctx context.Context, resourceType, name string, plan, prior types.List, diags *diag.Diagnostics) {
	if !plan.IsNull() || prior.IsNull() || prior.IsUnknown() {
		return
	}
	var targets []sidecarTargetModel
	diags.Append(prior.ElementsAs(ctx, &targets, false)...)
	if len(targets) == 0 {
		return
	}
	sidecars := map[string]bool{}
	for _, t := range targets {
		sidecars[t.SidecarID.ValueString()] = true
	}
	diags.AddAttributeWarning(path.Root("sidecar_targets"), "Rule Will Be Unbound From Its Sidecars",
		fmt.Sprintf("Removing sidecar_targets from %s %q unbinds it from %d listener(s) on %d sidecar(s). "+
			"The sidecars stop enforcing the rule on their next heartbeat. "+
			"Leave the attribute out of the configuration from the start to keep bindings Terraform does not manage.",
			resourceType, name, len(targets), len(sidecars)))
}
