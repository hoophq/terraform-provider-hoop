// Copyright (c) HashiCorp, Inc.

package hoop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Sidecar is a sidecar the control plane serves a configuration to. The
// gateway answers the same shape on every read and write, the one-time token
// excepted.
type Sidecar struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	// Configuration is the daemon config document as stored. The gateway
	// decodes it with unknown fields refused and drops every zero value, so
	// the document it returns is the canonical form of what was sent.
	Configuration json.RawMessage `json:"configuration,omitempty"`

	// BoundRules names the rules the control plane folds into the served
	// document, and the listener each one lands on. The rule APIs own them;
	// this is a read-only view. BoundRulesUnavailable true means the gateway
	// could not read them, and an empty BoundRules then means unknown.
	BoundRules            []SidecarRuleBinding `json:"bound_rules,omitempty"`
	BoundRulesUnavailable bool                 `json:"bound_rules_unavailable,omitempty"`

	// DetachedRules is set only on the PATCH answer that flipped
	// load_from_disk. See the type.
	DetachedRules *SidecarDetachedRules `json:"detached_rules,omitempty"`

	// The fields below come from the sidecar's handshakes and the gateway's
	// clock. No write sets them.
	Version         string `json:"version,omitempty"`
	LastSeenAt      string `json:"last_seen_at,omitempty"`
	ConfigState     string `json:"config_state,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	IdentityIssuer  string `json:"identity_issuer,omitempty"`
	IdentitySubject string `json:"identity_subject,omitempty"`
}

// SidecarRuleBinding is one rule a sidecar enforces: its kind (guardrail,
// datamasking or analyzer), its name, and the listener it is bound to. An
// empty ListenerName is a binding to every listener of the sidecar.
type SidecarRuleBinding struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	ListenerName string `json:"listener_name"`
}

// SidecarDetachedRules is what an owner switch (a load_from_disk flip) did to
// the rules bound to a sidecar. Deleted rules were imported from this
// sidecar's file and nothing else used them; unbound rules stay for their
// other targets. The gateway answers it on the PATCH that switched.
type SidecarDetachedRules struct {
	Deleted SidecarRuleNames `json:"deleted"`
	Unbound SidecarRuleNames `json:"unbound"`
}

// SidecarRuleNames names rules by feature.
type SidecarRuleNames struct {
	Guardrails  []string `json:"guardrails"`
	DataMasking []string `json:"data_masking"`
	Analyzers   []string `json:"analyzers"`
}

// Empty reports whether the switch touched no rule.
func (d *SidecarDetachedRules) Empty() bool {
	return d == nil || (d.Deleted.count()+d.Unbound.count()) == 0
}

func (n SidecarRuleNames) count() int {
	return len(n.Guardrails) + len(n.DataMasking) + len(n.Analyzers)
}

// SidecarCreated is the answer to a create. Token is the bearer the sidecar
// sends in hoop-sidecar-token; the gateway stores a hash and never shows it
// again.
type SidecarCreated struct {
	Sidecar
	Token string `json:"token"`
}

type sidecarCreateRequest struct {
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration,omitempty"`
}

type sidecarConfigurationRequest struct {
	Configuration json.RawMessage `json:"configuration"`
}

// SidecarSlackChannels is the wire form of GET and PUT
// /sidecars/{name}/slack-channels: one entry per listener, with the Slack
// channel ids its reviews go to.
type SidecarSlackChannels struct {
	Listeners []SidecarListenerSlackChannels `json:"listeners"`
}

type SidecarListenerSlackChannels struct {
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

func sidecarPath(nameOrID string) string {
	return "/sidecars/" + url.PathEscape(nameOrID)
}

// CreateSidecar registers a sidecar. A nil configuration stores an empty
// document, and the sidecar then refuses to start until one is set.
func (c *Client) CreateSidecar(name string, configuration json.RawMessage) (*SidecarCreated, error) {
	body, err := json.Marshal(sidecarCreateRequest{Name: name, Configuration: configuration})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sidecar, reason=%v", err)
	}
	req, err := c.newRequest("POST", "/sidecars", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusCreated)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out SidecarCreated
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding sidecar resource, reason=%v", err)
	}
	return &out, nil
}

