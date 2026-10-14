// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                   = &sidecarResource{}
	_ resource.ResourceWithImportState    = &sidecarResource{}
	_ resource.ResourceWithValidateConfig = &sidecarResource{}
)

func NewSidecarResource() resource.Resource {
	return &sidecarResource{}
}

type sidecarResourceModel struct {
	ID            types.String         `tfsdk:"id"`
	Name          types.String         `tfsdk:"name"`
	Configuration jsontypes.Normalized `tfsdk:"configuration"`
	LoadFromDisk  types.Bool           `tfsdk:"load_from_disk"`
	SlackChannels types.Map            `tfsdk:"slack_channels"`
	Token         types.String         `tfsdk:"token"`

	Version         types.String `tfsdk:"version"`
	LastSeenAt      types.String `tfsdk:"last_seen_at"`
	ConfigState     types.String `tfsdk:"config_state"`
	LastError       types.String `tfsdk:"last_error"`
	IdentityIssuer  types.String `tfsdk:"identity_issuer"`
	IdentitySubject types.String `tfsdk:"identity_subject"`
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
			"The sidecar fetches that configuration on every heartbeat. Rules reach it through the " +
			"`sidecar_targets` of `hoop_guardrail`, `hoop_datamasking_rules` and `hoop_ai_session_analyzer_rule`, " +
			"not through this resource. Needs an Enterprise license.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the sidecar. Rule resources reference it in `sidecar_targets`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The sidecar name, unique in the organization. Changing it replaces the sidecar and its token.",
				Required:    true,
				Validators:  NonEmptyStringValidator,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"configuration": schema.StringAttribute{
				Description: "The sidecar configuration document as a JSON object, in the schema of the sidecar " +
					"config file: `listeners`, `admin`, `audit` and so on. `jsonencode(yamldecode(file(\"sidecar.yaml\")))` " +
					"converts an existing file. Leave zero values out: the gateway drops them and the plan would " +
					"show them as a change. Omit the attribute to store an empty document; the sidecar then refuses " +
					"to start until one is set. Do not put `load_from_disk` here; use the attribute.",
				CustomType: jsontypes.NormalizedType{},
				Optional:   true,
			},
			"load_from_disk": schema.BoolAttribute{
				Description: "When true the sidecar runs its own config file and the control plane serves only the " +
					"license. Switching it on detaches the rules imported from this sidecar; switching it off clears " +
					"the stored document before `configuration` is written.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"slack_channels": schema.MapAttribute{
				Description: "Slack channel ids per listener name, where that listener's reviews are posted. " +
					"A listener left out uses the organization's default channel. Leave the attribute out to keep " +
					"channels set outside Terraform; removing a declared value clears every listener it named.",
				ElementType: slackChannelsElementType,
				Optional:    true,
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
			},
			"identity_subject": schema.StringAttribute{
				Description: "The subject of the service account identity bound to this sidecar, if one reached it.",
				Computed:    true,
			},
		},
	}
}

// ValidateConfig refuses a load_from_disk key inside the configuration
// document. The attribute exists so the resource can route the change through
// PATCH; a copy inside the JSON would bypass that and fail at apply.
func (r *sidecarResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data sidecarResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Configuration.IsNull() || data.Configuration.IsUnknown() {
		return
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data.Configuration.ValueString()), &doc); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("configuration"),
			"Invalid Sidecar Configuration",
			fmt.Sprintf("The configuration must be a JSON object: %v", err))
		return
	}
	if _, ok := doc[loadFromDiskKey]; ok {
		resp.Diagnostics.AddAttributeError(path.Root("configuration"),
			"load_from_disk Belongs in Its Own Attribute",
			"Remove the load_from_disk key from configuration and set the load_from_disk attribute instead. "+
				"The gateway only accepts a change to it through a separate call, which the attribute drives.")
	}
}

// documentForWrite builds the document the gateway stores: the planned
// configuration, or an empty object, with load_from_disk added when on.
func documentForWrite(cfg jsontypes.Normalized, loadFromDisk bool) (json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	if !cfg.IsNull() && !cfg.IsUnknown() {
		if err := json.Unmarshal([]byte(cfg.ValueString()), &doc); err != nil {
			return nil, fmt.Errorf("the configuration must be a JSON object: %w", err)
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
		model.Configuration = jsontypes.NewNormalizedNull()
	} else {
		model.Configuration = jsontypes.NewNormalizedValue(string(doc))
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
// plan dropped goes out with no channels, which clears it.
func slackChannelsForWrite(ctx context.Context, plan, prior types.Map) (map[string][]string, bool, diag.Diagnostics) {
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
			if _, kept := out[name]; !kept {
				out[name] = []string{}
			}
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

	channels, write, diags := slackChannelsForWrite(ctx, plan.SlackChannels, types.MapNull(slackChannelsElementType))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if write {
		if _, err := r.client.SetSidecarSlackChannels(created.ID, channels); err != nil {
			resp.Diagnostics.AddError("Error Setting Sidecar Slack Channels",
				fmt.Sprintf("The sidecar %q was created, but its slack channels failed: %v", created.Name, err))
			// The sidecar exists. Record it so the next apply retries the
			// channels instead of trying to create the sidecar again.
			resp.Diagnostics.Append(r.readSidecarInto(ctx, &plan, &created.Sidecar)...)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
		if _, err := r.client.PatchSidecarConfiguration(id, patch); err != nil {
			resp.Diagnostics.AddError("Error Switching Sidecar Configuration Owner",
				fmt.Sprintf("Failed to set load_from_disk to %s on sidecar %q: %v", flag, plan.Name.ValueString(), err))
			return
		}
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

	channels, write, diags := slackChannelsForWrite(ctx, plan.SlackChannels, state.SlackChannels)
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
