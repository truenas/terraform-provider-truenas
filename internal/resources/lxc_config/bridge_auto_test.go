// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestResponseToModel_BridgeAuto guards B-081: the server stores "[AUTO]" and
// "" as null. Null reads as "[AUTO]" (a configured "" is kept); a real bridge
// is read as is.
func TestResponseToModel_BridgeAuto(t *testing.T) {
	for _, v := range []string{"[AUTO]", ""} {
		m := LXCConfigModel{Bridge: types.StringValue(v)}
		responseToModel(&lxcConfigAPI{}, &m)
		if m.Bridge.ValueString() != v || m.Bridge.IsNull() {
			t.Errorf("bridge %q read back as %v", v, m.Bridge)
		}
	}
	// Import (no prior value) and a bridge removed out of band read as "[AUTO]".
	for _, prior := range []types.String{types.StringNull(), types.StringValue("br0")} {
		m := LXCConfigModel{Bridge: prior}
		responseToModel(&lxcConfigAPI{}, &m)
		if m.Bridge.ValueString() != "[AUTO]" {
			t.Errorf("prior %v with server null = %v, want [AUTO]", prior, m.Bridge)
		}
	}
	br := "br1"
	m := LXCConfigModel{Bridge: types.StringValue("[AUTO]")}
	responseToModel(&lxcConfigAPI{Bridge: &br}, &m)
	if m.Bridge.ValueString() != "br1" {
		t.Errorf("server bridge br1 read as %v", m.Bridge)
	}
}
