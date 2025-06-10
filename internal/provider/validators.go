// Copyright (c) HashiCorp, Inc.

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var NonEmptyListValidator = []validator.List{
	listvalidator.SizeAtLeast(1),
	listvalidator.ValueStringsAre(
		stringvalidator.LengthAtLeast(1),
	),
}

var NonEmptyMapValidator = []validator.Map{
	mapvalidator.SizeAtLeast(1),
	mapvalidator.ValueStringsAre(
		stringvalidator.LengthAtLeast(1),
	),
}

var AccessModeValidator = []validator.String{
	stringvalidator.OneOf("enabled", "disabled"),
}

var ConnectionTypeValidator = []validator.String{
	stringvalidator.OneOf("database", "application", "custom"),
}

var PluginNameValidator = []validator.String{
	stringvalidator.OneOf("slack", "webhooks", "runbooks", "access_control"),
}

var NonEmptyStringValidator = []validator.String{
	stringvalidator.LengthAtLeast(1),
}
