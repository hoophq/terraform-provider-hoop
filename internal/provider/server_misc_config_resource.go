// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                = &serverMiscConfigResource{}
	_ resource.ResourceWithImportState = &serverMiscConfigResource{}
)

// serverMiscConfigID is the id of the one row a gateway has.
const serverMiscConfigID = "server"

func NewServerMiscConfigResource() resource.Resource {
	return &serverMiscConfigResource{}
}

type serverMiscConfigResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	HttpProxyServerConfig types.Object `tfsdk:"http_proxy_server_config"`
}

type serverProxyConfigModel struct {
	ListenAddress types.String `tfsdk:"listen_address"`
}

var serverProxyConfigObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"listen_address": types.StringType,
	},
}

// listenAddressValidator is the gateway's check: host:port, or empty to stop
// the proxy.
var listenAddressValidator = []validator.String{
	stringvalidator.RegexMatches(regexp.MustCompile(`^$|:`), "must be `host:port`, or empty to stop the proxy"),
}

type serverMiscConfigResource struct {
	client *hoop.Client
}

func (r *serverMiscConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_misc_config"
}

func (r *serverMiscConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the HTTP proxy listener of `/api/serverconfig/misc`, the HTTP proxy block behind " +
			"Settings → Server → Proxies of a self-hosted gateway. The RDP, PostgreSQL and SSH listeners stay " +
			"with the web app: every write reads the row and passes them back unchanged. The block is written " +
			"exactly as written on every apply, so one without `listen_address` stops the proxy, and a change " +
			"made in the web app shows as drift that the next apply puts back. Destroying the resource stops " +
			"the HTTP proxy and leaves the other listeners untouched. An import reads the block into state " +
			"when the gateway holds it. A multi-tenant gateway refuses the route.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Always `server`: a gateway has one row.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"http_proxy_server_config": schema.SingleNestedAttribute{
				Description: "The HTTP proxy. Leaving the block out leaves it to the web app: the value the gateway " +
					"holds is passed back unchanged on every write and never read into state. Declaring it, even " +
					"as `{}`, writes it exactly as written on every apply, so a change made in the web app shows " +
					"as drift on the next plan and the apply puts it back. Removing a declared block stops the proxy.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"listen_address": schema.StringAttribute{
						Description: "The `host:port` the proxy listens on, such as `0.0.0.0:18888`. " +
							"Leaving it out stops the proxy.",
						Optional:   true,
						Validators: listenAddressValidator,
					},
				},
			},
		},
	}
}

// toApiProxyConfig picks what goes out for the block: the plan's value when
// declared, the stop value when the prior declared it and the plan no longer
// does, and the gateway's own value when neither did.
func toApiProxyConfig(ctx context.Context, plan, prior types.Object, current *hoop.ServerProxyConfig) (*hoop.ServerProxyConfig, diag.Diagnostics) {
	if plan.IsNull() {
		if prior.IsNull() {
			return current, nil
		}
		return &hoop.ServerProxyConfig{}, nil
	}

	var m serverProxyConfigModel
	diags := plan.As(ctx, &m, basetypesObjectAsOptions)
	return &hoop.ServerProxyConfig{ListenAddress: m.ListenAddress.ValueString()}, diags
}

// toApi builds the whole row from the plan over current, the gateway's row:
// the gRPC URL and the blocks the provider does not manage pass through
// unchanged.
func (m *serverMiscConfigResourceModel) toApi(ctx context.Context, prior *serverMiscConfigResourceModel, current *hoop.ServerMiscConfig) (*hoop.ServerMiscConfig, diag.Diagnostics) {
	cfg := &hoop.ServerMiscConfig{
		GrpcServerURL:        current.GrpcServerURL,
		PostgresServerConfig: current.PostgresServerConfig,
		SSHServerConfig:      current.SSHServerConfig,
		RDPServerConfig:      current.RDPServerConfig,
	}
	httpProxy, diags := toApiProxyConfig(ctx, m.HttpProxyServerConfig, prior.HttpProxyServerConfig, current.HttpProxyServerConfig)
	cfg.HttpProxyServerConfig = httpProxy
	return cfg, diags
}

// fromApi writes the gateway's answer over prior: the plan after a write, the
// state on a read. The block lands in state when the prior declared it and
// stays null otherwise, however the gateway answers it. An import has no
// prior and the gateway does not record whether Terraform owns the block, so
// it sets every to read it in when the gateway holds it.
func (m *serverMiscConfigResourceModel) fromApi(ctx context.Context, prior *serverMiscConfigResourceModel, cfg *hoop.ServerMiscConfig, every bool) diag.Diagnostics {
	m.ID = types.StringValue(serverMiscConfigID)
	httpProxy, diags := fromApiProxyConfig(ctx, prior.HttpProxyServerConfig, cfg.HttpProxyServerConfig, every)
	m.HttpProxyServerConfig = httpProxy
	return diags
}

