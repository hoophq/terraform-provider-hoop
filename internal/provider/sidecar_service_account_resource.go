// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = &sidecarServiceAccountResource{}
	_ resource.ResourceWithImportState = &sidecarServiceAccountResource{}
)

func NewSidecarServiceAccountResource() resource.Resource {
	return &sidecarServiceAccountResource{}
}

type sidecarServiceAccountResourceModel struct {
	ID                    types.String         `tfsdk:"id"`
	Name                  types.String         `tfsdk:"name"`
	Issuer                types.String         `tfsdk:"issuer"`
	Audience              types.String         `tfsdk:"audience"`
	Claim                 types.String         `tfsdk:"claim"`
	SubjectPattern        types.String         `tfsdk:"subject_pattern"`
	NameTemplate          types.String         `tfsdk:"name_template"`
	JWKS                  jsontypes.Normalized `tfsdk:"jwks"`
	AllowAnySubject       types.Bool           `tfsdk:"allow_any_subject"`
	AdoptExistingSidecars types.Bool           `tfsdk:"adopt_existing_sidecars"`
}

type sidecarServiceAccountResource struct {
	client *hoop.Client
}

func (r *sidecarServiceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sidecar_service_account"
}

var sidecarClaimValidator = []validator.String{
	stringvalidator.OneOf("sub", "email"),
}

func (r *sidecarServiceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lets sidecars authenticate with a token their platform issued, a Kubernetes projected " +
			"service account token or a Google ID token, instead of a token the control plane issued per sidecar. " +
			"A matching token reaches the sidecar `name_template` names, and the control plane creates that sidecar " +
			"on its first handshake. Use `data.hoop_sidecar` to bind rules to a sidecar created this way. " +
			"Needs an Enterprise license.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "A label for this mapping, unique in the organization.",
				Required:    true,
				Validators:  NonEmptyStringValidator,
			},
			"issuer": schema.StringAttribute{
				Description: "The exact `iss` of the tokens. An https URL. The control plane fetches the signing keys " +
					"through OIDC discovery unless `jwks` is set.",
				Required:   true,
				Validators: NonEmptyStringValidator,
			},
			"audience": schema.StringAttribute{
				Description: "The `aud` the tokens must carry: the control plane URL the sidecar uses. " +
					"One organization owns each issuer and audience pair.",
				Required:   true,
				Validators: NonEmptyStringValidator,
			},
			"claim": schema.StringAttribute{
				Description: "The claim matched against `subject_pattern`. `sub` for a Kubernetes service account; " +
					"`email` for a Google service account, whose token must carry `email_verified`.",
				Required:   true,
				Validators: sidecarClaimValidator,
			},
			"subject_pattern": schema.StringAttribute{
				Description: "An exact value, or one with a single `*` that matches one or more characters, " +
					"such as `system:serviceaccount:*:hoop-sidecar`. A bare `*` needs `allow_any_subject`.",
				Required:   true,
				Validators: NonEmptyStringValidator,
			},
			"name_template": schema.StringAttribute{
				Description: "The name of the sidecar a matching token reaches. `{1}` is the text the `*` matched.",
				Required:    true,
				Validators:  NonEmptyStringValidator,
			},
			"jwks": jsonAttribute("A static JWKS document for an issuer the control plane cannot reach. " +
				"Omit it to use OIDC discovery on the issuer."),
			"allow_any_subject": schema.BoolAttribute{
				Description: "Allows the bare `*` pattern, which admits every subject of the issuer.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"adopt_existing_sidecars": schema.BoolAttribute{
				Description: "Lets a matching token reach a sidecar an admin registered with a token, when no identity " +
					"is bound to it yet, and binds it. Without it such a token is refused.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
		},
	}
}

func (m *sidecarServiceAccountResourceModel) toApi() hoop.SidecarServiceAccount {
	var jwks json.RawMessage
	if !m.JWKS.IsNull() && !m.JWKS.IsUnknown() {
		jwks = json.RawMessage(m.JWKS.ValueString())
	}
	return hoop.SidecarServiceAccount{
		ID:                    m.ID.ValueString(),
		Name:                  m.Name.ValueString(),
		Issuer:                m.Issuer.ValueString(),
		Audience:              m.Audience.ValueString(),
		Claim:                 m.Claim.ValueString(),
		SubjectPattern:        m.SubjectPattern.ValueString(),
		NameTemplate:          m.NameTemplate.ValueString(),
		JWKS:                  jwks,
		AllowAnySubject:       m.AllowAnySubject.ValueBool(),
		AdoptExistingSidecars: m.AdoptExistingSidecars.ValueBool(),
	}
}

func (m *sidecarServiceAccountResourceModel) fromApi(sa *hoop.SidecarServiceAccount) {
	m.ID = types.StringValue(sa.ID)
	m.Name = types.StringValue(sa.Name)
	m.Issuer = types.StringValue(sa.Issuer)
	m.Audience = types.StringValue(sa.Audience)
	m.Claim = types.StringValue(sa.Claim)
	m.SubjectPattern = types.StringValue(sa.SubjectPattern)
	m.NameTemplate = types.StringValue(sa.NameTemplate)
	m.AllowAnySubject = types.BoolValue(sa.AllowAnySubject)
	m.AdoptExistingSidecars = types.BoolValue(sa.AdoptExistingSidecars)
	if len(sa.JWKS) == 0 || string(sa.JWKS) == "null" {
		m.JWKS = jsontypes.NewNormalizedNull()
	} else {
		m.JWKS = jsontypes.NewNormalizedValue(string(sa.JWKS))
	}
}

func (r *sidecarServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sidecarServiceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sa, err := r.client.CreateSidecarServiceAccount(plan.toApi())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Sidecar Service Account",
			fmt.Sprintf("Failed to create sidecar service account %q: %v", plan.Name.ValueString(), err),
		)
		return
	}
	plan.fromApi(sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sidecarServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sidecarServiceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sa, err := r.client.GetSidecarServiceAccount(state.ID.ValueString())
	if hoop.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Sidecar Service Account",
			fmt.Sprintf("Failed reading sidecar service account with ID %q: %v", state.ID.ValueString(), err),
		)
		return
	}
	state.fromApi(sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sidecarServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan sidecarServiceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sa, err := r.client.UpdateSidecarServiceAccount(plan.toApi())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Sidecar Service Account",
			fmt.Sprintf("Failed to update sidecar service account %q: %v", plan.Name.ValueString(), err),
		)
		return
	}
	plan.fromApi(sa)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sidecarServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sidecarServiceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteSidecarServiceAccount(state.ID.ValueString())
	if err != nil && !hoop.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Error Deleting Sidecar Service Account",
			fmt.Sprintf("Failed to delete sidecar service account with ID %q: %v", state.ID.ValueString(), err),
		)
	}
}

func (r *sidecarServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *sidecarServiceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
