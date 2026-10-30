// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package stateupgrade holds small, schema-independent state upgraders.
//
// Terraform 1.11+ rejects prior state that carries a value for an attribute
// the current schema marks WriteOnly ("Invalid resource state upgrade ...
// returned a value for the write-only attribute"), and the whole
// configuration is blocked — plan, apply and destroy. Any attribute that was
// stored by an earlier provider version and is write-only now therefore needs
// a SchemaVersion bump plus an upgrader that removes the stored value. The
// upgraders here work on the raw JSON so they need no copy of the old schema.
package stateupgrade

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// DropAttrs returns a StateUpgrader that removes the named top-level
// attributes from the prior state and otherwise passes it through unchanged.
// Use it when attributes became write-only (or were removed) so state written
// by an older provider version can still be decoded.
func DropAttrs(attrs ...string) resource.StateUpgrader {
	return resource.StateUpgrader{
		StateUpgrader: func(_ context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
			if req.RawState == nil || len(req.RawState.JSON) == 0 {
				return
			}
			out, err := DropAttrsJSON(req.RawState.JSON, attrs...)
			if err != nil {
				resp.Diagnostics.AddError("Upgrade resource state failed", err.Error())
				return
			}
			resp.DynamicValue = &tfprotov6.DynamicValue{JSON: out}
		},
	}
}

// DropAttrsJSON removes attrs from a JSON object; exported for tests.
func DropAttrsJSON(raw []byte, attrs ...string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for _, a := range attrs {
		delete(m, a)
	}
	return json.Marshal(m)
}
