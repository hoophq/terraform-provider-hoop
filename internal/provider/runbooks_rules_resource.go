// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                = &runbookRulesResource{}
	_ resource.ResourceWithImportState = &runbookRulesResource{}
)

// NewRunbookRulesResource is a helper function to simplify the provider implementation.
func NewRunbookRulesResource() resource.Resource {
	return &runbookRulesResource{}
}

// runbooksRulesResourceModel maps the data source schema data.
type runbooksRulesResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Connections types.List   `tfsdk:"connections"`
	UserGroups  types.List   `tfsdk:"user_groups"`
	Runbooks    types.List   `tfsdk:"runbooks"`
}

// runbookRulesResource is the data source implementation.
type runbookRulesResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *runbookRulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runbook_rule"
}

// Schema defines the schema for the data source.
func (r *runbookRulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Runbook Rules resources. It allows defining which connections and groups could interact with runbooks. Make sure to work with this resource only with the gateway version 1.47.0 and onwards",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the connection resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the rule.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description of the rule.",
				Required:    true,
			},
			"connections": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "List of connection names which this rule applies to.",
				Required:    true,
			},
			"user_groups": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "List of user groups names which this rule applies to.",
				Required:    true,
			},
			"runbooks": schema.ListNestedAttribute{
				Required:    true,
				Description: "List of supported entity types",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The relative git path of the runbook file.",
						},
						"repository": schema.StringAttribute{
							Required:    true,
							Description: "The normalized name of the repository. E.g.: 'github.com/hoophq/runbooks'",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *runbookRulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentState runbooksRulesResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetRunbookRuleByID(currentState.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Runbook Rule",
			fmt.Sprintf("Failed reading runbook rule %v, err=%v", currentState.ID.ValueString(), err),
		)
		return
	}

	currentState.ID = types.StringValue(rule.ID)
	currentState.Name = types.StringValue(rule.Name)
	currentState.Description = types.StringValue(rule.Description)
	currentState.Connections, diags = types.ListValueFrom(ctx, types.StringType, rule.Connections)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Names",
			fmt.Sprintf("Failed to convert connection names: %v", diags),
		)
		return
	}
	currentState.UserGroups, diags = types.ListValueFrom(ctx, types.StringType, rule.UserGroups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting User Group Names",
			fmt.Sprintf("Failed to convert user group names: %v", diags),
		)
		return
	}

	currentState.Runbooks, diags = fromApiRunbookRulesItemList(rule.Runbooks)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Runbook Rule Items",
			fmt.Sprintf("Failed to convert Runbook Rule Items: %v", diags),
		)
		return
	}

	diags = resp.State.Set(ctx, currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *runbookRulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runbooksRulesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	runbookRuleItems := []hoop.RunbookRuleItem{}
	var err error
	if !plan.Runbooks.IsNull() && !plan.Runbooks.IsUnknown() {
		runbookRuleItems, err = toApiRunbookRulesItemList(plan.Runbooks)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Runbook Rule Items (create)",
				fmt.Sprintf("Failed to convert runbook rule items: %v", err),
			)
			return
		}
	}

	connectionNames, diags := convertListToStringSlice(ctx, plan.Connections)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Names",
			fmt.Sprintf("Failed to convert connection Names: %v", diags),
		)
		return
	}

	userGroups, diags := convertListToStringSlice(ctx, plan.UserGroups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting User Group Names",
			fmt.Sprintf("Failed to convert user group Names: %v", diags),
		)
		return
	}

	rule, err := r.client.CreateRunbookRule(hoop.RunbookRule{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Connections: connectionNames,
		UserGroups:  userGroups,
		Runbooks:    runbookRuleItems,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Runbook Rule",
			fmt.Sprintf("Failed creating runbook rule: %v", err),
		)
		return
	}

	plan.ID = types.StringValue(rule.ID)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *runbookRulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runbooksRulesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	runbookRuleItems := []hoop.RunbookRuleItem{}
	var err error
	if !plan.Runbooks.IsNull() && !plan.Runbooks.IsUnknown() {
		runbookRuleItems, err = toApiRunbookRulesItemList(plan.Runbooks)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Runbook Rule Items (create)",
				fmt.Sprintf("Failed to convert runbook rule items: %v", err),
			)
			return
		}
	}

	connectionNames, diags := convertListToStringSlice(ctx, plan.Connections)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection Names",
			fmt.Sprintf("Failed to convert connection Names: %v", diags),
		)
		return
	}

	userGroups, diags := convertListToStringSlice(ctx, plan.UserGroups)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting User Group Names",
			fmt.Sprintf("Failed to convert user group Names: %v", diags),
		)
		return
	}

	_, err = r.client.UpdateRunbookRuleByID(hoop.RunbookRule{
		ID:          plan.ID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Connections: connectionNames,
		UserGroups:  userGroups,
		Runbooks:    runbookRuleItems,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Runbook Rule",
			fmt.Sprintf("Failed to update runbook rule: %v", err),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *runbookRulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runbooksRulesResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRunbookRuleByID(state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Runbook Rule",
			fmt.Sprintf("Failed to delete runbook rule: %v", err),
		)
		return
	}
}

func (r *runbookRulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Configure adds the provider configured client to the data source.
func (r *runbookRulesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func toApiRunbookRulesItemList(items types.List) ([]hoop.RunbookRuleItem, error) {
	runbookRuleItems := []hoop.RunbookRuleItem{}
	for _, elem := range items.Elements() {
		// Convert the value to an object
		obj, ok := elem.(types.Object)
		if !ok {
			return nil, fmt.Errorf("failed to convert runbook rule item to types.Object, found=%T", elem)
		}
		attrs := obj.Attributes()
		nameVal, ok := attrs["name"]
		if !ok {
			return nil, fmt.Errorf("missing 'name' attribute in runbook rule item")
		}
		repositoryVal, ok := attrs["repository"]
		if !ok {
			return nil, fmt.Errorf("missing 'repository' attribute in runbook rule item")

		}
		name, ok := nameVal.(types.String)
		if !ok {
			return nil, fmt.Errorf("failed to convert 'name' attribute to string, found=%T", nameVal)
		}
		repository, ok := repositoryVal.(types.String)
		if !ok {
			return nil, fmt.Errorf("failed to convert 'repository' attribute to string, found=%T", repositoryVal)
		}

		runbookRuleItems = append(runbookRuleItems, hoop.RunbookRuleItem{
			Name:       name.ValueString(),
			Repository: repository.ValueString(),
		})
	}

	return runbookRuleItems, nil
}

func fromApiRunbookRulesItemList(runbookRuleItems []hoop.RunbookRuleItem) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	objectType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":       types.StringType,
			"repository": types.StringType,
		},
	}

	// If the slice is empty, return an empty list
	if len(runbookRuleItems) == 0 {
		emptyList, d := types.ListValue(objectType, []attr.Value{})
		diags.Append(d...)
		return emptyList, diags
	}

	// Convert each API entry to a Terraform object
	var objectValues []attr.Value
	for _, entry := range runbookRuleItems {
		// Create the object with name and entity_types attributes
		objectValue, d := types.ObjectValue(
			map[string]attr.Type{
				"name":       types.StringType,
				"repository": types.StringType,
			},
			map[string]attr.Value{
				"name":       types.StringValue(entry.Name),
				"repository": types.StringValue(entry.Repository),
			},
		)
		if d.HasError() {
			diags.Append(d...)
			continue
		}
		objectValues = append(objectValues, objectValue)
	}

	// Create the final list of objects
	resultList, d := types.ListValue(objectType, objectValues)
	diags.Append(d...)
	return resultList, diags
}
