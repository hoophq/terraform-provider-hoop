// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

var (
	_ resource.Resource                   = &serverAuthConfigResource{}
	_ resource.ResourceWithImportState    = &serverAuthConfigResource{}
	_ resource.ResourceWithValidateConfig = &serverAuthConfigResource{}
)

// serverAuthConfigID is the id of the one row a gateway has.
const serverAuthConfigID = "server"

func NewServerAuthConfigResource() resource.Resource {
	return &serverAuthConfigResource{}
}

type serverAuthConfigResourceModel struct {
	ID                          types.String `tfsdk:"id"`
	AuthMethod                  types.String `tfsdk:"auth_method"`
	OidcConfig                  types.Object `tfsdk:"oidc_config"`
	SamlConfig                  types.Object `tfsdk:"saml_config"`
	ProviderName                types.String `tfsdk:"provider_name"`
	ApiKey                      types.String `tfsdk:"api_key"`
	WebappUsersManagementStatus types.String `tfsdk:"webapp_users_management_status"`
	AdminRoleName               types.String `tfsdk:"admin_role_name"`
	AuditorRoleName             types.String `tfsdk:"auditor_role_name"`
}

type serverAuthOidcModel struct {
	IssuerURL           types.String `tfsdk:"issuer_url"`
	ClientID            types.String `tfsdk:"client_id"`
	ClientSecret        types.String `tfsdk:"client_secret"`
	ClientAssertionFile types.String `tfsdk:"client_assertion_file"`
	Scopes              types.List   `tfsdk:"scopes"`
	Audience            types.String `tfsdk:"audience"`
	GroupsClaim         types.String `tfsdk:"groups_claim"`
}

type serverAuthSamlModel struct {
	IdpMetadataURL   types.String `tfsdk:"idp_metadata_url"`
	GroupsClaim      types.String `tfsdk:"groups_claim"`
	ResolvedMetadata types.Object `tfsdk:"resolved_metadata"`
}

var serverAuthOidcObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"issuer_url":            types.StringType,
		"client_id":             types.StringType,
		"client_secret":         types.StringType,
		"client_assertion_file": types.StringType,
		"scopes":                types.ListType{ElemType: types.StringType},
		"audience":              types.StringType,
		"groups_claim":          types.StringType,
	},
}

var serverAuthSamlResolvedObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"entity_id":              types.StringType,
		"sso_url":                types.StringType,
		"certificate_expires_at": types.StringType,
		"resolved_at":            types.StringType,
	},
}

var serverAuthSamlObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"idp_metadata_url":  types.StringType,
		"groups_claim":      types.StringType,
		"resolved_metadata": serverAuthSamlResolvedObjectType,
	},
}

var authMethodValidator = []validator.String{
	stringvalidator.OneOf("local", "oidc", "saml"),
}

var webappUsersManagementStatusValidator = []validator.String{
	stringvalidator.OneOf("active", "inactive"),
}

type serverAuthConfigResource struct {
	client *hoop.Client
}

func (r *serverAuthConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_auth_config"
}

