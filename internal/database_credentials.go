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

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "; "))
	}

	return nil
}
