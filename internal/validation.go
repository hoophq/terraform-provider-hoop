package internal

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// ConnectionNameRegex defines the validation pattern for connection names
var ConnectionNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+(?:[-\.]?[a-zA-Z0-9_]+){2,253}$`)

// ValidateConnectionName returns a StringMatch validator for connection names
func ValidateConnectionName() func(interface{}, string) ([]string, []error) {
	return validation.StringMatch(
		ConnectionNameRegex,
		"name must contain only letters, numbers, underscores, hyphens, and dots, "+
			"start with a letter/number/underscore, and be between 2 and 253 characters long",
	)
}
