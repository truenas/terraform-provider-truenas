// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every
// field with the exact keys observed in the auth.twofactor.update probe.
func TestUpdatePayload_AllFieldsSet(t *testing.T) {
	ctx := context.Background()
	services, diags := types.ObjectValueFrom(ctx, servicesAttrTypes, ServicesModel{SSH: types.BoolValue(true)})
	if diags.HasError() {
		t.Fatalf("building services object: %v", diags)
	}
	m := &TwoFactorAuthModel{
		Enabled:  types.BoolValue(true),
		Window:   types.Int64Value(30),
		Services: services,
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["enabled"] != true {
		t.Errorf("payload[%q] = %v, want true", "enabled", p["enabled"])
	}
	if p["window"] != int64(30) {
		t.Errorf("payload[%q] = %v, want 30", "window", p["window"])
	}
	svc, ok := p["services"].(map[string]bool)
	if !ok {
		t.Fatalf("payload[%q] = %T, want map[string]bool", "services", p["services"])
	}
	if !svc["ssh"] {
		t.Errorf("payload services.ssh = %v, want true", svc["ssh"])
	}
	if len(p) != 3 {
		t.Errorf("payload has %d keys (%v), want 3", len(p), p)
	}
}

// TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is
// omitted when null (the real-world req.Config shape for an attribute the
// user never set in HCL), so the current TrueNAS-side value is left
// unchanged rather than overwritten with a zero value. This is the
// mechanism the committed acceptance test relies on to mutate "window"
// without ever touching "enabled".
func TestUpdatePayload_UnsetOptionalsOmitted(t *testing.T) {
	m := &TwoFactorAuthModel{
		Enabled:  types.BoolNull(),
		Window:   types.Int64Null(),
		Services: types.ObjectNull(servicesAttrTypes),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// TestUpdatePayload_WindowOnly verifies the shape updatePayload actually
// sees on the real Create/Update path when the caller passes a model built
// from req.Config (as resource.go now does): the acceptance test's HCL
// sets only "window", so "enabled" and "services" are null in config —
// NOT Unknown. (Unknown never occurs in req.Config: Terraform resolves
// config to either a concrete value or null before the provider ever sees
// it; Unknown only ever appears in the plan, e.g. via a
// UseStateForUnknown-populated echo of the prior state for an attribute
// the user didn't configure — which is exactly the shape that must NOT
// drive payload inclusion, per updatePayload's doc comment.) So this test
// asserts "enabled" and "services" are omitted and "window" is included.
func TestUpdatePayload_WindowOnly(t *testing.T) {
	m := &TwoFactorAuthModel{
		Enabled:  types.BoolNull(),
		Window:   types.Int64Value(60),
		Services: types.ObjectNull(servicesAttrTypes),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["enabled"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "enabled", p["enabled"])
	}
	if _, ok := p["services"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "services", p["services"])
	}
	if p["window"] != int64(60) {
		t.Errorf("payload[%q] = %v, want 60", "window", p["window"])
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_EnabledExplicitlySet verifies that a user who DOES set
// "enabled" in their HCL config still gets it included in the payload:
// the config-driven guard in updatePayload must not suppress
// explicitly-configured values, only unconfigured (null-in-config) ones.
func TestUpdatePayload_EnabledExplicitlySet(t *testing.T) {
	m := &TwoFactorAuthModel{
		Enabled:  types.BoolValue(true),
		Window:   types.Int64Null(),
		Services: types.ObjectNull(servicesAttrTypes),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if p["enabled"] != true {
		t.Errorf("payload[%q] = %v, want true", "enabled", p["enabled"])
	}
	if _, ok := p["window"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "window", p["window"])
	}
	if _, ok := p["services"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "services", p["services"])
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestResponseToModel verifies responseToModel against the shape probed
// from a live auth.twofactor.config call (identical on TrueNAS 25.10 and
// 26.0).
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()
	api := &twoFactorAuthAPI{
		ID:      1,
		Enabled: false,
		Window:  0,
		Services: servicesAPI{
			SSH: false,
		},
	}

	m := &TwoFactorAuthModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != twoFactorAuthResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), twoFactorAuthResourceID)
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled = true, want false")
	}
	if m.Window.ValueInt64() != 0 {
		t.Errorf("Window = %v, want 0", m.Window.ValueInt64())
	}

	var services ServicesModel
	diags = m.Services.As(ctx, &services, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back services: %v", diags)
	}
	if services.SSH.ValueBool() {
		t.Error("services.ssh = true, want false")
	}
}

// TestResponseToModel_EnabledAndSSHTrue verifies the mapping also handles
// the "everything true" shape, matching what the SAFETY VERIFICATION probe
// observed after auth.twofactor.update({"enabled": true}).
func TestResponseToModel_EnabledAndSSHTrue(t *testing.T) {
	ctx := context.Background()
	api := &twoFactorAuthAPI{
		ID:      1,
		Enabled: true,
		Window:  60,
		Services: servicesAPI{
			SSH: true,
		},
	}

	m := &TwoFactorAuthModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled = false, want true")
	}
	if m.Window.ValueInt64() != 60 {
		t.Errorf("Window = %v, want 60", m.Window.ValueInt64())
	}

	var services ServicesModel
	diags = m.Services.As(ctx, &services, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back services: %v", diags)
	}
	if !services.SSH.ValueBool() {
		t.Error("services.ssh = false, want true")
	}
}

// TestResponseToDataSourceModel verifies the datasource mapping mirrors
// responseToModel.
func TestResponseToDataSourceModel(t *testing.T) {
	ctx := context.Background()
	api := &twoFactorAuthAPI{
		ID:      1,
		Enabled: false,
		Window:  0,
		Services: servicesAPI{
			SSH: false,
		},
	}

	m := &TwoFactorAuthDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != twoFactorAuthResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), twoFactorAuthResourceID)
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled = true, want false")
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable,
// which matters here specifically because Delete must never disable 2FA.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "Two-factor authentication configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}
