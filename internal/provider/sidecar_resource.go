// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                   = &sidecarResource{}
	_ resource.ResourceWithImportState    = &sidecarResource{}
	_ resource.ResourceWithValidateConfig = &sidecarResource{}
	_ resource.ResourceWithModifyPlan     = &sidecarResource{}
)

func NewSidecarResource() resource.Resource {
	return &sidecarResource{}
}

type sidecarResourceModel struct {
	ID            types.String       `tfsdk:"id"`
	Name          types.String       `tfsdk:"name"`
	Configuration sidecarConfigValue `tfsdk:"configuration"`
	LoadFromDisk  types.Bool         `tfsdk:"load_from_disk"`
	SlackChannels types.Map          `tfsdk:"slack_channels"`
	Token         types.String       `tfsdk:"token"`
	BoundRules    types.List         `tfsdk:"bound_rules"`

	Version         types.String `tfsdk:"version"`
	LastSeenAt      types.String `tfsdk:"last_seen_at"`
	ConfigState     types.String `tfsdk:"config_state"`
	LastError       types.String `tfsdk:"last_error"`
	IdentityIssuer  types.String `tfsdk:"identity_issuer"`
	IdentitySubject types.String `tfsdk:"identity_subject"`
}

// sidecarRuleBindingModel is one bound_rules entry.
type sidecarRuleBindingModel struct {
	Kind         types.String `tfsdk:"kind"`
	Name         types.String `tfsdk:"name"`
	ListenerName types.String `tfsdk:"listener_name"`
}

var sidecarRuleBindingObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"kind":          types.StringType,
		"name":          types.StringType,
		"listener_name": types.StringType,
	},
}

// boundRulesValue is the bindings the gateway reported, or null when it could
// not read them. Null rather than empty, because the plan guard reads an
// empty list as "nothing bound" and must not do so for "unknown".
func boundRulesValue(ctx context.Context, sc *hoop.Sidecar) (types.List, diag.Diagnostics) {
	if sc.BoundRulesUnavailable {
		return types.ListNull(sidecarRuleBindingObjectType), nil
	}
	items := make([]sidecarRuleBindingModel, 0, len(sc.BoundRules))
	for _, b := range sc.BoundRules {
		items = append(items, sidecarRuleBindingModel{
			Kind:         types.StringValue(b.Kind),
			Name:         types.StringValue(b.Name),
			ListenerName: types.StringValue(b.ListenerName),
		})
	}
	return types.ListValueFrom(ctx, sidecarRuleBindingObjectType, items)
}

type sidecarResource struct {
	client *hoop.Client
}

// loadFromDiskKey is the one configuration key with its own attribute. The
// gateway refuses a PUT that changes it and wants a PATCH instead, so the
// resource has to see the change coming rather than find it inside a JSON
// string.
const loadFromDiskKey = "load_from_disk"

var slackChannelsElementType = types.ListType{ElemType: types.StringType}

func (r *sidecarResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sidecar"
}

