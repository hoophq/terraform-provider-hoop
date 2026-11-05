// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                   = &serverMiscConfigResource{}
	_ resource.ResourceWithImportState    = &serverMiscConfigResource{}
	_ resource.ResourceWithValidateConfig = &serverMiscConfigResource{}
)

// serverMiscConfigID is the id of the one row a gateway has.
const serverMiscConfigID = "server"

func NewServerMiscConfigResource() resource.Resource {
	return &serverMiscConfigResource{}
}

type serverMiscConfigResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	RdpServerConfig       types.Object `tfsdk:"rdp_server_config"`
	PostgresServerConfig  types.Object `tfsdk:"postgres_server_config"`
	HttpProxyServerConfig types.Object `tfsdk:"http_proxy_server_config"`
	SshServerConfig       types.Object `tfsdk:"ssh_server_config"`
}

type serverProxyConfigModel struct {
	ListenAddress types.String `tfsdk:"listen_address"`
}

type serverSshConfigModel struct {
	ListenAddress types.String `tfsdk:"listen_address"`
	HostsKey      types.String `tfsdk:"hosts_key"`
	TrustedCAs    types.List   `tfsdk:"trusted_cas"`
	UserMapping   types.Object `tfsdk:"user_mapping"`
}

type serverSshUserMappingModel struct {
	CertAttr types.String `tfsdk:"cert_attr"`
	UserAttr types.String `tfsdk:"user_attr"`
}

var serverProxyConfigObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"listen_address": types.StringType,
	},
}

var serverSshUserMappingObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"cert_attr": types.StringType,
		"user_attr": types.StringType,
	},
}

var serverSshConfigObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"listen_address": types.StringType,
		"hosts_key":      types.StringType,
		"trusted_cas":    types.ListType{ElemType: types.StringType},
		"user_mapping":   serverSshUserMappingObjectType,
	},
}

// listenAddressValidator is the gateway's check: host:port, or empty to stop
// the proxy.
var listenAddressValidator = []validator.String{
	stringvalidator.RegexMatches(regexp.MustCompile(`^$|:`), "must be `host:port`, or empty to stop the proxy"),
}

var sshCertAttrValidator = []validator.String{
	stringvalidator.OneOf("principal", "key_id"),
}

var sshUserAttrValidator = []validator.String{
	stringvalidator.OneOf("email", "subject", "user_id"),
}

type serverMiscConfigResource struct {
	client *hoop.Client
}

func (r *serverMiscConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_misc_config"
}

// serverMiscBlockDescription is the rule every block follows.
const serverMiscBlockDescription = "Leaving the block out leaves it to the web app: the value the gateway holds is " +
	"passed back unchanged on every write and never read into state. Declaring it, even as `{}`, writes it " +
	"exactly as written on every apply, so a change made in the web app shows as drift on the next plan and " +
	"the apply puts it back. Removing a declared block stops that proxy."

func serverListenAddressAttribute(example string) schema.StringAttribute {
	return schema.StringAttribute{
		Description: "The `host:port` the proxy listens on, such as `" + example + "`. " +
			"Leaving it out stops the proxy.",
		Optional:   true,
		Validators: listenAddressValidator,
	}
}

func serverProxyConfigAttribute(proxy, example string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Description: "The " + proxy + " proxy. " + serverMiscBlockDescription,
		Optional:    true,
		Attributes: map[string]schema.Attribute{
			"listen_address": serverListenAddressAttribute(example),
		},
	}
}

func (r *serverMiscConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the proxy listeners of `/api/serverconfig/misc`, the blocks behind " +
			"Settings → Server → Proxies of a self-hosted gateway. Each block follows one rule: left out, it " +
			"stays with the web app and is passed back unchanged on every write; declared, even as `{}`, it is " +
			"written exactly as written on every apply, so a change made in the web app shows as drift that " +
			"the next apply puts back; removed, that proxy stops. Destroying the resource stops every declared " +
			"proxy and leaves the others untouched. An import reads every block the gateway holds into state: " +
			"the gateway cannot say which ones Terraform owns, so declare the blocks to keep; the next apply " +
			"stops the proxy of an imported block the configuration leaves out. " +
			"A multi-tenant gateway refuses the route.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Always `server`: a gateway has one row.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"rdp_server_config":        serverProxyConfigAttribute("RDP", "0.0.0.0:13389"),
			"postgres_server_config":   serverProxyConfigAttribute("PostgreSQL", "0.0.0.0:15432"),
			"http_proxy_server_config": serverProxyConfigAttribute("HTTP", "0.0.0.0:18888"),
			"ssh_server_config": schema.SingleNestedAttribute{
				Description: "The SSH proxy. " + serverMiscBlockDescription,
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"listen_address": serverListenAddressAttribute("0.0.0.0:12222"),
					"hosts_key": schema.StringAttribute{
						Description: "The host key, base64 PEM. Generated by the gateway when the SSH proxy starts " +
							"without one, and kept across applies.",
						Optional:  true,
						Computed:  true,
						Sensitive: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"trusted_cas": schema.ListAttribute{
						Description: "SSH CA public keys in `authorized_keys` format. When set, the proxy accepts " +
							"certificate authentication and `user_mapping` is required.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"user_mapping": schema.SingleNestedAttribute{
						Description: "How a certificate maps to a user. Required with `trusted_cas`.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"cert_attr": schema.StringAttribute{
								Description: "The certificate field looked up: `principal` (the first matching " +
									"principal wins) or `key_id`.",
								Required:   true,
								Validators: sshCertAttrValidator,
							},
							"user_attr": schema.StringAttribute{
								Description: "The user field matched against it: `email`, `subject` or `user_id`.",
								Required:    true,
								Validators:  sshUserAttrValidator,
							},
						},
					},
				},
			},
		},
	}
}

