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
	_ resource.Resource                = &datamaskingRulesResource{}
	_ resource.ResourceWithImportState = &datamaskingRulesResource{}
)

// NewDatamaskingRulesResource is a helper function to simplify the provider implementation.
func NewDatamaskingRulesResource() resource.Resource {
	return &datamaskingRulesResource{}
}

// datamaskingRulesResourceModel maps the data source schema data.
type datamaskingRulesResourceModel struct {
	ID                   types.String  `tfsdk:"id"`
	Name                 types.String  `tfsdk:"name"`
	Description          types.String  `tfsdk:"description"`
	ScoreThreshold       types.Float64 `tfsdk:"score_threshold"`
	SupportedEntityTypes types.List    `tfsdk:"supported_entity_types"`
	CustomEntityTypes    types.List    `tfsdk:"custom_entity_types"`
	ConnectionIDs        types.List    `tfsdk:"connection_ids"`
}

// datamaskingRulesResource is the data source implementation.
type datamaskingRulesResource struct {
	client *hoop.Client
}

// Metadata returns the resource type name.
func (r *datamaskingRulesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_datamasking_rules"
}

// Schema defines the schema for the data source.
func (r *datamaskingRulesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Datamasking Rules resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The unique name of the data masking rule.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "The description of the data masking rule.",
				Required:    true,
				Validators:  nil,
			},
			"score_threshold": schema.Float64Attribute{
				Description: "The minimal detection score threshold for the entities to be masked.",
				Required:    true,
				Validators:  ScoreThresholdValidator,
			},
			"supported_entity_types": schema.ListNestedAttribute{
				Required:    true,
				Description: "List of supported entity types",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_types": schema.ListAttribute{
							ElementType: types.StringType,
							Required:    true,
							Description: "The registered entity types in the redact provider.",
						},
						"name": schema.StringAttribute{
							Required:    true,
							Description: "An identifier for this structure, it's used as an identifier of a collection of entities.",
						},
					},
				},
			},
			"custom_entity_types": schema.ListNestedAttribute{
				Description: "The custom entity types that this rule applies to.",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"deny_list": schema.ListAttribute{
							ElementType: types.StringType,
							Description: "List of words to be returned as PII if found.",
							Required:    true,
						},
						"name": schema.StringAttribute{
							Description: "The name of the custom entity type as uppercase.",
							Required:    true,
						},
						"regex": schema.StringAttribute{
							Description: "The regex pattern to match (python) the custom entity type.",
							Required:    true,
						},
						"score": schema.Float64Attribute{
							Description: "Detection confidence of this pattern (0.01 if very noisy, 0.6-1.0 if very specific)",
							Required:    true,
						},
					},
				},
			},
			"connection_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "List of connection IDs which this rule applies to.",
				Required:    true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *datamaskingRulesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentState datamaskingRulesResourceModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetDatamaskingRule(currentState.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Data Masking Rule",
			fmt.Sprintf("Failed reading data masking rule with ID %q: %v", currentState.ID.ValueString(), err),
		)
		return
	}

	currentState.SupportedEntityTypes, diags = fromApiSupportedEntityTypesList(rule.SupportedEntityTypes)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Supported Entity Types",
			fmt.Sprintf("Failed to convert supported entity types: %v", diags),
		)
		return
	}

	currentState.CustomEntityTypes, diags = fromApiCustomEntityTypesList(rule.CustomEntityTypes)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Custom Entity Types",
			fmt.Sprintf("Failed to convert custom entity types: %v", diags),
		)
		return
	}

	currentState.ID = types.StringValue(rule.ID)
	currentState.Name = types.StringValue(rule.Name)
	currentState.Description = types.StringValue(rule.Description)
	currentState.ScoreThreshold = types.Float64Value(ptrToFloat64(rule.ScoreThreshold))
	currentState.ConnectionIDs, diags = types.ListValueFrom(ctx, types.StringType, rule.ConnectionIDs)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection IDs",
			fmt.Sprintf("Failed to convert connection IDs: %v", diags),
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
func (r *datamaskingRulesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan datamaskingRulesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	score := plan.ScoreThreshold.ValueFloat64()

	connectionIDs, diags := convertListToStringSlice(ctx, plan.ConnectionIDs)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection IDs",
			fmt.Sprintf("Failed to convert connection IDs: %v", diags),
		)
		return
	}

	supportedEntityTypeItems := []hoop.SupportedEntityTypesEntry{}
	var err error
	if !plan.SupportedEntityTypes.IsNull() && !plan.SupportedEntityTypes.IsUnknown() {
		supportedEntityTypeItems, err = toApiSupportedEntityTypesList(ctx, plan.SupportedEntityTypes)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Supported Entity Types (create)",
				fmt.Sprintf("Failed to convert supported entity types: %v", err),
			)
			return
		}
	}
	customEntityTypeItems := []hoop.CustomEntityTypesEntry{}
	if !plan.CustomEntityTypes.IsNull() && !plan.CustomEntityTypes.IsUnknown() {
		customEntityTypeItems, err = toApiCustomEntityTypesList(ctx, plan.CustomEntityTypes)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Custom Entity Types (create)",
				fmt.Sprintf("Failed to convert custom entity types: %v", err),
			)
			return
		}
	}

	rule, err := r.client.CreateDatamaskingRule(hoop.DataMaskingRule{
		ID:                   plan.ID.ValueString(),
		Name:                 plan.Name.ValueString(),
		Description:          plan.Description.ValueString(),
		ScoreThreshold:       &score,
		ConnectionIDs:        connectionIDs,
		SupportedEntityTypes: supportedEntityTypeItems,
		CustomEntityTypes:    customEntityTypeItems,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Data Masking Rule",
			fmt.Sprintf("Failed to create data masking rule: %v", err),
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
func (r *datamaskingRulesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan datamaskingRulesResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	scoreThreshold := plan.ScoreThreshold.ValueFloat64()
	connectionIDs, diags := convertListToStringSlice(ctx, plan.ConnectionIDs)
	if diags.HasError() {
		resp.Diagnostics.AddError(
			"Error Converting Connection IDs",
			fmt.Sprintf("Failed to convert connection IDs: %v", diags),
		)
		return
	}

	supportedEntityTypeItems := []hoop.SupportedEntityTypesEntry{}
	var err error
	if !plan.SupportedEntityTypes.IsNull() && !plan.SupportedEntityTypes.IsUnknown() {
		supportedEntityTypeItems, err = toApiSupportedEntityTypesList(ctx, plan.SupportedEntityTypes)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Supported Entity Types (create)",
				fmt.Sprintf("Failed to convert supported entity types: %v", err),
			)
			return
		}
	}

	customEntityTypeItems := []hoop.CustomEntityTypesEntry{}
	if !plan.CustomEntityTypes.IsNull() && !plan.CustomEntityTypes.IsUnknown() {
		customEntityTypeItems, err = toApiCustomEntityTypesList(ctx, plan.CustomEntityTypes)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Converting Custom Entity Types (create)",
				fmt.Sprintf("Failed to convert custom entity types: %v", err),
			)
			return
		}
	}

	rule, err := r.client.UpdateDatamaskingRule(hoop.DataMaskingRule{
		ID:                   plan.ID.ValueString(),
		Name:                 plan.Name.ValueString(),
		Description:          plan.Description.ValueString(),
		ScoreThreshold:       &scoreThreshold,
		ConnectionIDs:        connectionIDs,
		SupportedEntityTypes: supportedEntityTypeItems,
		CustomEntityTypes:    customEntityTypeItems,
	})

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Data Masking Rule",
			fmt.Sprintf("Failed to update data masking rule: %v", err),
		)
		return
	}

	plan.ID = types.StringValue(rule.ID)
	plan.Name = types.StringValue(rule.Name)
	plan.Description = types.StringValue(rule.Description)
	plan.ScoreThreshold = types.Float64Value(*rule.ScoreThreshold)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *datamaskingRulesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state datamaskingRulesResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDatamaskingRule(state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Data Masking Rule",
			fmt.Sprintf("Failed to delete data masking rule with ID %q: %v", state.ID.ValueString(), err),
		)
		return
	}
}