func (r *sidecarResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers a sidecar with the control plane and owns the configuration it serves. " +
			"The sidecar fetches that configuration on every heartbeat. Pass `token` to it as `HOOP_SIDECAR_TOKEN` " +
			"next to `HOOP_CONTROL_PLANE_URL`, or `controlPlane.token` in the helm chart. " +
			"Rules reach the sidecar through the `sidecar_targets` of `hoop_guardrail`, `hoop_datamasking_rules` " +
			"and `hoop_ai_session_analyzer_rule`; `bound_rules` shows what reached it. " +
			"A fleet whose pods identify themselves with a platform token uses `hoop_sidecar_service_account` and " +
			"`data.hoop_sidecar` instead; the guide \"Managing a sidecar fleet with Terraform\" walks through both. " +
			"Needs an Enterprise license, checked at plan.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the sidecar. Rule resources reference it in `sidecar_targets`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The sidecar name, unique in the organization. Changing it replaces the sidecar and its token. " +
					"Destroying a sidecar records its name, and an identity that maps to that name is refused until an " +
					"admin clears it with `DELETE /api/sidecar-deleted-names/{name}`.",
				Required:   true,
				Validators: NonEmptyStringValidator,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"configuration": schema.StringAttribute{
				Description: "The sidecar configuration document as a JSON object, in the schema of the sidecar " +
					"config file: `listeners`, `admin`, `audit` and so on. `jsonencode(yamldecode(file(\"sidecar.yaml\")))` " +
					"converts an existing file. The gateway drops zero values from the stored document; the plan " +
					"compares past them, so `\"\"`, `0`, `false` and empty blocks are harmless. Omit the attribute to " +
					"store an empty document; the sidecar then refuses to start until one is set. " +
					"Do not put `load_from_disk` here; use the attribute.",
				CustomType: sidecarConfigType{},
				Optional:   true,
			},
			"load_from_disk": schema.BoolAttribute{
				Description: "When true the sidecar runs its own config file and the control plane serves only the " +
					"license. A switch in either direction detaches every rule bound to the sidecar: rules imported " +
					"from its file that nothing else uses are deleted, the rest unbound. The plan warns, the apply " +
					"lists them. Switching it off also clears the stored document before `configuration` is written.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"slack_channels": schema.MapAttribute{
				Description: "Slack channel ids per listener name, where that listener's reviews are posted. " +
					"Every key must be a listener of `configuration`, with at least one channel. A listener left " +
					"out uses the organization's default channel. Leave the attribute out to keep channels set " +
					"outside Terraform; removing a declared value clears every listener it named.",
				ElementType: slackChannelsElementType,
				Optional:    true,
				Validators: []validator.Map{
					mapvalidator.ValueListsAre(listvalidator.SizeAtLeast(1)),
				},
			},
			"token": schema.StringAttribute{
				Description: "The token the sidecar sends in `hoop-sidecar-token`. The gateway shows it once, at " +
					"creation, and stores a hash, so it lives only in this state. Empty on an imported sidecar.",
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.StringAttribute{
				Description: "The sidecar binary version reported at the last handshake.",
				Computed:    true,
			},
			"last_seen_at": schema.StringAttribute{
				Description: "When the sidecar last handshook, RFC 3339. Empty until it does.",
				Computed:    true,
			},
			"config_state": schema.StringAttribute{
				Description: "What the sidecar runs relative to the stored configuration: `applied`, `applying`, " +
					"`not_applied`, `refused`, `not_served`, `restart` or `unknown`. Empty before the first handshake " +
					"and under `load_from_disk`.",
				Computed: true,
			},
			"last_error": schema.StringAttribute{
				Description: "Why the sidecar refused the last configuration, or why the control plane did not serve it.",
				Computed:    true,
			},
			"identity_issuer": schema.StringAttribute{
				Description: "The issuer of the service account identity bound to this sidecar, if one reached it.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identity_subject": schema.StringAttribute{
				Description: "The subject of the service account identity bound to this sidecar, if one reached it.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bound_rules": schema.ListNestedAttribute{
				Description: "The rules the control plane folds into this sidecar's configuration, read from the " +
					"gateway. The rule resources own them through `sidecar_targets`; this is a view. The plan " +
					"refuses a `configuration` change that removes, renames or re-protocols a listener named here.",
				Computed: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"kind": schema.StringAttribute{
							Description: "`guardrail`, `datamasking` or `analyzer`.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "The rule name.",
							Computed:    true,
						},
						"listener_name": schema.StringAttribute{
							Description: "The listener the rule is bound to.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

// ValidateConfig refuses at plan what the gateway refuses at write: a
// load_from_disk key inside the document (the attribute routes it through
// PATCH), and a slack_channels key that names no listener of the document.
func (r *sidecarResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data sidecarResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Configuration.IsUnknown() {
		return
	}
	listeners := listenerProtocols(data.Configuration)
	if !data.Configuration.IsNull() {
		var doc map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &doc); err != nil {
			// The type's own validation reports the syntax error.
			return
		}
		if _, ok := doc[loadFromDiskKey]; ok {
			resp.Diagnostics.AddAttributeError(path.Root("configuration"),
				"load_from_disk Belongs in Its Own Attribute",
				"Remove the load_from_disk key from configuration and set the load_from_disk attribute instead. "+
					"The gateway only accepts a change to it through a separate call, which the attribute drives.")
		}
	}
	if data.SlackChannels.IsNull() || data.SlackChannels.IsUnknown() {
		return
	}
	for name := range data.SlackChannels.Elements() {
		if _, ok := listeners[name]; !ok {
			resp.Diagnostics.AddAttributeError(path.Root("slack_channels").AtMapKey(name),
				"Slack Channels Name No Listener",
				fmt.Sprintf("slack_channels names listener %q, and configuration has no listener with that name. "+
					"The gateway refuses channels for a listener it does not have.", name))
		}
	}
}

// ModifyPlan catches two failures before the apply starts.
//
// The license: every sidecar route needs an Enterprise license, and the
// gateway answers 403 from the middle of an apply. The same predicate is
// readable from /serverinfo, so the plan asks first.
//
// The listeners: a bound rule names its listener, and the gateway refuses a
// document that removes, renames or re-protocols a listener a rule is bound
// to. Terraform orders the rule change after the sidecar it depends on, so
// the apply would fail on the sidecar and never reach the rule. The bindings
// came with the last read, so the plan can say which rules stand in the way.
func (r *sidecarResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	refuseWithoutEnterpriseLicense(r.client, "hoop_sidecar", &resp.Diagnostics)
	if req.State.Raw.IsNull() {
		return
	}
	var plan, state sidecarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || plan.Configuration.IsUnknown() {
		return
	}
	// The owner switch detaches every rule bound to this sidecar, in either
	// direction, so after the apply bound_rules is whatever the gateway kept:
	// unknown now, and no listener is protected by a binding the switch
	// removes. Say so while the operator can still stop, and skip the guard.
	flipping := plan.LoadFromDisk.IsUnknown() || plan.LoadFromDisk.ValueBool() != state.LoadFromDisk.ValueBool()
	if flipping {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("bound_rules"), types.ListUnknown(sidecarRuleBindingObjectType))...)
	}
	if state.BoundRules.IsNull() {
		if !state.Configuration.IsNull() && !flipping && !state.Configuration.Equal(plan.Configuration) {
			resp.Diagnostics.AddAttributeWarning(path.Root("configuration"), "Sidecar Bindings Unknown",
				"The rules bound to this sidecar could not be read, so a listener this change removes or renames "+
					"is not checked here. The gateway refuses the write if a rule is bound to it.")
		}
		return
	}
	var bound []sidecarRuleBindingModel
	resp.Diagnostics.Append(state.BoundRules.ElementsAs(ctx, &bound, false)...)
	if resp.Diagnostics.HasError() || len(bound) == 0 {
		return
	}
	if flipping {
		resp.Diagnostics.AddAttributeWarning(path.Root("load_from_disk"), "Owner Switch Detaches Bound Rules",
			fmt.Sprintf("Changing load_from_disk unbinds the %d rule(s) bound to sidecar %q. Rules imported from this "+
				"sidecar's file that nothing else uses are deleted. Rules managed by Terraform are recreated or rebound "+
				"on the next apply; the sidecar runs without them until then.", len(bound), state.Name.ValueString()))
		return
	}
	before := listenerProtocols(state.Configuration)
	after := listenerProtocols(plan.Configuration)
	var broken []string
	for _, b := range bound {
		name := b.ListenerName.ValueString()
		// The gateway refuses a binding with no listener today
		// (services.ValidateSidecarRuleTargets). Should a later version
		// distribute whole-sidecar bindings, they name no listener to break.
		if name == "" {
			continue
		}
		protocol, kept := after[name]
		switch {
		case !kept:
			broken = append(broken, fmt.Sprintf("%s %q is bound to listener %q, which this change removes or renames",
				b.Kind.ValueString(), b.Name.ValueString(), name))
		case protocol != before[name]:
			broken = append(broken, fmt.Sprintf("%s %q is bound to listener %q, which this change moves from %s to %s",
				b.Kind.ValueString(), b.Name.ValueString(), name, before[name], protocol))
		}
	}
	if len(broken) == 0 {
		return
	}
	resp.Diagnostics.AddAttributeError(path.Root("configuration"),
		"Listener Change Breaks Bound Rules",
		"The gateway refuses a configuration that removes, renames or re-protocols a listener a rule is bound to, "+
			"and Terraform applies the sidecar before the rules that depend on it.\n\n"+
			strings.Join(broken, "\n")+
			"\n\nUpdate the sidecar_targets of those rules first, in a separate apply.")
}

// listenerProtocols maps listener name to protocol for a configuration, or
// returns an empty map for a document that does not parse. The gateway
// already validated anything that reached state, so an unparsable planned
// document fails elsewhere with the right message.
func listenerProtocols(cfg sidecarConfigValue) map[string]string {
	out := map[string]string{}
	if cfg.IsNull() || cfg.IsUnknown() {
		return out
	}
	var doc struct {
		Listeners []struct {
			Name     string `json:"name"`
			Protocol string `json:"protocol"`
		} `json:"listeners"`
	}
	if err := json.Unmarshal([]byte(cfg.ValueString()), &doc); err != nil {
		return out
	}
	for _, l := range doc.Listeners {
		if l.Name != "" {
			out[l.Name] = l.Protocol
		}
	}
	return out
}

// refuseWithoutEnterpriseLicense adds a plan error when the gateway reports
// no valid Enterprise license, which is the exact condition under which it
// answers 403 to every sidecar route. A gateway that cannot be asked, or
// one too old to report a license, is left to answer at apply.
func refuseWithoutEnterpriseLicense(client *hoop.Client, resourceType string, diags *diag.Diagnostics) {
	if client == nil {
		return
	}
	check, err := client.CheckEnterpriseLicense()
	if err != nil || check.Unknown || check.Enterprise {
		return
	}
	diags.AddError("Enterprise License Required",
		fmt.Sprintf("%s needs a valid Enterprise license and the gateway reports that %s. "+
			"Add the license under Settings > License and plan again.", resourceType, check.Reason))
}

// documentForWrite builds the document the gateway stores: the planned
// configuration, or an empty object, with load_from_disk added when on.
func documentForWrite(cfg sidecarConfigValue, loadFromDisk bool) (json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	if !cfg.IsNull() && !cfg.IsUnknown() {
		if err := json.Unmarshal([]byte(cfg.ValueString()), &doc); err != nil {
			return nil, fmt.Errorf("the configuration must be a JSON object: %w", err)
		}
		// The literal null unmarshals into a nil map without an error.
		if doc == nil {
			return nil, fmt.Errorf("the configuration must be a JSON object, got null")
		}
	}
	if loadFromDisk {
		doc[loadFromDiskKey] = json.RawMessage("true")
	}
	return json.Marshal(doc)
}

// splitDocument takes the stored document apart: the configuration without
// load_from_disk, and the flag. The gateway never stores a false flag, so an
// absent key is false.
func splitDocument(raw json.RawMessage) (json.RawMessage, bool, error) {
	doc := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, false, fmt.Errorf("the gateway answered a configuration that is not a JSON object: %w", err)
		}
	}
	loadFromDisk := false
	if v, ok := doc[loadFromDiskKey]; ok {
		loadFromDisk = string(v) == "true"
		delete(doc, loadFromDiskKey)
	}
	out, err := json.Marshal(doc)
	return out, loadFromDisk, err
}

// readSidecarInto refreshes model from the gateway. prior decides which
// optional attributes the operator declared.
func (r *sidecarResource) readSidecarInto(ctx context.Context, model *sidecarResourceModel, sc *hoop.Sidecar) diag.Diagnostics {
	var diags diag.Diagnostics
	doc, loadFromDisk, err := splitDocument(sc.Configuration)
	if err != nil {
		diags.AddError("Error Reading Sidecar Configuration", err.Error())
		return diags
	}
	model.ID = types.StringValue(sc.ID)
	model.Name = types.StringValue(sc.Name)
	model.LoadFromDisk = types.BoolValue(loadFromDisk)
	// An empty document over an undeclared attribute stays null, so a sidecar
	// registered without a configuration shows no diff.
	if model.Configuration.IsNull() && string(doc) == "{}" {
		model.Configuration = newSidecarConfigNull()
	} else {
		model.Configuration = newSidecarConfigValue(string(doc))
	}
	// Bindings the gateway could not read are not "none". Keep what the last
	// read knew, so the listener guard still has something to check; with no
	// prior knowledge the attribute is null and the guard says so.
	if sc.BoundRulesUnavailable && !model.BoundRules.IsNull() && !model.BoundRules.IsUnknown() {
		diags.AddWarning("Sidecar Bindings Unavailable",
			fmt.Sprintf("The gateway could not read the rules bound to sidecar %q; the last known bindings are kept.", sc.Name))
	} else {
		var d diag.Diagnostics
		model.BoundRules, d = boundRulesValue(ctx, sc)
		diags.Append(d...)
	}
	model.Version = types.StringValue(sc.Version)
	model.LastSeenAt = types.StringValue(sc.LastSeenAt)
	model.ConfigState = types.StringValue(sc.ConfigState)
	model.LastError = types.StringValue(sc.LastError)
	model.IdentityIssuer = types.StringValue(sc.IdentityIssuer)
	model.IdentitySubject = types.StringValue(sc.IdentitySubject)
	if model.Token.IsNull() || model.Token.IsUnknown() {
		model.Token = types.StringValue("")
	}

	if model.SlackChannels.IsNull() {
		model.SlackChannels = types.MapNull(slackChannelsElementType)
		return diags
	}
	channels, err := r.client.GetSidecarSlackChannels(sc.ID)
	if err != nil {
		diags.AddError("Error Reading Sidecar Slack Channels",
			fmt.Sprintf("Failed reading the slack channels of sidecar %q: %v", sc.Name, err))
		return diags
	}
	value, d := types.MapValueFrom(ctx, slackChannelsElementType, channels)
	diags.Append(d...)
	model.SlackChannels = value
	return diags
}

// slackChannelsForWrite builds the PUT body. The gateway leaves out listeners
// the body does not name, so every listener the prior state named and the
// plan dropped goes out with no channels, which clears it. When the current
// listeners are known, a dropped listener that no longer exists is left out:
// the gateway already pruned its channels and refuses an entry for it.
func slackChannelsForWrite(ctx context.Context, plan, prior types.Map, listeners map[string]string) (map[string][]string, bool, diag.Diagnostics) {
	if plan.IsNull() || plan.IsUnknown() {
		if prior.IsNull() {
			return nil, false, nil
		}
	}
	out := map[string][]string{}
	var diags diag.Diagnostics
	if !plan.IsNull() && !plan.IsUnknown() {
		diags.Append(plan.ElementsAs(ctx, &out, false)...)
		if diags.HasError() {
			return nil, false, diags
		}
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		previous := map[string][]string{}
		diags.Append(prior.ElementsAs(ctx, &previous, false)...)
		if diags.HasError() {
			return nil, false, diags
		}
		for name := range previous {
			if _, kept := out[name]; kept {
				continue
			}
			if _, exists := listeners[name]; listeners != nil && !exists {
				continue
			}
			out[name] = []string{}
		}
	}
	return out, true, diags
}

func (r *sidecarResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sidecarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	doc, err := documentForWrite(plan.Configuration, plan.LoadFromDisk.ValueBool())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid Sidecar Configuration", err.Error())
		return
	}
	created, err := r.client.CreateSidecar(plan.Name.ValueString(), doc)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Sidecar",
			fmt.Sprintf("Failed to create sidecar %q: %v", plan.Name.ValueString(), err))
		return
	}
	plan.Token = types.StringValue(created.Token)

	channels, write, diags := slackChannelsForWrite(ctx, plan.SlackChannels, types.MapNull(slackChannelsElementType), nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if write {
		if _, err := r.client.SetSidecarSlackChannels(created.ID, channels); err != nil {
			// A sidecar saved with an error is tainted, and the next apply
			// replaces it: a new token, and the name recorded as deleted.
			// Nothing has used this one yet, so undo it and let the next
			// apply create it again whole.
			msg := fmt.Sprintf("The sidecar %q was created, but its slack channels failed: %v", created.Name, err)
			if derr := r.client.DeleteSidecar(created.ID); derr != nil {
				msg += fmt.Sprintf(". Removing the sidecar again failed too: %v. Import it with `terraform import` or delete it in the control plane", derr)
			} else {
				msg += ". The sidecar was removed again; fix the channels and apply"
			}
			resp.Diagnostics.AddError("Error Setting Sidecar Slack Channels", msg)
			return
		}
	}
	resp.Diagnostics.Append(r.readSidecarInto(ctx, &plan, &created.Sidecar)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sidecarResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sidecarResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The gateway resolves a name as well as an id, which is what lets an
	// import pass either.
	sc, err := r.client.GetSidecar(state.ID.ValueString())
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Sidecar",
			fmt.Sprintf("Failed reading sidecar %q: %v", state.ID.ValueString(), err))
		return
	}
	resp.Diagnostics.Append(r.readSidecarInto(ctx, &state, sc)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sidecarResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state sidecarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	// The owner switch goes first and alone. PUT refuses a document that
	// flips load_from_disk, and PATCH with the flag false must carry nothing
	// else, so the switch is its own call and the document follows.
	if plan.LoadFromDisk.ValueBool() != state.LoadFromDisk.ValueBool() {
		flag := "false"
		if plan.LoadFromDisk.ValueBool() {
			flag = "true"
		}
		patch := json.RawMessage(fmt.Sprintf(`{%q: %s}`, loadFromDiskKey, flag))
		switched, err := r.client.PatchSidecarConfiguration(id, patch)
		if err != nil {
			resp.Diagnostics.AddError("Error Switching Sidecar Configuration Owner",
				fmt.Sprintf("Failed to set load_from_disk to %s on sidecar %q: %v", flag, plan.Name.ValueString(), err))
			return
		}
		reportDetachedRules(plan.Name.ValueString(), switched.DetachedRules, &resp.Diagnostics)
	}

	doc, err := documentForWrite(plan.Configuration, plan.LoadFromDisk.ValueBool())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("configuration"), "Invalid Sidecar Configuration", err.Error())
		return
	}
	sc, err := r.client.UpdateSidecarConfiguration(id, doc)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Sidecar",
			fmt.Sprintf("Failed to update sidecar %q: %v", plan.Name.ValueString(), err))
		return
	}

	// The gateway pruned the channels of every listener the new document no
	// longer has, and refuses a channel entry for a listener it does not
	// know. So the clear-outs for dropped listeners are limited to listeners
	// that still exist; the rest are already gone.
	channels, write, diags := slackChannelsForWrite(ctx, plan.SlackChannels, state.SlackChannels, listenerProtocols(plan.Configuration))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if write {
		if _, err := r.client.SetSidecarSlackChannels(id, channels); err != nil {
			resp.Diagnostics.AddError("Error Setting Sidecar Slack Channels",
				fmt.Sprintf("Failed to set the slack channels of sidecar %q: %v", plan.Name.ValueString(), err))
			return
		}
	}

	plan.Token = state.Token
	resp.Diagnostics.Append(r.readSidecarInto(ctx, &plan, sc)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sidecarResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sidecarResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteSidecar(state.ID.ValueString())
	if err != nil && !hoop.IsNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Sidecar",
			fmt.Sprintf("Failed to delete sidecar %q: %v", state.Name.ValueString(), err))
	}
}