func fromApiProxyConfig(ctx context.Context, prior types.Object, cfg *hoop.ServerProxyConfig, every bool) (types.Object, diag.Diagnostics) {
	null := types.ObjectNull(serverProxyConfigObjectType.AttrTypes)
	if cfg == nil || (prior.IsNull() && !every) {
		return null, nil
	}

	var p serverProxyConfigModel
	if !prior.IsNull() && !prior.IsUnknown() {
		if d := prior.As(ctx, &p, basetypesObjectAsOptions); d.HasError() {
			return null, d
		}
	}
	return types.ObjectValueFrom(ctx, serverProxyConfigObjectType.AttrTypes, serverProxyConfigModel{
		ListenAddress: fromApiString(p.ListenAddress, cfg.ListenAddress),
	})
}

// write reads the row, rebuilds it from plan over prior and puts it back, so
// what the plan leaves out keeps what the gateway holds.
func (r *serverMiscConfigResource) write(ctx context.Context, plan, prior *serverMiscConfigResourceModel, verb string) diag.Diagnostics {
	var diags diag.Diagnostics
	current, err := r.client.GetServerMiscConfig()
	if err != nil {
		diags.AddError(fmt.Sprintf("Error %s Server Misc Config", verb),
			fmt.Sprintf("Failed reading the proxy servers configuration before writing it: %v", err))
		return diags
	}

	cfg, d := plan.toApi(ctx, prior, current)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	out, err := r.client.UpdateServerMiscConfig(cfg)
	if err != nil {
		diags.AddError(fmt.Sprintf("Error %s Server Misc Config", verb),
			fmt.Sprintf("Failed writing the proxy servers configuration: %v", err))
		return diags
	}

	var state serverMiscConfigResourceModel
	diags.Append(state.fromApi(ctx, plan, out, false)...)
	if !diags.HasError() {
		*plan = state
	}
	return diags
}

// serverMiscConfigNone is the model with no block declared: the prior of a
// create, the plan of a destroy.
func serverMiscConfigNone() serverMiscConfigResourceModel {
	return serverMiscConfigResourceModel{
		HttpProxyServerConfig: types.ObjectNull(serverProxyConfigObjectType.AttrTypes),
	}
}

func (r *serverMiscConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverMiscConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	prior := serverMiscConfigNone()
	resp.Diagnostics.Append(r.write(ctx, &plan, &prior, "Creating")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read treats a 404 as a gateway without the route, so the resource drops out
// of state. A 403 is a multi-tenant gateway, which is an error, not drift.
func (r *serverMiscConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverMiscConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, err := r.client.GetServerMiscConfig()
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Server Misc Config",
			fmt.Sprintf("Failed reading the proxy servers configuration: %v", err))
		return
	}

	var next serverMiscConfigResourceModel
	resp.Diagnostics.Append(next.fromApi(ctx, &state, cfg, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *serverMiscConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serverMiscConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.write(ctx, &plan, &state, "Updating")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete stops the HTTP proxy when the state declared it: the row stays, the
// gateway has no route to remove it. A gateway without the route has nothing
// to stop.
func (r *serverMiscConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverMiscConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current, err := r.client.GetServerMiscConfig()
	if hoop.IsNotFound(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Server Misc Config",
			fmt.Sprintf("Failed reading the proxy servers configuration before stopping the HTTP proxy: %v", err))
		return
	}

	none := serverMiscConfigNone()
	cfg, d := none.toApi(ctx, &state, current)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateServerMiscConfig(cfg); err != nil {
		resp.Diagnostics.AddError("Error Deleting Server Misc Config",
			fmt.Sprintf("Failed stopping the HTTP proxy: %v", err))
		return
	}
	resp.Diagnostics.AddWarning("Server Misc Config Proxy Stopped",
		"The HTTP proxy was stopped; the web app may start it again. "+
			"The RDP, PostgreSQL and SSH listeners are untouched.")
}

// ImportState reads the HTTP proxy block when the gateway holds it: the
// gateway does not record whether Terraform owns it, so the configuration
// decides after the import.
func (r *serverMiscConfigResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cfg, err := r.client.GetServerMiscConfig()
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Server Misc Config",
			fmt.Sprintf("Failed reading the proxy servers configuration: %v", err))
		return
	}

	none := serverMiscConfigNone()
	var state serverMiscConfigResourceModel
	resp.Diagnostics.Append(state.fromApi(ctx, &none, cfg, true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serverMiscConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
