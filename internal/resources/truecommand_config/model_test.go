// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUpdatePayload_EmptyWhenUnset verifies updatePayload returns an empty
// map when both "enabled" and "api_key" are null/unknown — the state used
// whenever a caller manages this resource purely for its read-only status
// fields.
func TestUpdatePayload_EmptyWhenUnset(t *testing.T) {
	m := &TrueCommandConfigModel{
		Enabled: types.BoolNull(),
		APIKey:  types.StringNull(),
	}
	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("updatePayload() = %v, want empty map", p)
	}
}

// TestUpdatePayload_NeverSendsEnabledTrue is a safety-critical guard: no
// path through this function's logic can ever produce {"enabled": true}
// unless the model itself already carries Enabled=true — this is the
// negative-space check that the function does no implicit escalation (e.g.
// defaulting an unknown/null value to true).
func TestUpdatePayload_NeverSendsEnabledTrue(t *testing.T) {
	cases := []*TrueCommandConfigModel{
		{Enabled: types.BoolNull(), APIKey: types.StringNull()},
		{Enabled: types.BoolUnknown(), APIKey: types.StringNull()},
		{Enabled: types.BoolValue(false), APIKey: types.StringNull()},
		{Enabled: types.BoolNull(), APIKey: types.StringValue("abcd1234abcd1234")},
	}
	for i, m := range cases {
		p := m.updatePayload()
		if v, ok := p["enabled"]; ok && v == true {
			t.Errorf("case %d: updatePayload() sent enabled=true from a model that never set it true: %v", i, p)
		}
	}
}

// TestUpdatePayload_IncludesEnabledWhenSet verifies "enabled" is included
// (and only "enabled") when set and api_key is left unset.
func TestUpdatePayload_IncludesEnabledWhenSet(t *testing.T) {
	m := &TrueCommandConfigModel{
		Enabled: types.BoolValue(false),
		APIKey:  types.StringNull(),
	}
	p := m.updatePayload()
	if len(p) != 1 {
		t.Fatalf("updatePayload() = %v, want exactly 1 key", p)
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
}

// TestUpdatePayload_IncludesAPIKeyWhenSet verifies "api_key" is included
// (and only "api_key") when set and enabled is left unset — the decisive
// probe's exact payload shape (see model.go's doc comment).
func TestUpdatePayload_IncludesAPIKeyWhenSet(t *testing.T) {
	m := &TrueCommandConfigModel{
		Enabled: types.BoolNull(),
		APIKey:  types.StringValue("abcd1234abcd1234"),
	}
	p := m.updatePayload()
	if len(p) != 1 {
		t.Fatalf("updatePayload() = %v, want exactly 1 key", p)
	}
	if p["api_key"] != "abcd1234abcd1234" {
		t.Errorf("payload[api_key] = %v, want abcd1234abcd1234", p["api_key"])
	}
}

// TestUpdatePayload_BothSet verifies both keys are included together when
// both are set.
func TestUpdatePayload_BothSet(t *testing.T) {
	m := &TrueCommandConfigModel{
		Enabled: types.BoolValue(false),
		APIKey:  types.StringValue("abcd1234abcd1234"),
	}
	p := m.updatePayload()
	if len(p) != 2 {
		t.Fatalf("updatePayload() = %v, want exactly 2 keys", p)
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
	if p["api_key"] != "abcd1234abcd1234" {
		t.Errorf("payload[api_key] = %v, want abcd1234abcd1234", p["api_key"])
	}
}

// TestResponseToModel_FullShape verifies responseToModel against the field
// shape probed live (identical on TrueNAS 25.10.4 HA and 26.0): api_key
// round-trips verbatim (unmasked), nullable fields decode to true null.
func TestResponseToModel_FullShape(t *testing.T) {
	apiKey := "abcd1234abcd1234"
	remoteURL := "https://truecommand.example.com"
	remoteIP := "203.0.113.5"
	api := &truecommandConfigAPI{
		ID:              1,
		APIKey:          &apiKey,
		Enabled:         false,
		Status:          "CONNECTED",
		StatusReason:    "Truecommand service is connected.",
		RemoteURL:       &remoteURL,
		RemoteIPAddress: &remoteIP,
	}

	m := &TrueCommandConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != trueCommandConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), trueCommandConfigResourceID)
	}
	if m.Enabled.ValueBool() != false {
		t.Errorf("Enabled = %v, want false", m.Enabled)
	}
	if m.APIKey.ValueString() != apiKey {
		t.Errorf("APIKey = %q, want it returned verbatim (not masked): %q", m.APIKey.ValueString(), apiKey)
	}
	if m.Status.ValueString() != "CONNECTED" {
		t.Errorf("Status = %q, want CONNECTED", m.Status.ValueString())
	}
	if m.RemoteURL.ValueString() != remoteURL {
		t.Errorf("RemoteURL = %q, want %q", m.RemoteURL.ValueString(), remoteURL)
	}
	if m.RemoteIPAddress.ValueString() != remoteIP {
		t.Errorf("RemoteIPAddress = %q, want %q", m.RemoteIPAddress.ValueString(), remoteIP)
	}
}

// TestResponseToModel_NullableFieldsNull verifies api_key/remote_url/
// remote_ip_address decode to true Terraform null (not empty string) when
// the API returns null, matching the probed disabled-state shape (both
// TrueNAS 25.10.4 HA and 26.0's live truecommand.config: {"api_key": null,
// "enabled": false, "remote_ip_address": null, "remote_url": null, ...}).
func TestResponseToModel_NullableFieldsNull(t *testing.T) {
	api := &truecommandConfigAPI{
		ID:              1,
		APIKey:          nil,
		Enabled:         false,
		Status:          "DISABLED",
		StatusReason:    "Truecommand service is disabled.",
		RemoteURL:       nil,
		RemoteIPAddress: nil,
	}

	m := &TrueCommandConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.APIKey.IsNull() {
		t.Errorf("APIKey = %v, want null", m.APIKey)
	}
	if !m.RemoteURL.IsNull() {
		t.Errorf("RemoteURL = %v, want null", m.RemoteURL)
	}
	if !m.RemoteIPAddress.IsNull() {
		t.Errorf("RemoteIPAddress = %v, want null", m.RemoteIPAddress)
	}
}

// TestResponseToDataSourceModel_FullShape mirrors
// TestResponseToModel_FullShape for the datasource model.
func TestResponseToDataSourceModel_FullShape(t *testing.T) {
	apiKey := "abcd1234abcd1234"
	api := &truecommandConfigAPI{
		ID:           1,
		APIKey:       &apiKey,
		Enabled:      false,
		Status:       "DISABLED",
		StatusReason: "Truecommand service is disabled.",
	}

	m := &TrueCommandConfigDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != trueCommandConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), trueCommandConfigResourceID)
	}
	if m.APIKey.ValueString() != apiKey {
		t.Errorf("APIKey = %q, want %q", m.APIKey.ValueString(), apiKey)
	}
	if !m.RemoteURL.IsNull() {
		t.Errorf("RemoteURL = %v, want null", m.RemoteURL)
	}
}
