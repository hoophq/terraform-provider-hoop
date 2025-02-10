package internal

import (
	"fmt"
	"strings"
)

// Required credential fields by database type
var databaseCredentials = map[string][]credentialField{
	"mysql": {
		{key: "host", required: true},
		{key: "user", required: true},
		{key: "pass", required: true},
		{key: "port", required: true},
		{key: "db", required: true},
	},
	"postgres": {
		{key: "host", required: true},
		{key: "user", required: true},
		{key: "pass", required: true},
		{key: "port", required: true},
		{key: "db", required: true},
		{key: "sslmode", required: false},
	},
	"mssql": {
		{key: "host", required: true},
		{key: "user", required: true},
		{key: "pass", required: true},
		{key: "port", required: true},
		{key: "db", required: true},
		{key: "insecure", required: false},
	},
	"oracledb": {
		{key: "host", required: true},
		{key: "user", required: true},
		{key: "pass", required: true},
		{key: "port", required: true},
		{key: "sid", required: true},
		{key: "ld_library_path", required: true, defaultValue: "/opt/oracle/instantclient_19_24"},
	},
	"mongodb": {
		{key: "connection_string", required: true},
	},
}

type credentialField struct {
	key          string
	required     bool
	defaultValue string
}

// ValidateCredentials checks if all required credentials are present for the given database type
func ValidateCredentials(dbType string, credentials map[string]interface{}) error {
	fields, ok := databaseCredentials[dbType]
	if !ok {
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	var missingFields []string
	var invalidFields []string

	// Check required fields and collect missing ones
	for _, field := range fields {
		value, exists := credentials[field.key]

		if field.required {
			if !exists || value == "" {
				if field.defaultValue != "" {
					credentials[field.key] = field.defaultValue
					continue
				}
				missingFields = append(missingFields, field.key)
			}
		}

		// Additional validation for specific fields
		if exists {
			if err := validateFieldValue(field.key, value); err != nil {
				invalidFields = append(invalidFields, fmt.Sprintf("%s (%s)", field.key, err))
			}
		}
	}

	// Build error message if there are any issues
	var errors []string

	if len(missingFields) > 0 {
		errors = append(errors, fmt.Sprintf(
			"missing required credentials for %s: %s",
			dbType,
			strings.Join(missingFields, ", "),
		))
	}

	if len(invalidFields) > 0 {
		errors = append(errors, fmt.Sprintf(
			"invalid credential values: %s",
			strings.Join(invalidFields, ", "),
		))
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}

	return nil
}

// validateFieldValue performs specific validations for certain field types
func validateFieldValue(key string, value interface{}) error {
	strValue, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value")
	}

	switch key {
	case "port":
		// Port should be a numeric string
		for _, c := range strValue {
			if c < '0' || c > '9' {
				return fmt.Errorf("port must be numeric")
			}
		}
	case "connection_string":
		// MongoDB connection string should start with mongodb:// or mongodb+srv://
		if !strings.HasPrefix(strValue, "mongodb://") && !strings.HasPrefix(strValue, "mongodb+srv://") {
			return fmt.Errorf("must be a valid MongoDB connection string")
		}
	case "sslmode":
		// PostgreSQL sslmode valid values
		validModes := map[string]bool{
			"disable":     true,
			"allow":       true,
			"prefer":      true,
			"require":     true,
			"verify-ca":   true,
			"verify-full": true,
			"":            true, // empty is allowed as it's optional
		}
		if !validModes[strValue] {
			return fmt.Errorf("invalid sslmode value")
		}
	case "insecure":
		// MSSQL insecure flag should be "true" or "false"
		if strValue != "true" && strValue != "false" && strValue != "" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
	}

	return nil
}
