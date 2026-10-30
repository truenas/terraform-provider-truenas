// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package planmod

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The "managed" flags behind ResetWhenUnset* live in the resource's private
// state under managedKey(attr). The framework discards private state written
// by plan modifiers during a CREATE plan (Create starts from empty private
// data and only its own response is kept), so resources must record the flags
// from Create with RecordManaged; on later plans the modifiers maintain them.

func managedKey(attr string) string { return "planmod.managed." + attr }

// RecordManaged marks every attr that is set (non-null or unknown) in the
// configuration as managed. Call it from Create, after the apply succeeded,
// with resp.Private, listing the attributes that use ResetWhenUnset* (and any
// other attribute whose "removed from config" handling depends on IsManaged).
func RecordManaged(ctx context.Context, cfg tfsdk.Config, private privateData, attrs ...string) diag.Diagnostics {
	var diags diag.Diagnostics
	if isNil(private) || cfg.Raw.IsNull() || !cfg.Raw.IsKnown() {
		return diags
	}
	for _, a := range attrs {
		v, ok := lookupPath(cfg.Raw, a)
		if !ok || (v.IsKnown() && v.IsNull()) {
			continue
		}
		diags.Append(private.SetKey(ctx, managedKey(a), []byte("1"))...)
	}
	return diags
}

// lookupPath walks a dotted attribute path ("attributes.prefix") through
// nested objects of a raw config value. A null/unknown object on the way
// means the leaf is not configured.
func lookupPath(v tftypes.Value, dotted string) (tftypes.Value, bool) {
	for _, part := range strings.Split(dotted, ".") {
		if !v.IsKnown() || v.IsNull() {
			return v, false
		}
		var obj map[string]tftypes.Value
		if err := v.As(&obj); err != nil {
			return v, false
		}
		next, ok := obj[part]
		if !ok {
			return v, false
		}
		v = next
	}
	return v, true
}

// IsManaged reports whether attr was recorded as configured.
func IsManaged(ctx context.Context, private privateData, attr string) bool {
	if isNil(private) {
		return false
	}
	raw, _ := private.GetKey(ctx, managedKey(attr))
	return len(raw) > 0
}

// SetManaged records attr as configured (used by resource-level ModifyPlan).
func SetManaged(ctx context.Context, private privateData, attr string) diag.Diagnostics {
	if isNil(private) {
		return nil
	}
	return private.SetKey(ctx, managedKey(attr), []byte("1"))
}

// ClearManaged forgets attr (after a one-time reset to the default).
func ClearManaged(ctx context.Context, private privateData, attr string) diag.Diagnostics {
	if isNil(private) {
		return nil
	}
	return private.SetKey(ctx, managedKey(attr), nil)
}
