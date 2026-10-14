// Copyright (c) HashiCorp, Inc.

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hoophq/terraform-provider-hoop/internal/hoop"
)

// recordedRequest is one call the fake gateway saw. The sidecar tests assert
// on bodies, because the gateway reads an absent key, an empty list and a
// null differently and only the bytes on the wire prove which one went out.
type recordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

type requestRecorder struct {
	mu   sync.Mutex
	seen []recordedRequest
}

func (r *requestRecorder) record(req *http.Request) []byte {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(strings.NewReader(string(body)))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, recordedRequest{Method: req.Method, Path: req.URL.Path, Body: body})
	return body
}

// last returns the most recent request matching method and path suffix.
func (r *requestRecorder) last(method, pathSuffix string) (recordedRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.seen) - 1; i >= 0; i-- {
		if r.seen[i].Method == method && strings.HasSuffix(r.seen[i].Path, pathSuffix) {
			return r.seen[i], true
		}
	}
	return recordedRequest{}, false
}

// writes lists the non-GET methods seen, in order, for requests whose path has
// the suffix. It is how a test proves the PATCH went out before the PUT, with
// the refresh reads around them left out.
func (r *requestRecorder) writes(pathSuffix string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, req := range r.seen {
		if req.Method != http.MethodGet && strings.HasSuffix(req.Path, pathSuffix) {
			out = append(out, req.Method)
		}
	}
	return out
}

func (r *requestRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = nil
}

// checkLastBody asserts on the JSON keys of the most recent request matching
// method and path suffix. want maps a key to its expected raw JSON, with the
// value "<absent>" meaning the key must not be present.
func checkLastBody(rec *requestRecorder, method, pathSuffix string, want map[string]string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		req, ok := rec.last(method, pathSuffix)
		if !ok {
			return fmt.Errorf("no %s request with path suffix %q was recorded", method, pathSuffix)
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(req.Body, &body); err != nil {
			return fmt.Errorf("%s %s body is not a JSON object: %v", method, req.Path, err)
		}
		for key, wantRaw := range want {
			got, present := body[key]
			if wantRaw == "<absent>" {
				if present {
					return fmt.Errorf("%s %s: key %q must be absent, got %s", method, req.Path, key, got)
				}
				continue
			}
			if !present {
				return fmt.Errorf("%s %s: key %q is absent, want %s", method, req.Path, key, wantRaw)
			}
			if !jsonEqual(got, json.RawMessage(wantRaw)) {
				return fmt.Errorf("%s %s: key %q: got %s, want %s", method, req.Path, key, got, wantRaw)
			}
		}
		return nil
	}
}

func checkWrites(rec *requestRecorder, pathSuffix string, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		got := rec.writes(pathSuffix)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			return fmt.Errorf("writes on %q: got %v, want %v", pathSuffix, got, want)
		}
		return nil
	}
}

// resetRecorder clears the recorder at the end of a step, so the next step
// asserts on its own requests only.
func resetRecorder(rec *requestRecorder) resource.TestCheckFunc {
	return func(*terraform.State) error {
		rec.reset()
		return nil
	}
}

func jsonEqual(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}

func lastPathSegment(p string) string {
	parts := strings.Split(strings.TrimSuffix(p, "/"), "/")
	return parts[len(parts)-1]
}

// fakeSidecarGateway serves /sidecars and /sidecars/{nameOrID}/slack-channels
// the way the gateway does: a one-time token on create, the stored document
// echoed with load_from_disk inside it, PATCH as a merge, PUT as a replace
// that refuses a load_from_disk flip, and slack channels as a per-listener
// merge where an empty list deletes.
type fakeSidecarGateway struct {
	t        *testing.T
	rec      *requestRecorder
	mu       sync.Mutex
	byID     map[string]*hoop.Sidecar
	channels map[string]map[string][]string
	nextID   int
	license  fakeLicense
}

// bind records a rule binding the way the rule API would, so the next read
// of the sidecar reports it under bound_rules.
func (g *fakeSidecarGateway) bind(sidecarID string, b hoop.SidecarRuleBinding) {
	g.mu.Lock()
	defer g.mu.Unlock()
	sc := g.byID[sidecarID]
	sc.BoundRules = append(sc.BoundRules, b)
}

func newFakeSidecarGateway(t *testing.T) *fakeSidecarGateway {
	return &fakeSidecarGateway{
		t:        t,
		rec:      &requestRecorder{},
		byID:     map[string]*hoop.Sidecar{},
		channels: map[string]map[string][]string{},
	}
}

func (g *fakeSidecarGateway) find(nameOrID string) *hoop.Sidecar {
	for _, sc := range g.byID {
		if sc.ID == nameOrID || sc.Name == nameOrID {
			return sc
		}
	}
	return nil
}

func docFlag(raw json.RawMessage) bool {
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	return string(doc["load_from_disk"]) == "true"
}

