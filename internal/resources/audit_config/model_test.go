// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// --- updatePayload -------------------------------------------------------
//
// updatePayload's CALLERS MUST contract (see model.go) requires callers to
// invoke it on a model populated from req.Config, never req.Plan: for an
// Optional+Computed field the user never set in HCL, req.Config leaves it
// null, while req.Plan (via UseStateForUnknown) echoes the prior state's
// value. The tests below therefore exercise the config-driven shapes —
// null-in-config means omitted, explicitly-set means included — mirroring
// the webshare_config/twofactor_auth precedent.

// TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every
// field with the exact keys observed in the audit.update probe.
func TestUpdatePayload_AllFieldsSet(t *testing.T) {
	m := &AuditConfigModel{
		Retention:         types.Int64Value(14),
		Reservation:       types.Int64Value(1),
		Quota:             types.Int64Value(2),
		QuotaFillWarning:  types.Int64Value(60),
		QuotaFillCritical: types.Int64Value(90),
	}

	p := m.updatePayload()

	want := map[string]any{
		"retention":           int64(14),
		"reservation":         int64(1),
		"quota":               int64(2),
		"quota_fill_warning":  int64(60),
		"quota_fill_critical": int64(90),
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if len(p) != 5 {
		t.Errorf("payload has %d keys (%v), want 5", len(p), p)
	}
}

// TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is
// omitted when null/unknown, so the current TrueNAS-side value is left
// unchanged rather than overwritten with a zero value.
func TestUpdatePayload_UnsetOptionalsOmitted(t *testing.T) {
	m := &AuditConfigModel{
		Retention:         types.Int64Null(),
		Reservation:       types.Int64Unknown(),
		Quota:             types.Int64Null(),
		QuotaFillWarning:  types.Int64Unknown(),
		QuotaFillCritical: types.Int64Null(),
	}

	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// TestUpdatePayload_RetentionOnly verifies the shape updatePayload actually
// sees on the real Create/Update path when the caller passes a model built
// from req.Config: an HCL config that sets only "retention" leaves
// "reservation", "quota", "quota_fill_warning", and "quota_fill_critical"
// null in config — NOT Unknown (Unknown never occurs in req.Config;
// Terraform resolves config to either a concrete value or null before the
// provider ever sees it). This is what makes the config-vs-plan sourcing
// fix meaningful: on Update, a plan-sourced model would carry the prior
// state's values for the four unconfigured fields (UseStateForUnknown),
// resending them every apply, while a config-sourced model correctly omits
// them here.
func TestUpdatePayload_RetentionOnly(t *testing.T) {
	m := &AuditConfigModel{
		Retention:         types.Int64Value(21),
		Reservation:       types.Int64Null(),
		Quota:             types.Int64Null(),
		QuotaFillWarning:  types.Int64Null(),
		QuotaFillCritical: types.Int64Null(),
	}
	p := m.updatePayload()

	if p["retention"] != int64(21) {
		t.Errorf("payload[%q] = %v, want 21", "retention", p["retention"])
	}
	for _, k := range []string{"reservation", "quota", "quota_fill_warning", "quota_fill_critical"} {
		if _, ok := p[k]; ok {
			t.Errorf("payload contains %q = %v, want omitted", k, p[k])
		}
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestResponseToModel verifies responseToModel against the shape probed
// from a live audit.config call.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()
	api := &auditConfigAPI{
		ID:                   1,
		Retention:            7,
		Reservation:          0,
		Quota:                0,
		QuotaFillWarning:     75,
		QuotaFillCritical:    95,
		RemoteLoggingEnabled: false,
		Space: spaceAPI{
			Used:              1875968,
			UsedByDataset:     1875968,
			UsedByReservation: 0,
			UsedBySnapshots:   0,
			Available:         28895973376,
		},
		EnabledServices: enabledServicesAPI{
			Middleware: []string{},
			SMB:        []string{},
			SUDO:       []string{},
		},
	}

	m := &AuditConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != auditConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), auditConfigResourceID)
	}
	if m.Retention.ValueInt64() != 7 {
		t.Errorf("Retention = %v, want 7", m.Retention.ValueInt64())
	}
	if m.QuotaFillWarning.ValueInt64() != 75 {
		t.Errorf("QuotaFillWarning = %v, want 75", m.QuotaFillWarning.ValueInt64())
	}
	if m.QuotaFillCritical.ValueInt64() != 95 {
		t.Errorf("QuotaFillCritical = %v, want 95", m.QuotaFillCritical.ValueInt64())
	}
	if m.RemoteLoggingEnabled.ValueBool() {
		t.Error("RemoteLoggingEnabled = true, want false")
	}

	var space SpaceModel
	diags = m.Space.As(ctx, &space, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back space: %v", diags)
	}
	if space.Available.ValueInt64() != 28895973376 {
		t.Errorf("space.available = %v, want 28895973376", space.Available.ValueInt64())
	}

	var enabledServices EnabledServicesModel
	diags = m.EnabledServices.As(ctx, &enabledServices, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back enabled_services: %v", diags)
	}
	var middleware []string
	diags = enabledServices.Middleware.ElementsAs(ctx, &middleware, false)
	if diags.HasError() {
		t.Fatalf("reading back enabled_services.middleware: %v", diags)
	}
	if len(middleware) != 0 {
		t.Errorf("enabled_services.middleware = %v, want empty", middleware)
	}
}

// TestResponseToModel_NilServiceListsBecomeEmptyLists verifies that nil
// []string slices from the API map to empty (non-null) lists, matching the
// nil-guard convention used elsewhere for API-returned lists.
func TestResponseToModel_NilServiceListsBecomeEmptyLists(t *testing.T) {
	ctx := context.Background()
	api := &auditConfigAPI{
		ID: 1,
		EnabledServices: enabledServicesAPI{
			Middleware: nil,
			SMB:        nil,
			SUDO:       nil,
		},
	}

	m := &AuditConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.EnabledServices.IsNull() {
		t.Error("EnabledServices should not be null")
	}

	var enabledServices EnabledServicesModel
	diags = m.EnabledServices.As(ctx, &enabledServices, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back enabled_services: %v", diags)
	}
	if enabledServices.Sudo.IsNull() {
		t.Error("enabled_services.sudo should not be null when API returns nil, want empty list")
	}
	var sudo []string
	diags = enabledServices.Sudo.ElementsAs(ctx, &sudo, false)
	if diags.HasError() {
		t.Fatalf("reading back enabled_services.sudo: %v", diags)
	}
	if len(sudo) != 0 {
		t.Errorf("enabled_services.sudo = %v, want empty", sudo)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "Audit configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}