func (r *serverAuthConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the gateway's authentication configuration, the one row behind Settings → Authentication " +
			"on a self-hosted gateway. Every apply writes the whole row: an attribute left out here is cleared on the " +
			"gateway, including values set in the web app. There is one row per gateway and it cannot be deleted; " +
			"destroying the resource only forgets it. A multi-tenant gateway refuses the route.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Always `server`: a gateway has one row.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"auth_method": schema.StringAttribute{
				Description: "How users sign in: `local` (gateway-managed accounts), `oidc` or `saml`. " +
					"`oidc` needs `oidc_config`, `saml` needs `saml_config`.",
				Required:   true,
				Validators: authMethodValidator,
			},
			"oidc_config": schema.SingleNestedAttribute{
				Description: "The OIDC / OAuth2 identity provider. The gateway reaches the issuer when the row is written " +
					"and refuses the write when it cannot. Leaving the block out clears the stored OIDC settings.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"issuer_url": schema.StringAttribute{
						Description: "The issuer URL, such as `https://auth.example.com/oidc`.",
						Required:    true,
						Validators:  NonEmptyStringValidator,
					},
					"client_id": schema.StringAttribute{
						Description: "The OAuth2 client id.",
						Required:    true,
						Validators:  NonEmptyStringValidator,
					},
					"client_secret": schema.StringAttribute{
						Description: "The OAuth2 client secret. Set it or `client_assertion_file`, not both.",
						Optional:    true,
						Sensitive:   true,
					},
					"client_assertion_file": schema.StringAttribute{
						Description: "Path on the gateway to a JWT that authenticates the client in place of " +
							"`client_secret` (RFC 7523). It must be the path the gateway operator set in " +
							"`IDP_CLIENT_ASSERTION_FILE`.",
						Optional: true,
					},
					"scopes": schema.ListAttribute{
						Description: "Extra OAuth2 scopes appended to `openid`, `profile` and `email`. Leaving it out stores none.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"audience": schema.StringAttribute{
						Description: "The OAuth2 audience. Leaving it out stores none.",
						Optional:    true,
					},
					"groups_claim": schema.StringAttribute{
						Description: "The token claim that carries the user's groups. Defaults to `groups`.",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("groups"),
					},
				},
			},
			"saml_config": schema.SingleNestedAttribute{
				Description: "The SAML 2.0 identity provider. The gateway fetches the metadata when the row is written " +
					"and refuses the write when it cannot. Leaving the block out clears the stored SAML settings.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"idp_metadata_url": schema.StringAttribute{
						Description: "The identity provider metadata URL.",
						Required:    true,
						Validators:  NonEmptyStringValidator,
					},
					"groups_claim": schema.StringAttribute{
						Description: "The assertion attribute that carries the user's groups. Defaults to `groups`.",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("groups"),
					},
					"resolved_metadata": schema.SingleNestedAttribute{
						Description: "What the metadata URL resolved to on the last write, so the IdP can be checked.",
						Computed:    true,
						Attributes: map[string]schema.Attribute{
							"entity_id": schema.StringAttribute{
								Description: "The entity id declared in the metadata document.",
								Computed:    true,
							},
							"sso_url": schema.StringAttribute{
								Description: "The single sign-on URL users are redirected to.",
								Computed:    true,
							},
							"certificate_expires_at": schema.StringAttribute{
								Description: "When the newest signing certificate in the metadata expires, RFC 3339.",
								Computed:    true,
							},
							"resolved_at": schema.StringAttribute{
								Description: "When the metadata was fetched, RFC 3339.",
								Computed:    true,
							},
						},
					},
				},
			},
			"provider_name": schema.StringAttribute{
				Description: "The identity provider's name, shown on the sign-in page. Leaving it out stores none.",
				Optional:    true,
			},
			"webapp_users_management_status": schema.StringAttribute{
				Description: "`active` lets admins create, edit and delete users in the web app; `inactive` leaves " +
					"users to the identity provider.",
				Required:   true,
				Validators: webappUsersManagementStatusValidator,
			},
			"admin_role_name": schema.StringAttribute{
				Description: "The group that grants administrator access. Defaults to `admin`. The gateway authorizes " +
					"admin routes against this group right away, so the API key the provider uses must belong to the " +
					"new group (or be the gateway's legacy `API_KEY`) before the change is applied, or the next " +
					"refresh answers 403.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("admin"),
			},
			"auditor_role_name": schema.StringAttribute{
				Description: "The group that grants auditor access. Defaults to `auditor`.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("auditor"),
			},
			"api_key": schema.StringAttribute{
				Description: "The admin api key stored in the row after a key rollout; null until one happened. " +
					"Rollouts are not managed here.",
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

// ValidateConfig refuses at plan what the gateway refuses at write: an auth
// method without its block, and a client with two credentials.
func (r *serverAuthConfigResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data serverAuthConfigResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.AuthMethod.IsUnknown() {
		switch data.AuthMethod.ValueString() {
		case "oidc":
			if data.OidcConfig.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root("oidc_config"), "OIDC Configuration Required",
					"auth_method oidc needs oidc_config with the issuer and the client.")
			}
		case "saml":
			if data.SamlConfig.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root("saml_config"), "SAML Configuration Required",
					"auth_method saml needs saml_config with the identity provider metadata URL.")
			}
		}
	}
	if data.OidcConfig.IsNull() || data.OidcConfig.IsUnknown() {
		return
	}
	var oidc serverAuthOidcModel
	resp.Diagnostics.Append(data.OidcConfig.As(ctx, &oidc, basetypesObjectAsOptions)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !oidc.ClientSecret.IsNull() && !oidc.ClientAssertionFile.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("oidc_config").AtName("client_assertion_file"),
			"Client Credential Conflict",
			"Set client_secret or client_assertion_file, not both: the gateway authenticates the client with one.")
	}
}