// view is what every answer carries: the stored row plus the handshake fields
// a live gateway derives. One shape for POST, GET, PUT and PATCH keeps the
// state Terraform writes after an apply equal to what the next refresh reads.
func view(sc *hoop.Sidecar) hoop.Sidecar {
	out := *sc
	out.Version = "1.0.0"
	out.ConfigState = "applied"
	return out
}

// storedDocument is what the gateway keeps of a document: decoded into its
// struct and re-encoded with every zero value omitted. pruneZero is the
// provider's model of that encoder; using it here makes the fake answer
// what a live gateway answered in the smoke run, and the resource-level
// assertion is that the plan stays empty.
func storedDocument(raw json.RawMessage) json.RawMessage {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return raw
	}
	pruned, _ := pruneZero(doc, "")
	out, _ := json.Marshal(pruned)
	return out
}

// fakeLicense answers /serverinfo. The zero value reports a valid Enterprise
// license, which is what every test needs unless it is about the license.
type fakeLicense struct {
	typ    string
	valid  bool
	status string
	set    bool
}

func (l *fakeLicense) serve(req *http.Request) *http.Response {
	if !strings.HasSuffix(req.URL.Path, "/serverinfo") {
		return nil
	}
	info := hoop.ServerLicenseInfo{Type: "enterprise", IsValid: true, Status: "valid"}
	if l.set {
		info = hoop.ServerLicenseInfo{Type: l.typ, IsValid: l.valid, Status: l.status}
	}
	return httpTestOk(http.StatusOK, hoop.ServerInfo{LicenseInfo: &info})
}

