// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- nonNilStrings -----------------------------------------------------------

func TestNonNilStrings(t *testing.T) {
	if got := nonNilStrings(nil); got == nil || len(got) != 0 {
		t.Errorf("nonNilStrings(nil) = %#v, want non-nil empty slice", got)
	}
	in := []string{"a", "b"}
	if got := nonNilStrings(in); len(got) != 2 {
		t.Errorf("nonNilStrings(%#v) = %#v, want unchanged", in, got)
	}
}

// --- stringListOrNull ----------------------------------------------------

func TestStringListOrNull_NilPointerIsNullList(t *testing.T) {
	// A nil *[]string means the JSON key was entirely absent from this
	// release's tn_connect.config response (e.g. "ips" on TrueNAS 26.0) — it
	// must map to a null list, distinct from a present-but-empty one.
	l, diags := stringListOrNull(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !l.IsNull() {
		t.Errorf("expected a null list for a nil pointer, got %#v", l)
	}
}

func TestStringListOrNull_EmptySliceIsKnownEmptyList(t *testing.T) {
	empty := []string{}
	l, diags := stringListOrNull(context.Background(), &empty)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if l.IsNull() {
		t.Error("expected a known empty list for a non-nil-but-empty pointer, not null")
	}
	var out []string
	l.ElementsAs(context.Background(), &out, false)
	if len(out) != 0 {
		t.Errorf("expected 0 elements, got %#v", out)
	}
}

func TestStringListOrNull_PopulatedSlice(t *testing.T) {
	vals := []string{"192.168.1.68", "10.0.0.1"}
	l, diags := stringListOrNull(context.Background(), &vals)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	var out []string
	l.ElementsAs(context.Background(), &out, false)
	if len(out) != 2 || out[0] != "192.168.1.68" || out[1] != "10.0.0.1" {
		t.Errorf("got %#v, want %#v", out, vals)
	}
}

// --- registrationDetailsString --------------------------------------------

func TestRegistrationDetailsString_Empty(t *testing.T) {
	if got := registrationDetailsString(nil); got.ValueString() != "{}" {
		t.Errorf("registrationDetailsString(nil) = %q, want %q", got.ValueString(), "{}")
	}
	if got := registrationDetailsString(json.RawMessage("{}")); got.ValueString() != "{}" {
		t.Errorf("registrationDetailsString({}) = %q, want %q", got.ValueString(), "{}")
	}
}

func TestRegistrationDetailsString_Populated(t *testing.T) {
	raw := json.RawMessage(`{"account_id":"abc","tier":1}`)
	got := registrationDetailsString(raw)
	if got.ValueString() != `{"account_id":"abc","tier":1}` {
		t.Errorf("registrationDetailsString(%s) = %q, want it verbatim", raw, got.ValueString())
	}
}

// --- responseToModel / responseToDataSourceModel -------------------------
//
// Two API shapes below mirror the two probed releases exactly (see model.go's
// tnConnectConfigAPI doc comment): the 25.10 shape has ips/interfaces/
// interfaces_ips/use_all_interfaces populated and tier/last_heartbeat_failure_datetime
// absent (nil); the 26.0 shape is the reverse.

func api2510Shape() *tnConnectConfigAPI {
	ips := []string{}
	interfaces := []string{}
	interfacesIPs := []string{}
	useAll := true
	return &tnConnectConfigAPI{
		ID:                           1,
		Enabled:                      false,
		Status:                       "DISABLED",
		StatusReason:                 "TrueNAS Connect is disabled",
		Certificate:                  nil,
		AccountServiceBaseURL:        "https://account-service.tys1.truenasconnect.net/",
		LecaServiceBaseURL:           "https://dns-service.tys1.truenasconnect.net/",
		TNCBaseURL:                   "https://web.truenasconnect.net/",
		HeartbeatURL:                 "https://heartbeat-service.tys1.truenasconnect.net/",
		RegistrationDetails:          json.RawMessage("{}"),
		Tier:                         nil,
		LastHeartbeatFailureDatetime: nil,
		IPs:                          &ips,
		Interfaces:                   &interfaces,
		InterfacesIPs:                &interfacesIPs,
		UseAllInterfaces:             &useAll,
	}
}

func api260Shape() *tnConnectConfigAPI {
	cert := int64(2)
	tier := "FOUNDATION"
	return &tnConnectConfigAPI{
		ID:                           1,
		Enabled:                      true,
		Status:                       "CONFIGURED",
		StatusReason:                 "TrueNAS Connect is configured",
		Certificate:                  &cert,
		AccountServiceBaseURL:        "https://account-service.tys1.truenasconnect.net/",
		LecaServiceBaseURL:           "https://dns-service.tys1.truenasconnect.net/",
		TNCBaseURL:                   "https://web.truenasconnect.net/",
		HeartbeatURL:                 "https://heartbeat-service.tys1.truenasconnect.net/",
		RegistrationDetails:          json.RawMessage(`{"account_id":"abc"}`),
		Tier:                         &tier,
		LastHeartbeatFailureDatetime: nil,
		IPs:                          nil,
		Interfaces:                   nil,
		InterfacesIPs:                nil,
		UseAllInterfaces:             nil,
	}
}

func TestResponseToModel_2510Shape(t *testing.T) {
	m := &TnConnectConfigModel{}
	diags := responseToModel(context.Background(), api2510Shape(), m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != tnConnectConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), tnConnectConfigResourceID)
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled should be false")
	}
	if m.Tier.IsNull() != true {
		t.Errorf("Tier should be null on the 25.10 shape (absent key), got %#v", m.Tier)
	}
	if m.LastHeartbeatFailureDatetime.IsNull() != true {
		t.Errorf("LastHeartbeatFailureDatetime should be null on the 25.10 shape, got %#v", m.LastHeartbeatFailureDatetime)
	}
	if m.IPs.IsNull() {
		t.Error("IPs should be a known (empty) list on the 25.10 shape, not null")
	}
	if !m.UseAllInterfaces.ValueBool() {
		t.Error("UseAllInterfaces should be true on the 25.10 shape")
	}
	if m.Certificate.IsNull() != true {
		t.Errorf("Certificate should be null (API returned null), got %#v", m.Certificate)
	}
}

