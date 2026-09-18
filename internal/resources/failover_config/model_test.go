// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config

import (
	"context"
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

// --- responseToModel -------------------------------------------------------

func apiShape() *failoverConfigAPI {
	// Mirrors the live TrueNAS 25.10.4 Enterprise HA probe:
	// {"id": 1, "disabled": false, "master": true, "timeout": 0}.
	return &failoverConfigAPI{ID: 1, Disabled: false, Master: true, Timeout: 0}
}

func TestResponseToModel(t *testing.T) {
	m := &FailoverConfigModel{}
	responseToModel(apiShape(), m)
	if m.ID.ValueString() != failoverConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), failoverConfigResourceID)
	}
	if m.Disabled.ValueBool() {
		t.Error("Disabled should be false")
	}
	if !m.Master.ValueBool() {
		t.Error("Master should be true")
	}
	if m.Timeout.ValueInt64() != 0 {
		t.Errorf("Timeout = %d, want 0", m.Timeout.ValueInt64())
	}
}

func TestResponseToModel_NonDefaultValues(t *testing.T) {
	api := &failoverConfigAPI{ID: 1, Disabled: true, Master: false, Timeout: 30}
	m := &FailoverConfigModel{}
	responseToModel(api, m)
	if !m.Disabled.ValueBool() {
		t.Error("Disabled should be true")
	}
	if m.Master.ValueBool() {
		t.Error("Master should be false")
	}
	if m.Timeout.ValueInt64() != 30 {
		t.Errorf("Timeout = %d, want 30", m.Timeout.ValueInt64())
	}
}

// --- responseToDataSourceModel ---------------------------------------------

func TestResponseToDataSourceModel(t *testing.T) {
	m := &FailoverConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), apiShape(), "MASTER", "B", []string{}, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != failoverConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), failoverConfigResourceID)
	}
	if m.Status.ValueString() != "MASTER" {
		t.Errorf("Status = %q, want MASTER", m.Status.ValueString())
	}
	if m.Node.ValueString() != "B" {
		t.Errorf("Node = %q, want B", m.Node.ValueString())
	}
	if m.DisabledReasons.IsNull() {
		t.Error("DisabledReasons should be a known (empty) list, not null")
	}
	var reasons []string
	m.DisabledReasons.ElementsAs(context.Background(), &reasons, false)
	if len(reasons) != 0 {
		t.Errorf("DisabledReasons = %#v, want empty", reasons)
	}
}

func TestResponseToDataSourceModel_WithReasons(t *testing.T) {
	m := &FailoverConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), apiShape(), "ERROR", "MANUAL",
		[]string{"NO_LICENSE", "NO_VIP"}, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	var reasons []string
	m.DisabledReasons.ElementsAs(context.Background(), &reasons, false)
	if len(reasons) != 2 || reasons[0] != "NO_LICENSE" || reasons[1] != "NO_VIP" {
		t.Errorf("DisabledReasons = %#v, want [NO_LICENSE NO_VIP]", reasons)
	}
}

func TestResponseToDataSourceModel_NilReasonsIsKnownEmptyList(t *testing.T) {
	// A nil Go slice (e.g. from a zero-valued caller) must still map to a
	// known empty list, not null: failover.disabled.reasons was observed
	// live returning a present, empty array ("[]"), never an absent key.
	m := &FailoverConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), apiShape(), "SINGLE", "MANUAL", nil, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.DisabledReasons.IsNull() {
		t.Error("DisabledReasons should be a known empty list for nil input, not null")
	}
}

// --- updatePayload ---------------------------------------------------------
//
// This is the load-bearing safety-critical test group: no committed code
// path exercised by any test in this repository may ever cause
// failover.update to be called with a "disabled" or "master" value that
// differs from the box's live state. acceptance_test.go's
// TestAccFailoverConfig_setAndRestore only ever varies "timeout"; these
// unit tests independently pin the pure-function contract updatePayload's
// doc comment describes: each field is included ONLY when the model's
// corresponding field is non-null/non-unknown (i.e. only when a caller
// populated it from req.Config AND the user explicitly configured it in
// HCL).

func TestUpdatePayload_AllUnsetOmitsEverything(t *testing.T) {
	m := &FailoverConfigModel{}
	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("expected empty payload, got %#v", p)
	}
}

func TestUpdatePayload_OnlyTimeoutSet(t *testing.T) {
	m := &FailoverConfigModel{}
	m.Timeout = types.Int64Value(30)
	p := m.updatePayload()
	if len(p) != 1 {
		t.Fatalf("payload has %d keys (%v), want 1", len(p), p)
	}
	if p["timeout"] != int64(30) {
		t.Errorf(`p["timeout"] = %#v, want int64(30)`, p["timeout"])
	}
}

func TestUpdatePayload_AllFieldsSet(t *testing.T) {
	m := &FailoverConfigModel{}
	m.Disabled = types.BoolValue(true)
	m.Master = types.BoolValue(false)
	m.Timeout = types.Int64Value(10)
	p := m.updatePayload()
	if len(p) != 3 {
		t.Fatalf("payload has %d keys (%v), want 3", len(p), p)
	}
	if p["disabled"] != true {
		t.Errorf(`p["disabled"] = %#v, want true`, p["disabled"])
	}
	if p["master"] != false {
		t.Errorf(`p["master"] = %#v, want false`, p["master"])
	}
	if p["timeout"] != int64(10) {
		t.Errorf(`p["timeout"] = %#v, want int64(10)`, p["timeout"])
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