func (g *fakeSidecarGateway) handle(req *http.Request) (*http.Response, error) {
	body := g.rec.record(req)
	g.mu.Lock()
	defer g.mu.Unlock()

	if resp := g.license.serve(req); resp != nil {
		return resp, nil
	}
	switch {
	case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/sidecars"):
		var in struct {
			Name          string          `json:"name"`
			Configuration json.RawMessage `json:"configuration"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		if g.find(in.Name) != nil {
			return httpTestErr(http.StatusConflict, "sidecar %q already exists", in.Name), nil
		}
		g.nextID++
		cfg := in.Configuration
		if len(cfg) == 0 {
			cfg = json.RawMessage("{}")
		}
		sc := &hoop.Sidecar{ID: fmt.Sprintf("sc-%d", g.nextID), Name: in.Name, Configuration: storedDocument(cfg)}
		g.byID[sc.ID] = sc
		return httpTestOk(http.StatusCreated, hoop.SidecarCreated{Sidecar: view(sc), Token: "hsc_" + sc.ID}), nil

	case strings.HasSuffix(req.URL.Path, "/slack-channels"):
		parts := strings.Split(req.URL.Path, "/")
		sc := g.find(parts[len(parts)-2])
		if sc == nil {
			return httpTestErr(http.StatusNotFound, "sidecar not found"), nil
		}
		if req.Method == http.MethodPut {
			var in hoop.SidecarSlackChannels
			if err := json.Unmarshal(body, &in); err != nil {
				return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
			}
			if g.channels[sc.ID] == nil {
				g.channels[sc.ID] = map[string][]string{}
			}
			for _, l := range in.Listeners {
				if len(l.Channels) == 0 {
					delete(g.channels[sc.ID], l.Name)
					continue
				}
				g.channels[sc.ID][l.Name] = l.Channels
			}
		}
		out := hoop.SidecarSlackChannels{Listeners: []hoop.SidecarListenerSlackChannels{}}
		for name, ids := range g.channels[sc.ID] {
			out.Listeners = append(out.Listeners, hoop.SidecarListenerSlackChannels{Name: name, Channels: ids})
		}
		return httpTestOk(http.StatusOK, out), nil
	}

	sc := g.find(lastPathSegment(req.URL.Path))
	if sc == nil {
		return httpTestErr(http.StatusNotFound, "sidecar not found"), nil
	}
	switch req.Method {
	case http.MethodGet:
		return httpTestOk(http.StatusOK, view(sc)), nil
	case http.MethodPut:
		var in struct {
			Configuration json.RawMessage `json:"configuration"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		if docFlag(sc.Configuration) != docFlag(in.Configuration) {
			return httpTestErr(http.StatusUnprocessableEntity, "load_from_disk cannot change here: use PATCH"), nil
		}
		sc.Configuration = storedDocument(in.Configuration)
		return httpTestOk(http.StatusOK, view(sc)), nil
	case http.MethodPatch:
		var in struct {
			Configuration map[string]json.RawMessage `json:"configuration"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		var doc map[string]json.RawMessage
		_ = json.Unmarshal(sc.Configuration, &doc)
		if doc == nil {
			doc = map[string]json.RawMessage{}
		}
		for k, v := range in.Configuration {
			if k == "load_from_disk" && string(v) != "true" {
				if len(in.Configuration) != 1 {
					return httpTestErr(http.StatusUnprocessableEntity, "send load_from_disk false alone"), nil
				}
				doc = map[string]json.RawMessage{}
				continue
			}
			doc[k] = v
		}
		sc.Configuration, _ = json.Marshal(doc)
		return httpTestOk(http.StatusOK, view(sc)), nil
	case http.MethodDelete:
		delete(g.byID, sc.ID)
		delete(g.channels, sc.ID)
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	}
	return httpTestErr(http.StatusInternalServerError, "test: not implemented: %s %s", req.Method, req.URL.Path), nil
}

// fakeRuleGateway serves one rule collection keyed by id or by name. It
// stores the body it receives, applying the gateway's reading of
// sidecar_targets and sidecar_spec: absent keeps, present replaces.
type fakeRuleGateway struct {
	rec        *requestRecorder
	collection string
	keyByName  bool
	mu         sync.Mutex
	rules      map[string]map[string]json.RawMessage
	nextID     int
	license    fakeLicense
}

func newFakeRuleGateway(collection string, keyByName bool) *fakeRuleGateway {
	return &fakeRuleGateway{
		rec:        &requestRecorder{},
		collection: collection,
		keyByName:  keyByName,
		rules:      map[string]map[string]json.RawMessage{},
	}
}

func (g *fakeRuleGateway) key(rule map[string]json.RawMessage) string {
	if g.keyByName {
		var name string
		_ = json.Unmarshal(rule["name"], &name)
		return name
	}
	var id string
	_ = json.Unmarshal(rule["id"], &id)
	return id
}

// refuseNamelessTarget is the gateway's ValidateSidecarRuleTargets: a target
// with no listener name is a 422, because a listener's protocol decides which
// rules it can carry.
func refuseNamelessTarget(rule map[string]json.RawMessage) *http.Response {
	raw, ok := rule["sidecar_targets"]
	if !ok || string(raw) == "null" {
		return nil
	}
	var targets []hoop.SidecarRuleTarget
	if err := json.Unmarshal(raw, &targets); err != nil {
		return httpTestErr(http.StatusUnprocessableEntity, "malformed sidecar_targets: %v", err)
	}
	for _, t := range targets {
		if t.ListenerName == "" {
			return httpTestErr(http.StatusUnprocessableEntity,
				"a rule bound to sidecar %q names no listener; bind it to the listeners that must enforce it", t.SidecarID)
		}
	}
	return nil
}

// nullEmptyConnections does what the gateway does with a rule that names no
// connections: it answers null, not []. A resource that reads that null into
// a required list attribute plans a change on every run.
func nullEmptyConnections(rule map[string]json.RawMessage) {
	for _, key := range []string{"connection_ids", "connection_names"} {
		if v, ok := rule[key]; ok && string(v) == "[]" {
			rule[key] = json.RawMessage("null")
		}
	}
}

func (g *fakeRuleGateway) handle(req *http.Request) (*http.Response, error) {
	body := g.rec.record(req)
	g.mu.Lock()
	defer g.mu.Unlock()

	if resp := g.license.serve(req); resp != nil {
		return resp, nil
	}
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, g.collection) {
		var in map[string]json.RawMessage
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		if resp := refuseNamelessTarget(in); resp != nil {
			return resp, nil
		}
		g.nextID++
		id, _ := json.Marshal(fmt.Sprintf("rule-%d", g.nextID))
		in["id"] = id
		if _, ok := in["sidecar_targets"]; !ok {
			in["sidecar_targets"] = json.RawMessage("[]")
		}
		nullEmptyConnections(in)
		g.rules[g.key(in)] = in
		return httpTestOk(http.StatusCreated, in), nil
	}

	key := lastPathSegment(req.URL.Path)
	stored, ok := g.rules[key]
	if !ok {
		return httpTestErr(http.StatusNotFound, "%s %q not found", g.collection, key), nil
	}
	switch req.Method {
	case http.MethodGet:
		return httpTestOk(http.StatusOK, stored), nil
	case http.MethodPut:
		var in map[string]json.RawMessage
		if err := json.Unmarshal(body, &in); err != nil {
			return httpTestErr(http.StatusBadRequest, "bad body: %v", err), nil
		}
		if resp := refuseNamelessTarget(in); resp != nil {
			return resp, nil
		}
		in["id"] = stored["id"]
		// The gateway's three-way reading: a key that is absent keeps the
		// stored value, a JSON null clears, anything else replaces.
		for _, kept := range []string{"sidecar_targets", "sidecar_spec", "reviewers_groups"} {
			if _, present := in[kept]; !present {
				if v, had := stored[kept]; had {
					in[kept] = v
				}
			} else if string(in[kept]) == "null" {
				delete(in, kept)
			}
		}
		nullEmptyConnections(in)
		delete(g.rules, key)
		g.rules[g.key(in)] = in
		return httpTestOk(http.StatusOK, in), nil
	case http.MethodDelete:
		delete(g.rules, key)
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	}
	return httpTestErr(http.StatusInternalServerError, "test: not implemented: %s %s", req.Method, req.URL.Path), nil
}

const testProviderBlock = `
provider "hoop" {
  api_key = "xapi-hash"
  api_url = "http://localhost:8009/api"
}
`