func TestResponseToModel_260Shape(t *testing.T) {
	m := &TnConnectConfigModel{}
	diags := responseToModel(context.Background(), api260Shape(), m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled should be true")
	}
	if m.Tier.ValueString() != "FOUNDATION" {
		t.Errorf("Tier = %q, want FOUNDATION", m.Tier.ValueString())
	}
	if !m.IPs.IsNull() {
		t.Error("IPs should be null on the 26.0 shape (key absent), not a known list")
	}
	if !m.Interfaces.IsNull() {
		t.Error("Interfaces should be null on the 26.0 shape")
	}
	if !m.InterfacesIPs.IsNull() {
		t.Error("InterfacesIPs should be null on the 26.0 shape")
	}
	if !m.UseAllInterfaces.IsNull() {
		t.Error("UseAllInterfaces should be null on the 26.0 shape (key absent)")
	}
	if m.Certificate.ValueInt64() != 2 {
		t.Errorf("Certificate = %d, want 2", m.Certificate.ValueInt64())
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	m := &TnConnectConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api260Shape(), m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Status.ValueString() != "CONFIGURED" {
		t.Errorf("Status = %q, want CONFIGURED", m.Status.ValueString())
	}
}

// --- updatePayload ---------------------------------------------------------
//
// This is the load-bearing safety-critical test group: no committed code
// path exercised by any test in this repository may ever cause
// tn_connect.update to be called with {"enabled": true}. The committed
// acceptance test (acceptance_test.go) never calls Create/Update at all (its
// only non-permanently-skipped test is the read-only datasource test), so
// updatePayload is never invoked with a live client in the test suite in the
// first place — but these unit tests independently pin the pure-function
// contract updatePayload's doc comment describes: "enabled" is included ONLY
// when the model's Enabled field is non-null/non-unknown (i.e. only when a
// caller populated it from req.Config AND the user explicitly configured it
// in HCL), and every other field is NEVER included regardless of its value.

func TestUpdatePayload_AllUnknownOmitsEnabled(t *testing.T) {
	m := &TnConnectConfigModel{Enabled: types.BoolUnknown()}
	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("expected empty payload, got %#v", p)
	}
}

