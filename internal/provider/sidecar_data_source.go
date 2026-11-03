// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ datasource.DataSource              = &sidecarDataSource{}
	_ datasource.DataSourceWithConfigure = &sidecarDataSource{}
)

func NewSidecarDataSource() datasource.DataSource {
	return &sidecarDataSource{}
}

type sidecarDataSourceModel struct {
	ID              types.String       `tfsdk:"id"`
	Name            types.String       `tfsdk:"name"`
	Configuration   sidecarConfigValue `tfsdk:"configuration"`
	LoadFromDisk    types.Bool         `tfsdk:"load_from_disk"`
	Version         types.String       `tfsdk:"version"`
	LastSeenAt      types.String       `tfsdk:"last_seen_at"`
	ConfigState     types.String       `tfsdk:"config_state"`
	LastError       types.String       `tfsdk:"last_error"`
	IdentityIssuer  types.String       `tfsdk:"identity_issuer"`
	IdentitySubject types.String       `tfsdk:"identity_subject"`
	BoundRules      types.List         `tfsdk:"bound_rules"`
	Instances       types.List         `tfsdk:"instances"`
}

// sidecarInstanceModel is one instances entry.
type sidecarInstanceModel struct {
	Name            types.String `tfsdk:"name"`
	Status          types.String `tfsdk:"status"`
	Version         types.String `tfsdk:"version"`
	LastSeenAt      types.String `tfsdk:"last_seen_at"`
	AppliedRevision types.String `tfsdk:"applied_revision"`
	LastOutcome     types.String `tfsdk:"last_outcome"`
	LastError       types.String `tfsdk:"last_error"`
}

var sidecarInstanceObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"name":             types.StringType,
		"status":           types.StringType,
		"version":          types.StringType,
		"last_seen_at":     types.StringType,
		"applied_revision": types.StringType,
		"last_outcome":     types.StringType,
		"last_error":       types.StringType,
	},
}

// instancesValue is the instances the gateway reported, or null when it sent
// none: an older gateway, or no pod has reported yet.
func instancesValue(ctx context.Context, sc *hoop.Sidecar) (types.List, diag.Diagnostics) {
	if sc.Instances == nil {
		return types.ListNull(sidecarInstanceObjectType), nil
	}
	items := make([]sidecarInstanceModel, 0, len(sc.Instances))
	for _, in := range sc.Instances {
		items = append(items, sidecarInstanceModel{
			Name:            types.StringValue(in.Name),
			Status:          types.StringValue(in.Status),
			Version:         types.StringValue(in.Version),
			LastSeenAt:      types.StringValue(in.LastSeenAt),
			AppliedRevision: types.StringValue(in.AppliedRevision),
			LastOutcome:     types.StringValue(in.LastOutcome),
			LastError:       types.StringValue(in.LastError),
		})
	}
	return types.ListValueFrom(ctx, sidecarInstanceObjectType, items)
}

type sidecarDataSource struct {
	client *hoop.Client
}

func (d *sidecarDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sidecar"
}

func (d *sidecarDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a sidecar by name. A sidecar that a `hoop_sidecar_service_account` identity created " +
			"on its first handshake has no `hoop_sidecar` resource; this data source gives rule resources its id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the sidecar, for `sidecar_targets`.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The sidecar name.",
				Required:    true,
			},
			"configuration": schema.StringAttribute{
				Description: "The stored configuration document, as JSON, without `load_from_disk`.",
				CustomType:  sidecarConfigType{},
				Computed:    true,
			},
			"load_from_disk": schema.BoolAttribute{
				Description: "Whether the sidecar runs its own config file.",
				Computed:    true,
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
				Description: "What the sidecar runs relative to the stored configuration. See `hoop_sidecar`.",
				Computed:    true,
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
			"bound_rules": schema.ListNestedAttribute{
				Description: "The rules the control plane folds into this sidecar's configuration, and the listener each lands on.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"kind":          schema.StringAttribute{Computed: true, Description: "`guardrail`, `datamasking` or `analyzer`."},
						"name":          schema.StringAttribute{Computed: true, Description: "The rule name."},
						"listener_name": schema.StringAttribute{Computed: true, Description: "The listener the rule is bound to."},
					},
				},
			},
			"instances": schema.ListNestedAttribute{
				Description: "The pods behind this sidecar, one per hostname that reported to the control plane, " +
					"ordered by name. They share one configuration and one identity; this lists which of them " +
					"are up and what each runs. Null when none has reported, or on a gateway from before instances.",
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":             schema.StringAttribute{Computed: true, Description: "The hostname the pod reports."},
						"status":           schema.StringAttribute{Computed: true, Description: "`online` when the pod reported recently, else `offline`."},
						"version":          schema.StringAttribute{Computed: true, Description: "The sidecar binary version the pod runs."},
						"last_seen_at":     schema.StringAttribute{Computed: true, Description: "When the pod last reached the control plane, RFC 3339."},
						"applied_revision": schema.StringAttribute{Computed: true, Description: "The revision of the configuration the pod last applied."},
						"last_outcome":     schema.StringAttribute{Computed: true, Description: "What the pod did with the last configuration it received, such as `applied`."},
						"last_error":       schema.StringAttribute{Computed: true, Description: "Why the pod refused the last configuration. Empty when it applied."},
					},
				},
			},
		},
	}
}

func (d *sidecarDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data sidecarDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sc, err := d.client.GetSidecar(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Sidecar",
			fmt.Sprintf("Failed reading sidecar %q: %v", data.Name.ValueString(), err))
		return
	}
	doc, loadFromDisk, err := splitDocument(sc.Configuration)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Sidecar Configuration", err.Error())
		return
	}
	data.ID = types.StringValue(sc.ID)
	data.Name = types.StringValue(sc.Name)
	data.Configuration = newSidecarConfigValue(string(doc))
	data.LoadFromDisk = types.BoolValue(loadFromDisk)
	data.Version = types.StringValue(sc.Version)
	data.LastSeenAt = types.StringValue(sc.LastSeenAt)
	data.ConfigState = types.StringValue(sc.ConfigState)
	data.LastError = types.StringValue(sc.LastError)
	data.IdentityIssuer = types.StringValue(sc.IdentityIssuer)
	data.IdentitySubject = types.StringValue(sc.IdentitySubject)
	bound, diags := boundRulesValue(ctx, sc)
	resp.Diagnostics.Append(diags...)
	data.BoundRules = bound
	instances, diags := instancesValue(ctx, sc)
	resp.Diagnostics.Append(diags...)
	data.Instances = instances
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *sidecarDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}