// ValidateConfig refuses at plan what the gateway refuses at write: trusted
// CAs without the mapping that names the user.
func (r *serverMiscConfigResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data serverMiscConfigResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.SshServerConfig.IsNull() || data.SshServerConfig.IsUnknown() {
		return
	}

	var ssh serverSshConfigModel
	resp.Diagnostics.Append(data.SshServerConfig.As(ctx, &ssh, basetypesObjectAsOptions)...)
	if resp.Diagnostics.HasError() || ssh.TrustedCAs.IsNull() || ssh.TrustedCAs.IsUnknown() {
		return
	}

	if len(ssh.TrustedCAs.Elements()) > 0 && ssh.UserMapping.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("ssh_server_config").AtName("user_mapping"),
			"SSH User Mapping Required",
			"trusted_cas needs user_mapping: the gateway has to know which user a certificate signs in.")
	}
}

// toApiProxyConfig picks what goes out for one block: the plan's value when
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

// toApiSshConfig follows toApiProxyConfig. An unknown hosts_key goes out
// empty, so the gateway generates one; a known one is sent back as stored.
func toApiSshConfig(ctx context.Context, plan, prior types.Object, current *hoop.ServerSSHConfig) (*hoop.ServerSSHConfig, diag.Diagnostics) {
	if plan.IsNull() {
		if prior.IsNull() {
			return current, nil
		}
		return &hoop.ServerSSHConfig{}, nil
	}

	var m serverSshConfigModel
	diags := plan.As(ctx, &m, basetypesObjectAsOptions)
	cas, d := toApiStringList(ctx, m.TrustedCAs)
	diags.Append(d...)
	cfg := &hoop.ServerSSHConfig{
		ListenAddress: m.ListenAddress.ValueString(),
		HostsKey:      m.HostsKey.ValueString(),
		TrustedCAs:    cas,
	}
	if !m.UserMapping.IsNull() {
		var um serverSshUserMappingModel
		diags.Append(m.UserMapping.As(ctx, &um, basetypesObjectAsOptions)...)
		cfg.UserMapping = &hoop.ServerSSHUserMap{
			CertAttribute: um.CertAttr.ValueString(),
			UserAttribute: um.UserAttr.ValueString(),
		}
	}
	return cfg, diags
}

// toApi builds the whole row from the plan over current, the gateway's row:
// the gRPC URL and the blocks neither the plan nor the prior declare pass
// through unchanged.
func (m *serverMiscConfigResourceModel) toApi(ctx context.Context, prior *serverMiscConfigResourceModel, current *hoop.ServerMiscConfig) (*hoop.ServerMiscConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	cfg := &hoop.ServerMiscConfig{GrpcServerURL: current.GrpcServerURL}

	rdp, d := toApiProxyConfig(ctx, m.RdpServerConfig, prior.RdpServerConfig, current.RDPServerConfig)
	diags.Append(d...)
	cfg.RDPServerConfig = rdp

	pg, d := toApiProxyConfig(ctx, m.PostgresServerConfig, prior.PostgresServerConfig, current.PostgresServerConfig)
	diags.Append(d...)
	cfg.PostgresServerConfig = pg

	httpProxy, d := toApiProxyConfig(ctx, m.HttpProxyServerConfig, prior.HttpProxyServerConfig, current.HttpProxyServerConfig)
	diags.Append(d...)
	cfg.HttpProxyServerConfig = httpProxy

	ssh, d := toApiSshConfig(ctx, m.SshServerConfig, prior.SshServerConfig, current.SSHServerConfig)
	diags.Append(d...)
	cfg.SSHServerConfig = ssh
	return cfg, diags
}

