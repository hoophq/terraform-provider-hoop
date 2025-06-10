// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &connectionResource{}
	_ resource.ResourceWithConfigure = &connectionResource{}
)

// NewconnectionResource is a helper function to simplify the provider implementation.
func NewConnectionResource() resource.Resource {
	return &connectionResource{}

}

// connectionResourceModel maps the data source schema data.
type connectionResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	AgentID             types.String `tfsdk:"agent_id"`
	Type                types.String `tfsdk:"type"`
	Subtype             types.String `tfsdk:"subtype"`
	Command             types.List   `tfsdk:"command"`
	Secrets             types.Map    `tfsdk:"secrets"`
	Reviewers           types.List   `tfsdk:"reviewers"`
	RedactTypes         types.List   `tfsdk:"redact_types"`
	Tags                types.Map    `tfsdk:"tags"`
	AccessModeRunbooks  types.String `tfsdk:"access_mode_runbooks"`
	AccessModeExec      types.String `tfsdk:"access_mode_exec"`
	AccessModeConnect   types.String `tfsdk:"access_mode_connect"`
	AccessSchema        types.String `tfsdk:"access_schema"`
	GuardRailRules      types.List   `tfsdk:"guardrail_rules"`
	JiraIssueTemplateID types.String `tfsdk:"jira_issue_template_id"`
}

// connectionResource is the data source implementation.
type connectionResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *connectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

// Schema defines the schema for the data source.
func (r *connectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a connection resource in Hoop Platform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the connection resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the connection resource.",
				Required:    true,
			},
			"agent_id": schema.StringAttribute{
				Description: "The ID of the agent associated with the connection.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description: "The type of the connection resource. Valid values are 'database', 'application', or 'custom'.",
				Required:    true,
				Validators:  ConnectionTypeValidator,
			},
			"subtype": schema.StringAttribute{
				Description: "The subtype of the connection resource.",
				Optional:    true,
				Validators:  NonEmptyStringValidator,
			},
			"command": schema.ListAttribute{
				Description: "The command entrypoint that will be executed for one off executions. Each command argument should be a separate entry in the list.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyListValidator,
			},
			"secrets": schema.MapAttribute{
				Description: "A map of secrets to be used by the connection.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyMapValidator,
				Sensitive:   true,
			},
			"reviewers": schema.ListAttribute{
				Description: "A list of approver groups that are allowed to approve a session.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyListValidator,
			},
			"redact_types": schema.ListAttribute{
				Description: "A list of redact types, these values are dependent of which DLP provider is being used.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyListValidator,
			},
			"tags": schema.MapAttribute{
				Description: "A map of tags to be associated with the connection.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyMapValidator,
			},
			"access_mode_runbooks": schema.StringAttribute{
				Description: "Enables or disables access to runbooks for the connection. Accept values are 'enabled' or 'disabled'.",
				Required:    true,
				Validators:  AccessModeValidator,
			},
			"access_mode_exec": schema.StringAttribute{
				Description: "Enables or disables access to execute one off commands for the connection. Accept values are 'enabled' or 'disabled'.",
				Required:    true,
				Validators:  AccessModeValidator,
			},
			"access_mode_connect": schema.StringAttribute{
				Description: "Enables or disables access native access when interacting with the connection. Accept values are 'enabled' or 'disabled'.",
				Required:    true,
				Validators:  AccessModeValidator,
			},
			"access_schema": schema.StringAttribute{
				Description: "Enables or disables displaying the introspection schema tree of database type connections.",
				Required:    true,
				Validators:  AccessModeValidator,
			},

			"guardrail_rules": schema.ListAttribute{
				Description: "A list of guardrail rule ids to be applied to the connection.",
				Optional:    true,
				ElementType: types.StringType,
				Validators:  NonEmptyListValidator,
			},
			"jira_issue_template_id": schema.StringAttribute{
				Description: "The ID of the Jira issue template to be used for the connection.",
				Optional:    true,
				Validators:  NonEmptyStringValidator,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *connectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentState connectionResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Info(ctx, "running read for connection resource")

	connection, err := r.client.GetConnection(currentState.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Connection",
			fmt.Sprintf("Failed to read connection: %v", err),
		)
		return
	}

	newState, diags := toConnectionResourceModel(ctx, currentState, connection)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Model",
			fmt.Sprintf("Failed to convert connection model: %v", diags),
		)
		return
	}
	// Set refreshed state
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *connectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	requestConnection, diags := toConnectionHoopAPI(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Model",
			fmt.Sprintf("Failed to convert connection model: %v", diags),
		)
		return
	}

	connection, err := r.client.CreateConnection(requestConnection)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Connection",
			fmt.Sprintf("Failed to create connection: %v", err),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ID = types.StringValue(connection.ID)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *connectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	tflog.Info(ctx, "running update for connection resource")
	var plan connectionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqConn, diags := toConnectionHoopAPI(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Model",
			fmt.Sprintf("Failed to convert connection model: %v", diags),
		)
		return
	}

	newConn, err := r.client.UpdateConnection(reqConn)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Connection",
			fmt.Sprintf("Failed to update connection: %v", err),
		)
		return
	}

	newState, diags := toConnectionResourceModel(ctx, plan, newConn)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Model",
			fmt.Sprintf("Failed to convert connection model: %v", diags),
		)
		return
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *connectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteConnection(state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Connection",
			fmt.Sprintf("Failed to delete connection: %v", err),
		)
		return
	}
}

