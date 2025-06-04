package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hoophq/terraform-provider-hoop/client"
	"github.com/hoophq/terraform-provider-hoop/models"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAccessGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAccessGroupCreate,
		ReadContext:   resourceAccessGroupRead,
		UpdateContext: resourceAccessGroupUpdate,
		DeleteContext: resourceAccessGroupDelete,
		Schema: map[string]*schema.Schema{
			"group": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The group of users that will have access to the connections",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description of the access group",
			},
			"connections": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "List of connection names that this group can access",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			// Campo computado para rastrear dependências implícitas
			"connection_dependencies": {
				Type:        schema.TypeMap,
				Optional:    true,
				Computed:    true,
				Description: "Internal field for tracking dependencies",
			},
		},
	}
}

func resourceAccessGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "access_group")

	// Get resource data
	groupName := d.Get("group").(string)
	ctx = tflog.SetField(ctx, "group_name", groupName)

	tflog.Info(ctx, "Creating access group resource")
	c := m.(*client.Client)

	// Verificar se o grupo de usuários existe, se não, criar o grupo
	existingGroups, err := c.GetUserGroups(ctx)
	if err != nil {
		tflog.Error(ctx, "Failed to get user groups", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error checking if user group exists: %v", err))
	}

	// Verificar se o grupo existe
	groupExists := false
	for _, group := range existingGroups {
		if group == groupName {
			groupExists = true
			break
		}
	}

	// Se o grupo não existe, criar
	if !groupExists {
		tflog.Info(ctx, "User group does not exist, creating it", map[string]interface{}{
			"group_name": groupName,
		})

		err := c.CreateUserGroup(ctx, groupName)
		if err != nil {
			tflog.Error(ctx, "Failed to create user group", map[string]interface{}{
				"error":      err.Error(),
				"group_name": groupName,
			})
			return diag.FromErr(fmt.Errorf("error creating user group: %v", err))
		}

		tflog.Info(ctx, "User group created successfully")
	} else {
		tflog.Info(ctx, "User group already exists, skipping creation")
	}

	// Obter as conexões e checar por possíveis referências
	connections := expandStringList(d.Get("connections").([]interface{}))

	// Detectar referências do Terraform nestes valores para dependências implícitas
	connectionDeps := detectConnectionReferences(connections)

	// Armazenar as dependências detectadas no estado
	if len(connectionDeps) > 0 {
		if err := d.Set("connection_dependencies", connectionDeps); err != nil {
			tflog.Warn(ctx, "Failed to set connection dependencies", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue mesmo com erro, pois isso é apenas otimização
		}
	}

	// Build the access group model
	accessGroup := &models.AccessGroup{
		Name:        groupName,
		Description: d.Get("description").(string),
		Connections: connections,
	}

	tflog.Debug(ctx, "Prepared access group model", map[string]interface{}{
		"name":        accessGroup.Name,
		"connections": accessGroup.Connections,
	})

	// Create the access group
	err = c.CreateAccessGroup(ctx, accessGroup)
	if err != nil {
		tflog.Error(ctx, "Failed to create access group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error creating access group: %v", err))
	}

	tflog.Info(ctx, "Successfully created access group, setting ID in state")
	d.SetId(groupName)

	return resourceAccessGroupRead(ctx, d, m)
}

func resourceAccessGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "access_group")
	ctx = tflog.SetField(ctx, "group_name", d.Id())

	tflog.Info(ctx, "Reading access group resource")
	c := m.(*client.Client)

	accessGroup, err := c.GetAccessGroup(ctx, d.Id())
	if err != nil {
		tflog.Error(ctx, "Failed to get access group from API", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "Setting access group attributes in state")

	// Set all attributes in state
	if err := d.Set("group", accessGroup.Name); err != nil {
		tflog.Error(ctx, "Error setting group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	if err := d.Set("description", accessGroup.Description); err != nil {
		tflog.Error(ctx, "Error setting description", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	if err := d.Set("connections", accessGroup.Connections); err != nil {
		tflog.Error(ctx, "Error setting connections", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(err)
	}

	// Preservar as dependências detectadas durante a criação/atualização
	// Não alterar o campo connection_dependencies aqui

	tflog.Info(ctx, "Successfully read access group resource")
	return diags
}

func resourceAccessGroupUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	ctx = tflog.SetField(ctx, "resource_type", "access_group")
	ctx = tflog.SetField(ctx, "group_name", d.Id())

	tflog.Info(ctx, "Updating access group resource")
	c := m.(*client.Client)

	// Se as conexões mudaram, verificar por novas dependências implícitas
	if d.HasChange("connections") {
		connections := expandStringList(d.Get("connections").([]interface{}))

		// Detectar referências do Terraform nestes valores
		connectionDeps := detectConnectionReferences(connections)

		// Atualizar as dependências no estado
		if len(connectionDeps) > 0 {
			if err := d.Set("connection_dependencies", connectionDeps); err != nil {
				tflog.Warn(ctx, "Failed to update connection dependencies", map[string]interface{}{
					"error": err.Error(),
				})
				// Continue mesmo com erro, pois isso é apenas otimização
			}
		}
	}

	// Build the access group model
	accessGroup := &models.AccessGroup{
		Name:        d.Id(),
		Description: d.Get("description").(string),
		Connections: expandStringList(d.Get("connections").([]interface{})),
	}

	// Record which fields are changing
	var changedFields []string
	if d.HasChange("description") {
		changedFields = append(changedFields, "description")
	}
	if d.HasChange("connections") {
		changedFields = append(changedFields, "connections")
	}

	tflog.Info(ctx, "Updating access group with changed fields", map[string]interface{}{
		"changed_fields": changedFields,
	})

	// Update the access group
	err := c.UpdateAccessGroup(ctx, accessGroup)
	if err != nil {
		tflog.Error(ctx, "Failed to update access group", map[string]interface{}{
			"error": err.Error(),
		})
		return diag.FromErr(fmt.Errorf("error updating access group: %v", err))
	}

	tflog.Info(ctx, "Access group updated successfully")

	// Read the access group again to ensure state consistency
	return resourceAccessGroupRead(ctx, d, m)
}

func resourceAccessGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	ctx = tflog.SetField(ctx, "resource_type", "access_group")

	groupName := d.Id()
	ctx = tflog.SetField(ctx, "group_name", groupName)

	tflog.Info(ctx, "Deleting access group resource")
	c := m.(*client.Client)

	// Delete the access group from the plugin
	if err := c.DeleteAccessGroup(ctx, groupName); err != nil {
		// If not found, consider it deleted
		if strings.Contains(err.Error(), "not found") {
			tflog.Debug(ctx, "Access group not found, considering delete successful", map[string]interface{}{
				"group_name": groupName,
			})
		} else {
			tflog.Error(ctx, "Failed to delete access group", map[string]interface{}{
				"error": err.Error(),
			})
			return diag.FromErr(fmt.Errorf("error deleting access group: %v", err))
		}
	}

	// Agora, depois de remover o grupo do plugin, deletar o grupo de usuários
	if err := c.DeleteUserGroup(ctx, groupName); err != nil {
		tflog.Error(ctx, "Failed to delete user group", map[string]interface{}{
			"error":      err.Error(),
			"group_name": groupName,
		})
		return diag.FromErr(fmt.Errorf("error deleting user group: %v", err))
	}

	// Clear the ID from state
	d.SetId("")

	return diags
}

// expandStringList converte uma lista de interface{} em uma lista de string
func expandStringList(list []interface{}) []string {
	vs := make([]string, 0, len(list))
	for _, v := range list {
		val, ok := v.(string)
		if ok && val != "" {
			vs = append(vs, val)
		}
	}
	return vs
}

// detectConnectionReferences analisa uma lista de strings e detecta possíveis referências a recursos do Terraform
// Retorna um mapa que pode ser usado para criar dependências implícitas
func detectConnectionReferences(connections []string) map[string]interface{} {
	// Mapa para armazenar dependências detectadas
	deps := make(map[string]interface{})

	// Padrões comuns para detectar referências de recursos no Terraform
	// Por exemplo: "${hoop_connection.example.id}" ou "${hoop_connection.example.name}"
	for _, conn := range connections {
		// Verificar se a string parece ser uma referência do Terraform
		if strings.Contains(conn, ".") &&
			(strings.Contains(conn, "hoop_connection") ||
				strings.Contains(conn, "${") ||
				strings.Contains(conn, "}")) {
			// Armazenar a referência para criar dependência implícita
			deps[conn] = true
		}
	}

	return deps
}
