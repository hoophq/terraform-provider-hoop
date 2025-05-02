package internal

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func ConnectionNameSchema(isResource bool) *schema.Schema {
	schema := &schema.Schema{
		Type:        schema.TypeString,
		Description: "The name of the connection. Must follow the pattern: ^[a-zA-Z0-9_]+(?:[-\\.]?[a-zA-Z0-9_]+){2,253}$",
	}

	if isResource {
		schema.Required = true
		schema.ForceNew = true
		schema.ValidateFunc = ValidateConnectionName()
	} else {
		schema.Computed = true
	}

	return schema
}

// AccessModeSchema returns the schema for access mode configuration
func AccessModeSchema(computed bool) *schema.Schema {
	s := &schema.Schema{
		Type:     schema.TypeList,
		Optional: !computed,
		Computed: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"runbook": {
					Type:     schema.TypeBool,
					Optional: !computed,
					Computed: true,
					Default:  true,
				},
				"web": {
					Type:     schema.TypeBool,
					Optional: !computed,
					Computed: true,
					Default:  true,
				},
				"native": {
					Type:     schema.TypeBool,
					Optional: !computed,
					Computed: true,
					Default:  true,
				},
			},
		},
	}

	if !computed {
		s.Default = []interface{}{
			map[string]interface{}{
				"runbook": true,
				"web":     true,
				"native":  true,
			},
		}
	}

	return s
}

// SecretsSchema returns the schema for secrets configuration
func SecretsSchema(computed bool) *schema.Schema {
	return &schema.Schema{
		Type:      schema.TypeMap,
		Required:  !computed,
		Computed:  computed,
		Sensitive: true,
		Elem: &schema.Schema{
			Type: schema.TypeString,
		},
		DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
			// If we're creating a new resource, we need to set the secrets
			if d.Id() == "" {
				return false
			}

			// If we're comparing the same values, don't trigger a change
			if old == new {
				return true
			}

			// For partial updates, don't suppress the diff
			oldSecrets, newSecrets := d.GetChange("secrets")
			if oldSecrets == nil || newSecrets == nil {
				return false
			}

			// Only trigger an update if the user explicitly changed the secrets
			// This will prevent Terraform from showing a diff when nothing actually changed
			return !d.HasChange("secrets")
		},
	}
}

// CommonConnectionSchema returns the common schema elements for both resource and data source
func CommonConnectionSchema(isResource bool) map[string]*schema.Schema {
	s := map[string]*schema.Schema{
		"name": ConnectionNameSchema(isResource),
		"type": {
			Type:     schema.TypeString,
			Optional: isResource,
			Computed: !isResource,
			Default:  getSchemaWithDefault(isResource, "database"),
		},
		"subtype": {
			Type:     schema.TypeString,
			Required: isResource,
			Computed: !isResource,
		},
		"agent_id": {
			Type:     schema.TypeString,
			Required: isResource,
			Computed: !isResource,
		},
		"secrets": {
			Type:      schema.TypeMap,
			Required:  isResource,
			Computed:  !isResource,
			Sensitive: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"access_mode": getAccessModeSchema(isResource),
		"access_schema": {
			Type:     schema.TypeBool,
			Optional: isResource,
			Computed: !isResource,
			Default:  getSchemaWithDefault(isResource, true),
		},
		"datamasking": {
			Type:     schema.TypeBool,
			Optional: isResource,
			Computed: !isResource,
			Default:  getSchemaWithDefault(isResource, false),
		},
		"redact_types": {
			Type:     schema.TypeList,
			Optional: isResource,
			Computed: !isResource,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"review_groups": {
			Type:     schema.TypeList,
			Optional: isResource,
			Computed: !isResource,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"guardrails": {
			Type:     schema.TypeList,
			Optional: isResource,
			Computed: !isResource,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"jira_template_id": {
			Type:     schema.TypeString,
			Optional: isResource,
			Computed: !isResource,
			Default:  getSchemaWithDefault(isResource, ""),
		},
		"tags": {
			Type:     schema.TypeMap,
			Optional: isResource,
			Computed: !isResource,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}

	// Add DiffSuppressFunc only for resource, not for data source
	if isResource {
		secretsSchema := s["secrets"]
		secretsSchema.DiffSuppressFunc = func(k, old, new string, d *schema.ResourceData) bool {
			// If we're creating a new resource, we need to set the secrets
			if d.Id() == "" {
				return false
			}

			// If we're comparing the same values, don't trigger a change
			if old == new {
				return true
			}

			// For partial updates, don't suppress the diff
			oldSecrets, newSecrets := d.GetChange("secrets")
			if oldSecrets == nil || newSecrets == nil {
				return false
			}

			// Only trigger an update if the user explicitly changed the secrets
			// This will prevent Terraform from showing a diff when nothing actually changed
			return !d.HasChange("secrets")
		}
	}

	return s
}

// getAccessModeSchema returns the appropriate access_mode schema based on whether it's for a resource or data source
func getAccessModeSchema(isResource bool) *schema.Schema {
	if isResource {
		return &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Computed: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"runbook": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  true,
					},
					"web": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  true,
					},
					"native": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  true,
					},
				},
			},
			DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
				if !d.GetRawConfig().GetAttr("access_mode").IsKnown() {
					return true
				}
				return false
			},
		}
	}

	return &schema.Schema{
		Type:     schema.TypeList,
		Computed: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"runbook": {
					Type:     schema.TypeBool,
					Computed: true,
				},
				"web": {
					Type:     schema.TypeBool,
					Computed: true,
				},
				"native": {
					Type:     schema.TypeBool,
					Computed: true,
				},
			},
		},
	}
}

// getSchemaWithDefault returns appropriate schema based on whether it's for a resource or data source
func getSchemaWithDefault(isResource bool, defaultValue interface{}) interface{} {
	if isResource {
		return defaultValue
	}
	return nil
}
