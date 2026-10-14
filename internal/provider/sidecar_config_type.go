// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// sidecarConfigType is the attribute type of hoop_sidecar.configuration: a
// JSON object compared the way the gateway stores it.
//
// The gateway decodes the document into its own struct, where every field
// omits its zero value, and answers that struct on each read. A configuration
// that spells out `"log_level": ""` or `"agentic": false` comes back without
// those keys, and a plain JSON comparison would plan a change on every run.
// This type drops zero-valued leaves from both sides before comparing, so the
// state keeps the operator's document while the gateway holds the canonical
// one. Both describe the same sidecar.
type sidecarConfigType struct {
	basetypes.StringType
}

var (
	_ basetypes.StringTypable                    = sidecarConfigType{}
	_ basetypes.StringValuableWithSemanticEquals = sidecarConfigValue{}
	_ xattr.ValidateableAttribute                = sidecarConfigValue{}
)

func (t sidecarConfigType) Equal(o attr.Type) bool {
	_, ok := o.(sidecarConfigType)
	return ok
}

func (t sidecarConfigType) String() string { return "SidecarConfigType" }

func (t sidecarConfigType) ValueFromString(_ context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return sidecarConfigValue{StringValue: in}, nil
}

func (t sidecarConfigType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	v, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	s, ok := v.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", v)
	}
	return sidecarConfigValue{StringValue: s}, nil
}

func (t sidecarConfigType) ValueType(context.Context) attr.Value {
	return sidecarConfigValue{}
}

type sidecarConfigValue struct {
	basetypes.StringValue
}

func newSidecarConfigValue(s string) sidecarConfigValue {
	return sidecarConfigValue{StringValue: basetypes.NewStringValue(s)}
}

func newSidecarConfigNull() sidecarConfigValue {
	return sidecarConfigValue{StringValue: basetypes.NewStringNull()}
}

func (v sidecarConfigValue) Equal(o attr.Value) bool {
	other, ok := o.(sidecarConfigValue)
	return ok && v.StringValue.Equal(other.StringValue)
}

func (v sidecarConfigValue) Type(context.Context) attr.Type {
	return sidecarConfigType{}
}

// ValidateAttribute refuses anything but a JSON object. The gateway stores an
// object and nothing else, and a string that is valid JSON but not an object
// would otherwise fail at apply with a less direct message.
func (v sidecarConfigValue) ValidateAttribute(_ context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v.ValueString()), &doc); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Sidecar Configuration",
			fmt.Sprintf("The configuration must be a JSON object: %v", err))
	}
}

// StringSemanticEquals reports whether two documents describe the same
// stored configuration: equal after key order, whitespace and zero-valued
// leaves are set aside.
func (v sidecarConfigValue) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	other, ok := newValuable.(sidecarConfigValue)
	if !ok {
		diags.AddError("Semantic Equality Check Error",
			fmt.Sprintf("Expected sidecarConfigValue, got %T. Please report this issue to the provider developers.", newValuable))
		return false, diags
	}
	if v.IsNull() || v.IsUnknown() || other.IsNull() || other.IsUnknown() {
		return v.StringValue.Equal(other.StringValue), diags
	}
	a, err := canonicalSidecarConfig([]byte(v.ValueString()))
	if err != nil {
		return false, diags
	}
	b, err := canonicalSidecarConfig([]byte(other.ValueString()))
	if err != nil {
		return false, diags
	}
	return a == b, diags
}

// canonicalSidecarConfig renders a document the way the gateway would store
// it: zero-valued leaves gone, keys sorted, no whitespace.
func canonicalSidecarConfig(raw []byte) (string, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	pruned, _ := pruneZero(doc, "")
	out, err := json.Marshal(pruned)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// pruneZero removes what the sidecar's encoder omits and reports whether
// what is left is itself zero. Objects lose zero-valued members; arrays keep
// every element, since an encoder drops a slice, not its items. The one
// exception is a `rules` key: the sidecar config keeps an empty rules list as
// the explicit opt-out it is (sidecar/CLAUDE.md, omitzero on rules), so [] under
// that key is not zero.
func pruneZero(v any, key string) (any, bool) {
	switch x := v.(type) {
	case nil:
		return nil, true
	case bool:
		return x, !x
	case float64:
		return x, x == 0
	case string:
		return x, x == ""
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			pruned, _ := pruneZero(item, "")
			out = append(out, pruned)
		}
		return out, len(out) == 0 && key != "rules"
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			pruned, zero := pruneZero(item, k)
			if zero {
				continue
			}
			out[k] = pruned
		}
		return out, len(out) == 0
	}
	return v, false
}