func TestUpdatePayload_NullOmitsEnabled(t *testing.T) {
	// The real-world req.Config shape for a user who never set "enabled" in
	// HCL: null, not Unknown (Unknown never occurs in req.Config).
	m := &TnConnectConfigModel{Enabled: types.BoolNull()}
	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("expected empty payload (enabled omitted), got %#v", p)
	}
}

func TestUpdatePayload_ExplicitFalseIncluded(t *testing.T) {
	m := &TnConnectConfigModel{Enabled: types.BoolValue(false)}
	p := m.updatePayload()
	if len(p) != 1 {
		t.Fatalf("payload has %d keys (%v), want 1", len(p), p)
	}
	if p["enabled"] != false {
		t.Errorf(`p["enabled"] = %#v, want false`, p["enabled"])
	}
}

func TestUpdatePayload_ExplicitTrueIncluded(t *testing.T) {
	// Confirms updatePayload's pure config-driven contract is correct in
	// BOTH directions (it must not silently swallow a real true either) —
	// this is an in-memory assertion about a Go map, not a live API call,
	// and is never invoked from any acceptance test path (see the group
	// doc comment above).
	m := &TnConnectConfigModel{Enabled: types.BoolValue(true)}
	p := m.updatePayload()
	if p["enabled"] != true {
		t.Errorf(`p["enabled"] = %#v, want true`, p["enabled"])
	}
}

func TestUpdatePayload_NeverIncludesAnyOtherField(t *testing.T) {
	// Every field besides "enabled" is Computed-only: updatePayload must
	// never include them regardless of what the model holds, since
	// tn_connect.update's own accepts schema on TrueNAS 26.0 (this resource's
	// primary target — probed live) does not even accept most of them.
	ips, _ := types.ListValueFrom(context.Background(), types.StringType, []string{"127.0.0.1"})
	m := &TnConnectConfigModel{
		Enabled:               types.BoolNull(),
		Status:                types.StringValue("CONFIGURED"),
		StatusReason:          types.StringValue("whatever"),
		Certificate:           types.Int64Value(2),
		AccountServiceBaseURL: types.StringValue("https://example.com/"),
		Tier:                  types.StringValue("FOUNDATION"),
		IPs:                   ips,
		UseAllInterfaces:      types.BoolValue(true),
	}
	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("payload should be empty when enabled is unset, got %#v (only \"enabled\" may ever be sent)", p)
	}
}

// --- needsUpdateCall -------------------------------------------------------
//
// This is what makes "an empty-config Create/Update performs a read-only
// fetchConfig, never an unprobed tn_connect.update({})" independently
// testable without a live client: Create/Update call needsUpdateCall(payload)
// on the exact map updatePayload returns, and skip applyUpdate entirely when
// it reports false.

func TestNeedsUpdateCall_EmptyPayloadIsFalse(t *testing.T) {
	m := &TnConnectConfigModel{Enabled: types.BoolNull()}
	if needsUpdateCall(m.updatePayload()) {
		t.Error("needsUpdateCall should be false for an empty payload (enabled never configured)")
	}
}

func TestNeedsUpdateCall_ExplicitFalseIsTrue(t *testing.T) {
	m := &TnConnectConfigModel{Enabled: types.BoolValue(false)}
	if !needsUpdateCall(m.updatePayload()) {
		t.Error("needsUpdateCall should be true when the user explicitly configured enabled = false")
	}
}

func TestNeedsUpdateCall_ExplicitTrueIsTrue(t *testing.T) {
	m := &TnConnectConfigModel{Enabled: types.BoolValue(true)}
	if !needsUpdateCall(m.updatePayload()) {
		t.Error("needsUpdateCall should be true when the user explicitly configured enabled = true")
	}
}

func TestNeedsUpdateCall_DirectEmptyMap(t *testing.T) {
	if needsUpdateCall(map[string]any{}) {
		t.Error("needsUpdateCall(map[string]any{}) should be false")
	}
	if needsUpdateCall(nil) {
		t.Error("needsUpdateCall(nil) should be false")
	}
	if !needsUpdateCall(map[string]any{"enabled": false}) {
		t.Error("needsUpdateCall should be true for any non-empty payload")
	}
}

// --- deleteWarningDiagnostics --------------------------------------------

func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("expected a warning, not an error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Severity().String() != "Warning" {
		t.Errorf("severity = %v, want Warning", diags[0].Severity())
	}
}