func (m *serverAuthConfigResourceModel) toApi(ctx context.Context) (*hoop.ServerAuthConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	cfg := &hoop.ServerAuthConfig{
		AuthMethod:                  m.AuthMethod.ValueString(),
		ProviderName:                m.ProviderName.ValueString(),
		WebappUsersManagementStatus: m.WebappUsersManagementStatus.ValueString(),
		AdminRoleName:               m.AdminRoleName.ValueString(),
		AuditorRoleName:             m.AuditorRoleName.ValueString(),
	}
	if !m.OidcConfig.IsNull() {
		var oidc serverAuthOidcModel
		diags.Append(m.OidcConfig.As(ctx, &oidc, basetypesObjectAsOptions)...)
		scopes, d := toApiStringList(ctx, oidc.Scopes)
		diags.Append(d...)
		cfg.OidcConfig = &hoop.ServerAuthOidcConfig{
			IssuerURL:           oidc.IssuerURL.ValueString(),
			ClientID:            oidc.ClientID.ValueString(),
			ClientSecret:        oidc.ClientSecret.ValueString(),
			ClientAssertionFile: oidc.ClientAssertionFile.ValueString(),
			Scopes:              scopes,
			Audience:            oidc.Audience.ValueString(),
			GroupsClaim:         oidc.GroupsClaim.ValueString(),
		}
	}
	if !m.SamlConfig.IsNull() {
		var saml serverAuthSamlModel
		diags.Append(m.SamlConfig.As(ctx, &saml, basetypesObjectAsOptions)...)
		cfg.SamlConfig = &hoop.ServerAuthSamlConfig{
			IdpMetadataURL: saml.IdpMetadataURL.ValueString(),
			GroupsClaim:    saml.GroupsClaim.ValueString(),
		}
	}
	return cfg, diags
}

// fromApi writes the gateway's answer over prior: the plan after a write, the
// state on a read. Optional values the prior never declared stay null. A
// block the prior never declared is read in only when the auth method selects
// it, so an import or a drift to that method lands in state, while a block the
// gateway synthesizes from its environment under another method does not.
func (m *serverAuthConfigResourceModel) fromApi(ctx context.Context, prior *serverAuthConfigResourceModel, cfg *hoop.ServerAuthConfig) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(serverAuthConfigID)
	m.AuthMethod = types.StringValue(cfg.AuthMethod)
	m.ProviderName = fromApiString(prior.ProviderName, cfg.ProviderName)
	m.WebappUsersManagementStatus = types.StringValue(cfg.WebappUsersManagementStatus)
	m.AdminRoleName = types.StringValue(cfg.AdminRoleName)
	m.AuditorRoleName = types.StringValue(cfg.AuditorRoleName)
	m.ApiKey = types.StringNull()
	if cfg.ApiKey != nil {
		m.ApiKey = types.StringValue(*cfg.ApiKey)
	}

	oidc, d := fromApiOidcConfig(ctx, prior.OidcConfig, cfg.OidcConfig, cfg.AuthMethod == "oidc")
	diags.Append(d...)
	m.OidcConfig = oidc

	saml, d := fromApiSamlConfig(ctx, prior.SamlConfig, cfg.SamlConfig, cfg.AuthMethod == "saml")
	diags.Append(d...)
	m.SamlConfig = saml
	return diags
}

