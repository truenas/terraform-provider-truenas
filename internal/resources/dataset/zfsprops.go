// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// zfsSourced is one ZFS property as pool.dataset.get_instance reports it: its
// human value ("OFF", "128K", "1"), its numeric parsed form, and where the
// value comes from. LOCAL and RECEIVED values belong to this dataset;
// INHERITED and DEFAULT values are not explicitly set here.
type zfsSourced struct {
	Value  *string         `json:"value"`
	Parsed json.RawMessage `json:"parsed"`
	Source string          `json:"source"`
}

func isExplicitSource(source string) bool {
	return source == "LOCAL" || source == "RECEIVED"
}

// parsedInt decodes the parsed field as an int64, tolerating either a JSON
// number or a numeric string — pool.dataset.get_instance is not consistent
// about which it uses for byte-valued properties.
func (z zfsSourced) parsedInt() (int64, bool) {
	if len(z.Parsed) == 0 || string(z.Parsed) == "null" {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(z.Parsed, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(z.Parsed, &s); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}

// localStringOrInherit preserves explicit LOCAL or RECEIVED enum values.
// Only inherited/default values read back as INHERIT.
func localStringOrInherit(z zfsSourced) types.String {
	if isExplicitSource(z.Source) && z.Value != nil {
		return types.StringValue(*z.Value)
	}
	return types.StringValue("INHERIT")
}

// localString preserves explicit LOCAL or RECEIVED strings, otherwise null.
func localString(z zfsSourced) types.String {
	if isExplicitSource(z.Source) && z.Value != nil {
		return types.StringValue(*z.Value)
	}
	return types.StringNull()
}

// localInt preserves explicit LOCAL or RECEIVED integers, including zero.
func localInt(z zfsSourced) types.Int64 {
	if isExplicitSource(z.Source) {
		if n, ok := z.parsedInt(); ok {
			return types.Int64Value(n)
		}
	}
	return types.Int64Null()
}

// putEnum adds an uppercase enum property to a create/update payload when set.
func putEnum(p map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = strings.ToUpper(v.ValueString())
	}
}

// putStr adds a string property verbatim when set (e.g. recordsize "128K").
func putStr(p map[string]any, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueString()
	}
}

// putInt adds an integer property when set. Unlike volsize, there is no zero
// guard: 0 only ever reaches here when the property was set LOCAL (see
// localInt), so it is a real value to send, not a stand-in for "unset".
func putInt(p map[string]any, key string, v types.Int64) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueInt64()
	}
}
