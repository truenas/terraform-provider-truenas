// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// zfsSourced is one ZFS property as pool.dataset.get_instance reports it: its
// human value ("OFF", "128K", "1"), its numeric parsed form, and where the
// value comes from. The source is "LOCAL" when set directly on this zvol and
// "RECEIVED" when set by a ZFS replication receive — both are explicit values
// that belong to this zvol. "INHERITED" and "DEFAULT" are not set here.
type zfsSourced struct {
	Value    *string         `json:"value"`
	Parsed   json.RawMessage `json:"parsed"`
	RawValue *string         `json:"rawvalue"`
	Source   string          `json:"source"`
}

// isExplicitSource reports whether a ZFS property source represents a value
// explicitly set on this zvol (LOCAL) or stamped onto it by a replication
// receive (RECEIVED), as opposed to INHERITED/DEFAULT. A RECEIVED value is a
// real property of a restored zvol; treating it as inherited loses it and makes
// an imported replica plan a spurious replacement. (#44)
func isExplicitSource(source string) bool {
	return source == "LOCAL" || source == "RECEIVED"
}

func (z zfsSourced) isExplicit() bool { return isExplicitSource(z.Source) }

// parsedInt decodes the parsed field as an int64, tolerating either a JSON
// number or a numeric string — pool.dataset.get_instance is not consistent
// about which it uses for byte-valued properties. A reservation or
// refreservation of 0 is reported as parsed = null even when set LOCAL, so a
// null parsed falls back to rawvalue ("0"). (B-090)
func (z zfsSourced) parsedInt() (int64, bool) {
	if len(z.Parsed) == 0 || string(z.Parsed) == "null" {
		if z.RawValue != nil {
			if v, err := strconv.ParseInt(strings.TrimSpace(*z.RawValue), 10, 64); err == nil {
				return v, true
			}
		}
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

// localString records a source-aware string property's value only when it is
// set LOCAL on this dataset, and null otherwise. An inherited, default, or
// received property therefore reads back as null (unset). This is what keeps
// an Optional+Computed attribute from carrying an inherited value into state
// and re-sending it on the next update, which would silently convert an
// inherited property into a local one. The trade-off: reverting a
// locally-set property to inherited cannot be expressed by removing it from
// the configuration (the last value is kept) — revert it out of band and
// refresh.
// localStringOrInherit is localString for inheritable ENUM properties that
// expose an explicit "INHERIT" member: a LOCAL value is reported verbatim; a
// non-LOCAL property reads back as "INHERIT" so that value round-trips. (#38 class)
func localStringOrInherit(z zfsSourced) types.String {
	if z.isExplicit() && z.Value != nil {
		return types.StringValue(*z.Value)
	}
	return types.StringValue("INHERIT")
}

// localInt is localString's integer counterpart: the parsed integer when the
// property is set LOCAL, null otherwise. 0 is a real value here and is kept
// when LOCAL (the source, not a zero check, is what distinguishes "set to 0"
// from "inherited").
func localInt(z zfsSourced) types.Int64 {
	if z.isExplicit() {
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

// putInt adds an integer property when set. Unlike volsize, there is no zero
// guard: 0 only ever reaches here when the property was set LOCAL (see
// localInt), so it is a real value to send, not a stand-in for "unset".
func putInt(p map[string]any, key string, v types.Int64) {
	if !v.IsNull() && !v.IsUnknown() {
		p[key] = v.ValueInt64()
	}
}