// ImportState accepts the sidecar name or its id. Read resolves either and
// stores the id. The token is not recoverable, so it stays empty.
func (r *sidecarResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *sidecarResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// reportDetachedRules turns the PATCH answer of an owner switch into a
// warning naming each rule it deleted or unbound. A deleted rule that
// Terraform manages comes back on the next apply; an unbound one is rebound
// when its sidecar_targets are applied again.
func reportDetachedRules(sidecar string, detached *hoop.SidecarDetachedRules, diags *diag.Diagnostics) {
	if detached.Empty() {
		return
	}
	var lines []string
	add := func(verb string, names hoop.SidecarRuleNames) {
		for _, n := range names.Guardrails {
			lines = append(lines, fmt.Sprintf("%s guardrail %q", verb, n))
		}
		for _, n := range names.DataMasking {
			lines = append(lines, fmt.Sprintf("%s data masking rule %q", verb, n))
		}
		for _, n := range names.Analyzers {
			lines = append(lines, fmt.Sprintf("%s analyzer rule %q", verb, n))
		}
	}
	add("deleted", detached.Deleted)
	add("unbound", detached.Unbound)
	diags.AddAttributeWarning(path.Root("load_from_disk"), "Owner Switch Detached Rules",
		fmt.Sprintf("Switching load_from_disk on sidecar %q detached these rules:\n\n%s\n\n"+
			"Run terraform plan to see which of them Terraform will recreate or rebind.",
			sidecar, strings.Join(lines, "\n")))
}
