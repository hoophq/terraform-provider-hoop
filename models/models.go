package models

type Connection struct {
	Name                string                 `json:"name"`
	Type                string                 `json:"type"`
	Subtype             string                 `json:"subtype,omitempty"`
	AgentID             string                 `json:"agent_id,omitempty"`
	ResourceType        string                 `json:"resource_type,omitempty"`
	Config              map[string]interface{} `json:"config,omitempty"`
	Labels              map[string]string      `json:"labels,omitempty"`
	ID                  string                 `json:"id,omitempty"`
	Secret              map[string]string      `json:"secret"`
	AccessModeRunbooks  string                 `json:"access_mode_runbooks"`
	AccessModeExec      string                 `json:"access_mode_exec"`
	AccessModeConnect   string                 `json:"access_mode_connect"`
	AccessSchema        string                 `json:"access_schema"`
	RedactEnabled       bool                   `json:"redact_enabled"`
	RedactTypes         []string               `json:"redact_types"`
	Reviewers           []string               `json:"reviewers"`
	GuardrailRules      []string               `json:"guardrail_rules"`
	JiraIssueTemplateID string                 `json:"jira_issue_template_id,omitempty"`
	Tags                map[string]string      `json:"connection_tags"`
}

type AccessGroup struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Connections []string `json:"connections"`
}

type Plugin struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Connections []PluginConnection `json:"connections,omitempty"`
	Config      interface{}        `json:"config"`
	Source      interface{}        `json:"source"`
	Priority    int                `json:"priority"`
	Installed   bool               `json:"installed?"`
}

type PluginConnection struct {
	ID     string   `json:"id"`
	Name   string   `json:"name,omitempty"`
	Config []string `json:"config,omitempty"`
}

type PluginConfig struct {
	ID      string            `json:"id"`
	Envvars map[string]string `json:"envvars"`
}
