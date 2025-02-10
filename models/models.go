package models

type Connection struct {
	Name                string            `json:"name"`
	Type                string            `json:"type"`
	Subtype             string            `json:"subtype"`
	AgentID             string            `json:"agent_id"`
	Secret              map[string]string `json:"secret"`
	AccessModeRunbooks  string            `json:"access_mode_runbooks"`
	AccessModeExec      string            `json:"access_mode_exec"`
	AccessModeConnect   string            `json:"access_mode_connect"`
	AccessSchema        string            `json:"access_schema"`
	RedactEnabled       bool              `json:"redact_enabled"`
	RedactTypes         []string          `json:"redact_types"`
	Reviewers           []string          `json:"reviewers"`
	GuardrailRules      []string          `json:"guardrail_rules"`
	JiraIssueTemplateID string            `json:"jira_issue_template_id,omitempty"`
	Tags                []string          `json:"tags"`
}
