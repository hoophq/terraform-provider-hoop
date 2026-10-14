// Copyright (c) HashiCorp, Inc.

package hoop

// SidecarRuleTarget is one place the control plane enforces a rule: a sidecar
// and one of its listeners. The gateway refuses a target with no listener
// name (services.ValidateSidecarRuleTargets): a listener's protocol decides
// which rules it can run, so a rule is always bound per listener.
type SidecarRuleTarget struct {
	SidecarID    string `json:"sidecar_id"`
	ListenerName string `json:"listener_name,omitempty"`
}

// SidecarTargets wraps a target list so a rule request can tell the gateway
// three different things. The gateway reads an absent sidecar_targets key as
// "keep the bindings", an empty list as "unbind everywhere", and a list as the
// new binding set. A plain slice with omitempty collapses the first two into
// one, and a resource that drops its targets would then leave the rule bound
// to the whole fleet.
//
// A nil *SidecarTargets omits the key. A non-nil one with no items sends [].
type SidecarTargets = *[]SidecarRuleTarget

// NewSidecarTargets returns the pointer form of targets. It never returns nil,
// so the key reaches the gateway even when the list is empty.
func NewSidecarTargets(targets []SidecarRuleTarget) SidecarTargets {
	if targets == nil {
		targets = []SidecarRuleTarget{}
	}
	return &targets
}

// TargetsOf unwraps a SidecarTargets for callers that only read it.
func TargetsOf(targets SidecarTargets) []SidecarRuleTarget {
	if targets == nil {
		return nil
	}
	return *targets
}