func fromApiOidcConfig(ctx context.Context, prior types.Object, cfg *hoop.ServerAuthOidcConfig, selected bool) (types.Object, diag.Diagnostics) {
	null := types.ObjectNull(serverAuthOidcObjectType.AttrTypes)
	if cfg == nil || (prior.IsNull() && !selected) {
		return null, nil
	}
	p := serverAuthOidcModel{Scopes: types.ListNull(types.StringType)}
	if !prior.IsNull() && !prior.IsUnknown() {
		if d := prior.As(ctx, &p, basetypesObjectAsOptions); d.HasError() {
			return null, d
		}
	}
	scopes, diags := fromApiStringList(ctx, p.Scopes, cfg.Scopes)
	if diags.HasError() {
		return null, diags
	}
	obj, d := types.ObjectValueFrom(ctx, serverAuthOidcObjectType.AttrTypes, serverAuthOidcModel{
		IssuerURL:           types.StringValue(cfg.IssuerURL),
		ClientID:            types.StringValue(cfg.ClientID),
		ClientSecret:        fromApiString(p.ClientSecret, cfg.ClientSecret),
		ClientAssertionFile: fromApiString(p.ClientAssertionFile, cfg.ClientAssertionFile),
		Scopes:              scopes,
		Audience:            fromApiString(p.Audience, cfg.Audience),
		GroupsClaim:         types.StringValue(cfg.GroupsClaim),
	})
	diags.Append(d...)
	return obj, diags
}

func fromApiSamlConfig(ctx context.Context, prior types.Object, cfg *hoop.ServerAuthSamlConfig, selected bool) (types.Object, diag.Diagnostics) {
	null := types.ObjectNull(serverAuthSamlObjectType.AttrTypes)
	if cfg == nil || (prior.IsNull() && !selected) {
		return null, nil
	}
	resolved := types.ObjectNull(serverAuthSamlResolvedObjectType.AttrTypes)
	if rm := cfg.ResolvedMetadata; rm != nil {
		resolved = types.ObjectValueMust(serverAuthSamlResolvedObjectType.AttrTypes, map[string]attr.Value{
			"entity_id":              types.StringValue(rm.EntityID),
			"sso_url":                types.StringValue(rm.SsoURL),
			"certificate_expires_at": types.StringValue(rm.CertificateExpiresAt),
			"resolved_at":            types.StringValue(rm.ResolvedAt),
		})
	}
	return types.ObjectValueFrom(ctx, serverAuthSamlObjectType.AttrTypes, serverAuthSamlModel{
		IdpMetadataURL:   types.StringValue(cfg.IdpMetadataURL),
		GroupsClaim:      types.StringValue(cfg.GroupsClaim),
		ResolvedMetadata: resolved,
	})
}

func (r *serverAuthConfigResource) write(ctx context.Context, plan *serverAuthConfigResourceModel, verb string) diag.Diagnostics {
	cfg, diags := plan.toApi(ctx)
	if diags.HasError() {
		return diags
	}
	out, err := r.client.UpdateServerAuthConfig(cfg)
	if err != nil {
		diags.AddError(fmt.Sprintf("Error %s Server Auth Config", verb),
			fmt.Sprintf("Failed writing the %s authentication configuration: %v", cfg.AuthMethod, err))
		return diags
	}
	var state serverAuthConfigResourceModel
	diags.Append(state.fromApi(ctx, plan, out)...)
	if !diags.HasError() {
		*plan = state
	}
	return diags
}

func (r *serverAuthConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serverAuthConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &plan, "Creating")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read treats a 404 as a gateway without the route, so the resource drops out
// of state. A 403 is a multi-tenant gateway, which is an error, not drift.
func (r *serverAuthConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serverAuthConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, err := r.client.GetServerAuthConfig()
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Server Auth Config",
			fmt.Sprintf("Failed reading the authentication configuration: %v", err))
		return
	}
	var next serverAuthConfigResourceModel
	resp.Diagnostics.Append(next.fromApi(ctx, &state, cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *serverAuthConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serverAuthConfigResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &plan, "Updating")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the row: the gateway has no route to remove it.
func (r *serverAuthConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Server Auth Config Kept",
		"The gateway keeps its authentication configuration; there is no route to delete it. "+
			"Change it with another apply or in the web app.")
}

func (r *serverAuthConfigResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), serverAuthConfigID)...)
}

func (r *serverAuthConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
