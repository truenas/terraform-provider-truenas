// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNullUnknowns: a plan model saved as state after a failed read-back must
// hold no unknown values (B-045).
func TestNullUnknowns(t *testing.T) {
	m := VMModel{
		ID:        types.Int64Unknown(),
		Name:      types.StringValue("v"),
		Status:    types.StringUnknown(),
		Running:   types.BoolUnknown(),
		Autostart: types.BoolValue(false),
	}
	nullUnknowns(&m)
	if !m.ID.IsNull() || !m.Status.IsNull() || !m.Running.IsNull() {
		t.Errorf("unknowns not nulled: %+v", m)
	}
	if m.Name.ValueString() != "v" || m.Autostart.ValueBool() {
		t.Errorf("known values changed: %+v", m)
	}
}
