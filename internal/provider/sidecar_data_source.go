// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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
	ID              types.String         `tfsdk:"id"`
	Name            types.String         `tfsdk:"name"`
	Configuration   jsontypes.Normalized `tfsdk:"configuration"`
	LoadFromDisk    types.Bool           `tfsdk:"load_from_disk"`
	Version         types.String         `tfsdk:"version"`
	LastSeenAt      types.String         `tfsdk:"last_seen_at"`
	ConfigState     types.String         `tfsdk:"config_state"`
	LastError       types.String         `tfsdk:"last_error"`
	IdentityIssuer  types.String         `tfsdk:"identity_issuer"`
	IdentitySubject types.String         `tfsdk:"identity_subject"`
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
				CustomType:  jsontypes.NormalizedType{},
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
	data.Configuration = jsontypes.NewNormalizedValue(string(doc))
	data.LoadFromDisk = types.BoolValue(loadFromDisk)
	data.Version = types.StringValue(sc.Version)
	data.LastSeenAt = types.StringValue(sc.LastSeenAt)
	data.ConfigState = types.StringValue(sc.ConfigState)
	data.LastError = types.StringValue(sc.LastError)
	data.IdentityIssuer = types.StringValue(sc.IdentityIssuer)
	data.IdentitySubject = types.StringValue(sc.IdentitySubject)
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
