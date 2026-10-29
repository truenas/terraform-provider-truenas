// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNullUnknowns: a plan model saved as state after a failed read-back must
// hold no unknown values (B-045).
func TestNullUnknowns(t *testing.T) {
	m := ContainerModel{
		ID:        types.Int64Unknown(),
		Name:      types.StringValue("c"),
		UUID:      types.StringUnknown(),
		Running:   types.BoolUnknown(),
		InitEnv:   types.MapUnknown(types.StringType),
		Image:     types.ObjectNull(imageAttrTypes),
		Autostart: types.BoolValue(true),
	}
	nullUnknowns(&m)
	if !m.ID.IsNull() || !m.UUID.IsNull() || !m.Running.IsNull() || !m.InitEnv.IsNull() {
		t.Errorf("unknowns not nulled: %+v", m)
	}
	if m.Name.ValueString() != "c" || !m.Autostart.ValueBool() {
		t.Errorf("known values changed: %+v", m)
	}
}
