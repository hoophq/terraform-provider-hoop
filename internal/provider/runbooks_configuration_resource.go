// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &runbookConfigurationResource{}
	_ resource.ResourceWithImportState = &runbookConfigurationResource{}
)

// NewRunbookConfigurationResource is a helper function to simplify the provider implementation.
func NewRunbookConfigurationResource() resource.Resource {
	return &runbookConfigurationResource{}
}

// runbookConfigurationResourceModel maps the data source schema data.
type runbookConfigurationResourceModel struct {
	Repository    types.String `tfsdk:"repository"`
	GitURL        types.String `tfsdk:"git_url"`
	GitUser       types.String `tfsdk:"git_user"`
	GitPassword   types.String `tfsdk:"git_password"`
	GitHookTTL    types.Int32  `tfsdk:"git_hook_ttl"`
	SSHUser       types.String `tfsdk:"ssh_user"`
	SSHKey        types.String `tfsdk:"ssh_key"`
	SSHKeyPass    types.String `tfsdk:"ssh_keypass"`
	SSHKnownHosts types.String `tfsdk:"ssh_known_hosts"`
}

// runbookConfigurationResource is the data source implementation.
type runbookConfigurationResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *runbookConfigurationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook_configuration"
}

// Schema defines the schema for the data source.
func (r *runbookConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Runbook Configuration resources. Make sure to work with this resource only with the gateway version 1.47.0 and onwards",
		Attributes: map[string]schema.Attribute{
			"repository": schema.StringAttribute{
				Description: "The normalized name of the repository. E.g.: 'github.com/hoophq/runbooks'",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"git_url": schema.StringAttribute{
				Required:    true,
				Description: "Git repository URL where the runbook is located.",
			},
			"git_user": schema.StringAttribute{
				Required:    true,
				Description: "Git username for repository authentication.",
			},
			"git_password": schema.StringAttribute{
				Required:    true,
				Description: "Git password or token for repository authentication.",
			},
			"git_hook_ttl": schema.Int32Attribute{
				Required:    true,
				Description: "Git password or token for repository authentication.",
			},
			"ssh_user": schema.StringAttribute{
				Required:    true,
				Description: "SSH username for Git repository authentication.",
			},
			"ssh_key": schema.StringAttribute{
				Required:    true,
				Description: "SSH private key for Git repository authentication.",
			},
			"ssh_keypass": schema.StringAttribute{
				Required:    true,
				Description: "SSH key passphrase for encrypted SSH keys.",
			},
			"ssh_known_hosts": schema.StringAttribute{
				Required:    true,
				Description: "SSH known hosts for host key verification.",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *runbookConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentState runbookConfigurationResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.GetRunbookConfigByURL(currentState.GitURL.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Runbook Repository",
			fmt.Sprintf("Failed reading runbooks repository for %v, err=%v", currentState.GitURL.ValueString(), err),
		)
		return
	}

	currentState.GitURL = types.StringValue(repo.GitURL)
	currentState.GitUser = types.StringValue(repo.GitUser)
	currentState.GitPassword = types.StringValue(repo.GitPassword)
	currentState.GitHookTTL = types.Int32Value(repo.GitHookTTL)
	currentState.SSHUser = types.StringValue(repo.SSHUser)
	currentState.SSHKey = types.StringValue(repo.SSHKey)
	currentState.SSHKeyPass = types.StringValue(repo.SSHKeyPass)
	currentState.SSHKnownHosts = types.StringValue(repo.SSHKnownHosts)
	currentState.Repository = types.StringValue(repo.Repository)
	diags = resp.State.Set(ctx, currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *runbookConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runbookConfigurationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.CreateRunbookRepo(hoop.RunbookRepo{
		GitURL:        plan.GitURL.ValueString(),
		GitUser:       plan.GitUser.ValueString(),
		GitPassword:   plan.GitPassword.ValueString(),
		GitHookTTL:    plan.GitHookTTL.ValueInt32(),
		SSHUser:       plan.SSHUser.ValueString(),
		SSHKey:        plan.SSHKey.ValueString(),
		SSHKeyPass:    plan.SSHKeyPass.ValueString(),
		SSHKnownHosts: plan.SSHKnownHosts.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Runbook Repository",
			fmt.Sprintf("Failed updating repository: %v", err),
		)
		return
	}

	plan.Repository = types.StringValue(repo.Repository)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *runbookConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runbookConfigurationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.UpdateRunbookRepoByID(hoop.RunbookRepo{
		GitURL:        plan.GitURL.ValueString(),
		GitUser:       plan.GitUser.ValueString(),
		GitPassword:   plan.GitPassword.ValueString(),
		GitHookTTL:    plan.GitHookTTL.ValueInt32(),
		SSHUser:       plan.SSHUser.ValueString(),
		SSHKey:        plan.SSHKey.ValueString(),
		SSHKeyPass:    plan.SSHKeyPass.ValueString(),
		SSHKnownHosts: plan.SSHKnownHosts.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Runbook Configuration",
			fmt.Sprintf("Failed to update runbook configuration: %v", err),
		)
		return
	}

	plan.Repository = types.StringValue(repo.Repository)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *runbookConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runbookConfigurationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRunbookRepoByID(state.GitURL.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Runbook Configuration",
			fmt.Sprintf("Failed to delete runbook repositories configuration: %v", err),
		)
		return
	}
}

func (r *runbookConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("git_url"), req, resp)
}

// Configure adds the provider configured client to the data source.
func (r *runbookConfigurationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
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
	r.client = client
}
