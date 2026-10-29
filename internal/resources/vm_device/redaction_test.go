// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"encoding/json"
	"testing"
)

// TestScrubRedacted: a READONLY_ADMIN session reads the DISPLAY password as
// "********". The mask must take the known value (no drift, nothing absorbed)
// or be dropped when no value is known (import). (B-033)
func TestScrubRedacted(t *testing.T) {
	api := map[string]any{"dtype": "DISPLAY", "type": "SPICE", "password": "********", "port": float64(5900)}
	state := map[string]any{"dtype": "DISPLAY", "type": "SPICE", "password": "secret"}
	scrubRedacted(api, state)
	if api["password"] != "secret" {
		t.Errorf("password = %v, want the state value", api["password"])
	}
	if attributesDrifted(state, api) {
		t.Error("redacted password reported as drift")
	}

	api = map[string]any{"dtype": "DISPLAY", "password": "********"}
	scrubRedacted(api, nil)
	if _, ok := api["password"]; ok {
		t.Error("redacted password kept on import")
	}
}

// TestResponseToDataSourceModel_Redacted: the data source reports a redacted
// secret as null, never as the mask. (B-033)
func TestResponseToDataSourceModel_Redacted(t *testing.T) {
	api := &deviceAPI{ID: 1, VM: 2, Attributes: map[string]any{"dtype": "DISPLAY", "password": "********"}}
	var m VMDeviceDataSourceModel
	if d := responseToDataSourceModel(api, &m); d.HasError() {
		t.Fatal(d)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &got); err != nil {
		t.Fatal(err)
	}
	if v, ok := got["password"]; !ok || v != nil {
		t.Errorf("password = %v (present %v), want null", v, ok)
	}
}