func (r *datamaskingRulesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Configure adds the provider configured client to the data source.
func (r *datamaskingRulesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func ptrToFloat64(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

func toApiSupportedEntityTypesList(ctx context.Context, items types.List) ([]hoop.SupportedEntityTypesEntry, error) {
	supportedEntityTypes := []hoop.SupportedEntityTypesEntry{}
	for _, elem := range items.Elements() {
		// Convert the value to an object
		obj, ok := elem.(types.Object)
		if !ok {
			return nil, fmt.Errorf("failed to convert supported entity types to types.Object, found=%T", elem)
		}
		attrs := obj.Attributes()
		nameVal, ok := attrs["name"]
		if !ok {
			return nil, fmt.Errorf("missing 'name' attribute in supported entity types")
		}
		entityTypesVal, ok := attrs["entity_types"]
		if !ok {
			return nil, fmt.Errorf("missing 'entity_types' attribute in supported entity types")

		}
		name, ok := nameVal.(types.String)
		if !ok {
			return nil, fmt.Errorf("failed to convert 'name' attribute to string, found=%T", nameVal)
		}
		entityTypes, ok := entityTypesVal.(types.List)
		if !ok {
			return nil, fmt.Errorf("failed to convert 'entity_types' attribute to list, found=%T", entityTypesVal)
		}

		var entityTypesSlice []string
		diags := entityTypes.ElementsAs(ctx, &entityTypesSlice, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert entity types, entity-types=%#v: %v", entityTypesSlice, diags)
		}

		supportedEntityTypes = append(supportedEntityTypes, hoop.SupportedEntityTypesEntry{
			Name:        name.ValueString(),
			EntityTypes: entityTypesSlice,
		})
	}

	return supportedEntityTypes, nil
}

func fromApiSupportedEntityTypesList(supportedEntityItems []hoop.SupportedEntityTypesEntry) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	// If the slice is empty, return an empty list
	if len(supportedEntityItems) == 0 {
		emptyList, d := types.ListValue(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":         types.StringType,
					"entity_types": types.ListType{ElemType: types.StringType},
				},
			},
			[]attr.Value{},
		)
		diags.Append(d...)
		return emptyList, diags
	}

	// Define the object type for supported entity types
	objectType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":         types.StringType,
			"entity_types": types.ListType{ElemType: types.StringType},
		},
	}

	// Convert each API entry to a Terraform object
	var objectValues []attr.Value
	for _, entry := range supportedEntityItems {
		// Convert entity types slice to types.List
		var entityTypeValues []attr.Value
		for _, entityType := range entry.EntityTypes {
			entityTypeValues = append(entityTypeValues, types.StringValue(entityType))
		}

		entityTypesList, d := types.ListValue(types.StringType, entityTypeValues)
		if d.HasError() {
			diags.Append(d...)
			continue
		}

		// Create the object with name and entity_types attributes
		objectValue, d := types.ObjectValue(
			map[string]attr.Type{
				"name":         types.StringType,
				"entity_types": types.ListType{ElemType: types.StringType},
			},
			map[string]attr.Value{
				"name":         types.StringValue(entry.Name),
				"entity_types": entityTypesList,
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

func toApiCustomEntityTypesList(ctx context.Context, items types.List) ([]hoop.CustomEntityTypesEntry, error) {
	customEntityTypes := []hoop.CustomEntityTypesEntry{}
	for _, elem := range items.Elements() {
		// Convert the value to an object
		obj, ok := elem.(types.Object)
		if !ok {
			return nil, fmt.Errorf("failed to convert supported entity types to types.Object, found=%T", elem)
		}
		attrs := obj.Attributes()
		name, ok := attrs["name"].(types.String)
		if !ok {
			return nil, fmt.Errorf("missing 'name' attribute in supported entity types")
		}
		regexStr, ok := attrs["regex"].(types.String)
		if !ok {
			return nil, fmt.Errorf("missing 'regex' attribute in supported entity types")
		}
		scoreThreshold, ok := attrs["score"].(types.Float64)
		if !ok {
			return nil, fmt.Errorf("missing 'score' attribute in supported entity types")
		}
		denyList, ok := attrs["deny_list"].(types.List)
		if !ok {
			return nil, fmt.Errorf("missing 'deny_list' attribute in supported entity types")
		}

		var denyListSlice []string
		if !denyList.IsNull() && !denyList.IsUnknown() {
			diags := denyList.ElementsAs(ctx, &denyListSlice, false)
			if diags.HasError() {
				return nil, fmt.Errorf("failed to convert deny list: %v", diags)
			}
		}
		customEntityTypes = append(customEntityTypes, hoop.CustomEntityTypesEntry{
			Name:     name.ValueString(),
			Regex:    regexStr.ValueString(),
			Score:    scoreThreshold.ValueFloat64(),
			DenyList: denyListSlice,
		})
	}

	return customEntityTypes, nil
}

func fromApiCustomEntityTypesList(customEntityItems []hoop.CustomEntityTypesEntry) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	// If the slice is empty, return an empty list
	if len(customEntityItems) == 0 {
		emptyList, d := types.ListValue(
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"name":      types.StringType,
					"regex":     types.StringType,
					"score":     types.Float64Type,
					"deny_list": types.ListType{ElemType: types.StringType},
				},
			},
			[]attr.Value{},
		)
		diags.Append(d...)
		return emptyList, diags
	}

	// Define the object type for supported entity types
	objectType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":      types.StringType,
			"regex":     types.StringType,
			"score":     types.Float64Type,
			"deny_list": types.ListType{ElemType: types.StringType},
		},
	}

	// Convert each API entry to a Terraform object
	var objectValues []attr.Value
	for _, entry := range customEntityItems {
		// Convert deny list slice to types.List
		var denyListValues []attr.Value
		for _, denyItem := range entry.DenyList {
			denyListValues = append(denyListValues, types.StringValue(denyItem))
		}

		denyList, d := types.ListValue(types.StringType, denyListValues)
		if d.HasError() {
			diags.Append(d...)
			continue
		}

		// Create the object with name and entity_types attributes
		objectValue, d := types.ObjectValue(
			map[string]attr.Type{
				"name":      types.StringType,
				"regex":     types.StringType,
				"score":     types.Float64Type,
				"deny_list": types.ListType{ElemType: types.StringType},
			},
			map[string]attr.Value{
				"name":      types.StringValue(entry.Name),
				"regex":     types.StringValue(entry.Regex),
				"score":     types.Float64Value(entry.Score),
				"deny_list": denyList,
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