// GetSidecar reads a sidecar by name or id.
func (c *Client) GetSidecar(nameOrID string) (*Sidecar, error) {
	req, err := c.newRequest("GET", sidecarPath(nameOrID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSidecar(resp.Body)
}

// UpdateSidecarConfiguration replaces the stored document. The gateway refuses
// a replace that flips load_from_disk; PatchSidecarConfiguration is the call
// for that.
func (c *Client) UpdateSidecarConfiguration(nameOrID string, configuration json.RawMessage) (*Sidecar, error) {
	return c.writeSidecarConfiguration("PUT", nameOrID, configuration)
}

// PatchSidecarConfiguration merges patch into the stored document. Only the
// keys present change. It is the one write that may change load_from_disk, and
// a patch setting it to false must carry nothing else.
func (c *Client) PatchSidecarConfiguration(nameOrID string, patch json.RawMessage) (*Sidecar, error) {
	return c.writeSidecarConfiguration("PATCH", nameOrID, patch)
}

func (c *Client) writeSidecarConfiguration(method, nameOrID string, configuration json.RawMessage) (*Sidecar, error) {
	if len(configuration) == 0 {
		configuration = json.RawMessage("{}")
	}
	body, err := json.Marshal(sidecarConfigurationRequest{Configuration: configuration})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sidecar configuration, reason=%v", err)
	}
	req, err := c.newRequest(method, sidecarPath(nameOrID), body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSidecar(resp.Body)
}

// DeleteSidecar removes a sidecar. The gateway also records the name, so a
// service account identity that reached it cannot create it again until an
// admin clears the name.
func (c *Client) DeleteSidecar(nameOrID string) error {
	req, err := c.newRequest("DELETE", sidecarPath(nameOrID), nil)
	if err != nil {
		return err
	}
	return c.sendDiscard(req, http.StatusNoContent)
}

// GetSidecarSlackChannels reads the Slack channels of each listener that has
// some. A listener with none is absent from the map.
func (c *Client) GetSidecarSlackChannels(nameOrID string) (map[string][]string, error) {
	req, err := c.newRequest("GET", sidecarPath(nameOrID)+"/slack-channels", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSlackChannels(resp.Body)
}

// SetSidecarSlackChannels writes the channels of the listeners in channels and
// leaves every other listener as it is. An entry with no channels clears that
// listener. A caller that owns the whole set sends an empty entry for each
// listener it stopped naming.
func (c *Client) SetSidecarSlackChannels(nameOrID string, channels map[string][]string) (map[string][]string, error) {
	payload := SidecarSlackChannels{Listeners: []SidecarListenerSlackChannels{}}
	for name, ids := range channels {
		if ids == nil {
			ids = []string{}
		}
		payload.Listeners = append(payload.Listeners, SidecarListenerSlackChannels{Name: name, Channels: ids})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sidecar slack channels, reason=%v", err)
	}
	req, err := c.newRequest("PUT", sidecarPath(nameOrID)+"/slack-channels", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(req, http.StatusOK)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeSlackChannels(resp.Body)
}

func decodeSidecar(body io.Reader) (*Sidecar, error) {
	var out Sidecar
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding sidecar resource, reason=%v", err)
	}
	return &out, nil
}

func decodeSlackChannels(body io.Reader) (map[string][]string, error) {
	var out SidecarSlackChannels
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed decoding sidecar slack channels, reason=%v", err)
	}
	channels := make(map[string][]string, len(out.Listeners))
	for _, l := range out.Listeners {
		if len(l.Channels) == 0 {
			continue
		}
		channels[l.Name] = l.Channels
	}
	return channels, nil
}
