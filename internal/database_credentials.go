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

// ValidateCredentials checks if all required credentials are present
func ValidateCredentials(config map[string]interface{}, subtype string) error {
	// Handle custom connection type
	if subtype == "custom" {
		// Custom connections have flexible credentials format
		// No validation needed for custom connections
		return nil
	}

	// For database connections, validate specific credentials
	var missingFields []string

	fields, ok := databaseCredentials[subtype]
	if !ok {
		return fmt.Errorf("unsupported database type: %s", subtype)
	}

	// Check required fields and collect missing ones
	for _, field := range fields {
		value, exists := config[field.key]

		if field.required {
			if !exists || value == "" {
				if field.defaultValue != "" {
					config[field.key] = field.defaultValue
					continue
				}
				missingFields = append(missingFields, field.key)
			}
		}
	}

	// Add some defaults if needed
	if _, exists := config["port"]; !exists && subtype != "bigquery" {
		config["port"] = getDefaultPort(subtype)
	}

	// if we have any missing fields, build an error message
	if len(missingFields) > 0 {
		return fmt.Errorf("missing required fields for %s: %s", subtype, strings.Join(missingFields, ", "))
	}

	return nil
}

// getDefaultPort returns the default port for each database type
func getDefaultPort(dbType string) string {
	switch dbType {
	case "mysql":
		return "3306"
	case "postgresql":
		return "5432"
	case "mssql":
		return "1433"
	case "oracle":
		return "1521"
	case "mongodb":
		return "27017"
	default:
		return ""
	}
}