func (r *connectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

// Configure adds the provider configured client to the data source.
func (r *connectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func toConnectionResourceModel(ctx context.Context, state connectionResourceModel, obj *hoop.Connection) (newState *connectionResourceModel, diags diag.Diagnostics) {
	// required attributes
	state.ID = types.StringValue(obj.ID)
	state.Name = types.StringValue(obj.Name)
	state.AgentID = types.StringValue(obj.AgentId)
	state.Type = types.StringValue(obj.Type)
	state.AccessModeRunbooks = types.StringValue(obj.AccessModeRunbooks)
	state.AccessModeExec = types.StringValue(obj.AccessModeExec)
	state.AccessModeConnect = types.StringValue(obj.AccessModeConnect)
	state.AccessSchema = types.StringValue(obj.AccessSchema)

	// we must check the state for null values before assigning when there are optional attributes
	if !state.Command.IsNull() {
		state.Command, diags = types.ListValueFrom(ctx, types.StringType, obj.Command)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !state.Reviewers.IsNull() {
		state.Reviewers, diags = types.ListValueFrom(ctx, types.StringType, obj.Reviewers)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !state.GuardRailRules.IsNull() {
		state.GuardRailRules, diags = types.ListValueFrom(ctx, types.StringType, obj.GuardRailRules)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !state.RedactTypes.IsNull() {
		state.RedactTypes, diags = types.ListValueFrom(ctx, types.StringType, obj.RedactTypes)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !state.Secrets.IsNull() {
		state.Secrets, diags = types.MapValueFrom(ctx, types.StringType, obj.Secrets)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !state.Tags.IsNull() {
		state.Tags, diags = types.MapValueFrom(ctx, types.StringType, obj.ConnectionTags)
		if diags.HasError() {
			return nil, diags
		}
	}

	if !state.Subtype.IsNull() {
		state.Subtype = types.StringValue(obj.SubType)
	}

	if !state.JiraIssueTemplateID.IsNull() {
		state.JiraIssueTemplateID = types.StringValue(obj.JiraIssueTemplateID)
	}

	return &state, nil
}

func toConnectionHoopAPI(ctx context.Context, obj connectionResourceModel) (conn hoop.Connection, diags diag.Diagnostics) {
	command, diags := convertListToStringSlice(ctx, obj.Command)
	if diags.HasError() {
		return
	}
	redactTypes, diags := convertListToStringSlice(ctx, obj.RedactTypes)
	if diags.HasError() {
		return
	}
	Reviewers, diags := convertListToStringSlice(ctx, obj.Reviewers)
	if diags.HasError() {
		return
	}
	guardRailRules, diags := convertListToStringSlice(ctx, obj.GuardRailRules)
	if diags.HasError() {
		return
	}

	var connectionTags map[string]string
	diags = obj.Tags.ElementsAs(ctx, &connectionTags, false)
	if diags.HasError() {
		return conn, diags
	}

	var secrets map[string]string
	diags = obj.Secrets.ElementsAs(ctx, &secrets, false)
	if diags.HasError() {
		return conn, diags
	}

	return hoop.Connection{
		Name:                obj.Name.ValueString(),
		Command:             command,
		Type:                obj.Type.ValueString(),
		SubType:             obj.Subtype.ValueString(),
		Secrets:             secrets,
		AgentId:             obj.AgentID.ValueString(),
		Reviewers:           Reviewers,
		RedactEnabled:       true,
		RedactTypes:         redactTypes,
		ConnectionTags:      connectionTags,
		AccessModeRunbooks:  obj.AccessModeRunbooks.ValueString(),
		AccessModeExec:      obj.AccessModeExec.ValueString(),
		AccessModeConnect:   obj.AccessModeConnect.ValueString(),
		AccessSchema:        obj.AccessSchema.ValueString(),
		GuardRailRules:      guardRailRules,
		JiraIssueTemplateID: obj.JiraIssueTemplateID.ValueString(),
	}, nil
}

func convertListToStringSlice(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}

	var result []string
	diags := list.ElementsAs(ctx, &result, false)
	if diags.HasError() {
		return nil, diags
	}

	return result, nil
}
