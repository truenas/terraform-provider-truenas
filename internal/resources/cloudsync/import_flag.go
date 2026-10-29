// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// importedKey is the private-state key ImportState sets: attributes in state
// is then the full server document, not a configured value.
const importedKey = "imported"

// privateSetter is the SetKey method of a response's private state.
type privateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// setImported sets (ImportState) or clears (Create, Update) the imported flag.
func setImported(ctx context.Context, p privateSetter, imported bool) diag.Diagnostics {
	if imported {
		return p.SetKey(ctx, importedKey, []byte("true"))
	}
	return p.SetKey(ctx, importedKey, nil)
}

// keepImportedIfEquivalent plans the state value of attributes after an
// import when the configured document is contained in the imported server
// document (server-added keys and spellings are not a change; B-118).
// It applies only while the imported flag is set: once Terraform has written
// the value, state holds the configured document, and keeping it would hide
// the removal of a key from the configuration. Terraform accepts a planned
// value that differs from config when it equals the prior state.
type keepImportedIfEquivalent struct{}

func (keepImportedIfEquivalent) Description(context.Context) string {
	return "After an import, keeps the state value when the configured JSON is contained in it."
}

func (m keepImportedIfEquivalent) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepImportedIfEquivalent) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if req.ConfigValue.ValueString() == req.StateValue.ValueString() || req.Private == nil {
		return
	}
	flag, diags := req.Private.GetKey(ctx, importedKey)
	resp.Diagnostics.Append(diags...)
	if len(flag) == 0 {
		return
	}
	if normalize.JSONSubsetOf(req.ConfigValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}