// fromApi writes the gateway's answer over prior: the plan after a write, the
// state on a read. A block lands in state when the prior declared it and
// stays null otherwise, however the gateway answers it. An import has no
// prior and the gateway does not record which blocks Terraform owns, so it
// sets every to read in each block the gateway holds.
func (m *serverMiscConfigResourceModel) fromApi(ctx context.Context, prior *serverMiscConfigResourceModel, cfg *hoop.ServerMiscConfig, every bool) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(serverMiscConfigID)

	rdp, d := fromApiProxyConfig(ctx, prior.RdpServerConfig, cfg.RDPServerConfig, every)
	diags.Append(d...)
	m.RdpServerConfig = rdp

	pg, d := fromApiProxyConfig(ctx, prior.PostgresServerConfig, cfg.PostgresServerConfig, every)
	diags.Append(d...)
	m.PostgresServerConfig = pg

	httpProxy, d := fromApiProxyConfig(ctx, prior.HttpProxyServerConfig, cfg.HttpProxyServerConfig, every)
	diags.Append(d...)
	m.HttpProxyServerConfig = httpProxy

	ssh, d := fromApiSshConfig(ctx, prior.SshServerConfig, cfg.SSHServerConfig, every)
	diags.Append(d...)
	m.SshServerConfig = ssh
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

func fromApiSshConfig(ctx context.Context, prior types.Object, cfg *hoop.ServerSSHConfig, every bool) (types.Object, diag.Diagnostics) {
	null := types.ObjectNull(serverSshConfigObjectType.AttrTypes)
	if cfg == nil || (prior.IsNull() && !every) {
		return null, nil
	}

	p := serverSshConfigModel{TrustedCAs: types.ListNull(types.StringType)}
	if !prior.IsNull() && !prior.IsUnknown() {
		if d := prior.As(ctx, &p, basetypesObjectAsOptions); d.HasError() {
			return null, d
		}
	}
	cas, diags := fromApiStringList(ctx, p.TrustedCAs, cfg.TrustedCAs)
	if diags.HasError() {
		return null, diags
	}

	userMapping := types.ObjectNull(serverSshUserMappingObjectType.AttrTypes)
	if um := cfg.UserMapping; um != nil {
		userMapping = types.ObjectValueMust(serverSshUserMappingObjectType.AttrTypes, map[string]attr.Value{
			"cert_attr": types.StringValue(um.CertAttribute),
			"user_attr": types.StringValue(um.UserAttribute),
		})
	}
	obj, d := types.ObjectValueFrom(ctx, serverSshConfigObjectType.AttrTypes, serverSshConfigModel{
		ListenAddress: fromApiString(p.ListenAddress, cfg.ListenAddress),
		HostsKey:      types.StringValue(cfg.HostsKey),
		TrustedCAs:    cas,
		UserMapping:   userMapping,
	})
	diags.Append(d...)
	return obj, diags
}

// write reads the row, rebuilds it from plan over prior and puts it back, so
// the blocks the plan leaves out keep what the gateway holds.
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
		RdpServerConfig:       types.ObjectNull(serverProxyConfigObjectType.AttrTypes),
		PostgresServerConfig:  types.ObjectNull(serverProxyConfigObjectType.AttrTypes),
		HttpProxyServerConfig: types.ObjectNull(serverProxyConfigObjectType.AttrTypes),
		SshServerConfig:       types.ObjectNull(serverSshConfigObjectType.AttrTypes),
	}
}

// declaredProxies names the proxies the model declares, in the order the
// schema lists them.
func (m *serverMiscConfigResourceModel) declaredProxies() []string {
	var out []string
	for _, b := range []struct {
		name  string
		block types.Object
	}{
		{"RDP", m.RdpServerConfig},
		{"PostgreSQL", m.PostgresServerConfig},
		{"HTTP", m.HttpProxyServerConfig},
		{"SSH", m.SshServerConfig},
	} {
		if !b.block.IsNull() {
			out = append(out, b.name)
		}
	}
	return out
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

// Delete stops every proxy the state declares: the row stays, the gateway has
// no route to remove it. With nothing declared there is nothing to stop, and
// a gateway without the route has nothing either.
func (r *serverMiscConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serverMiscConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	stopped := state.declaredProxies()
	if len(stopped) == 0 {
		return
	}

	current, err := r.client.GetServerMiscConfig()
	if hoop.IsNotFound(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Server Misc Config",
			fmt.Sprintf("Failed reading the proxy servers configuration before stopping the declared proxies: %v", err))
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
			fmt.Sprintf("Failed stopping the declared proxies: %v", err))
		return
	}
	resp.Diagnostics.AddWarning("Server Misc Config Proxies Stopped",
		fmt.Sprintf("Stopped: %s. The web app may start them again. "+
			"The blocks Terraform did not declare are untouched.", strings.Join(stopped, ", ")))
}

// ImportState reads every block the gateway holds: the gateway does not
// record which ones Terraform owns, so the configuration decides after the
// import.
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
