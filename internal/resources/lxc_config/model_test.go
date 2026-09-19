// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- versionGateDiagnostics ------------------------------------------------

// TestVersionGateDiagnostics_BelowFloor verifies the exact diagnostic
// emitted for a probed version below TrueNAS 26.0, including the 25.10 string
// actually observed live (see model.go's lxcConfigAPI doc comment).
func TestVersionGateDiagnostics_BelowFloor(t *testing.T) {
	for _, version := range []string{"25.10.3.1", "25.10", "24.10.2", "1.0", ""} {
		diags := versionGateDiagnostics(version)
		if !diags.HasError() {
			t.Fatalf("version %q: expected an error diagnostic, got none", version)
		}
		if len(diags) != 1 {
			t.Fatalf("version %q: expected exactly 1 diagnostic, got %d: %v", version, len(diags), diags)
		}
		if diags[0].Summary() != "TrueNAS version too old" {
			t.Errorf("version %q: summary = %q, want %q", version, diags[0].Summary(), "TrueNAS version too old")
		}
		if diags[0].Detail() != "truenas_lxc_config requires TrueNAS 26.0 or later" {
			t.Errorf("version %q: detail = %q, want the documented message", version, diags[0].Detail())
		}
	}
}

// TestVersionGateDiagnostics_AtOrAboveFloor verifies no diagnostic is
// emitted for the exact 26.0 floor and releases above it, including the
// live-probed 26.0 beta string (see model.go's lxcConfigAPI doc comment).
func TestVersionGateDiagnostics_AtOrAboveFloor(t *testing.T) {
	for _, version := range []string{"26.0.0", "26.0.0-BETA.2", "26.0", "26.1.0", "27.0.0"} {
		diags := versionGateDiagnostics(version)
		if diags.HasError() {
			t.Errorf("version %q: unexpected error diagnostic: %v", version, diags)
		}
		if len(diags) != 0 {
			t.Errorf("version %q: expected 0 diagnostics, got %d: %v", version, len(diags), diags)
		}
	}
}

// --- updatePayload ----------------------------------------------------------

// TestUpdatePayload_AllSet verifies updatePayload includes every field with
// the exact wire keys lxc.update accepts (probed live).
func TestUpdatePayload_AllSet(t *testing.T) {
	m := &LXCConfigModel{
		PreferredPool: types.StringValue("tank"),
		Bridge:        types.StringValue("br0"),
		V4Network:     types.StringValue("172.200.0.0/24"),
		V6Network:     types.StringValue("fd42:4c58:43ae::/64"),
	}

	p := m.updatePayload()

	if p["preferred_pool"] != "tank" {
		t.Errorf("preferred_pool = %#v, want \"tank\"", p["preferred_pool"])
	}
	if p["bridge"] != "br0" {
		t.Errorf("bridge = %#v, want \"br0\"", p["bridge"])
	}
	if p["v4_network"] != "172.200.0.0/24" {
		t.Errorf("v4_network = %#v, want \"172.200.0.0/24\"", p["v4_network"])
	}
	if p["v6_network"] != "fd42:4c58:43ae::/64" {
		t.Errorf("v6_network = %#v, want \"fd42:4c58:43ae::/64\"", p["v6_network"])
	}
	if len(p) != 4 {
		t.Errorf("payload has %d keys (%v), want 4", len(p), p)
	}
}

// TestUpdatePayload_UnknownOmitted verifies unknown preferred_pool/bridge/
// v4_network/v6_network are omitted entirely, leaving the current
// TrueNAS-side value unchanged rather than overwriting it.
func TestUpdatePayload_UnknownOmitted(t *testing.T) {
	m := &LXCConfigModel{
		PreferredPool: types.StringUnknown(),
		Bridge:        types.StringUnknown(),
		V4Network:     types.StringUnknown(),
		V6Network:     types.StringUnknown(),
	}

	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// TestUpdatePayload_PreferredPoolThreeWay verifies the three-way nullable
// convention for preferred_pool: unknown omits, null sends JSON nil (clears
// the pool), and a known value sends the string — mirroring docker_config's
// "pool" field. This resource's docker-pool safety rule (never set/change
// preferred_pool from the acceptance test) makes this unit coverage the only
// place the null-clear path is exercised.
func TestUpdatePayload_PreferredPoolThreeWay(t *testing.T) {
	t.Run("unknown omits", func(t *testing.T) {
		m := &LXCConfigModel{PreferredPool: types.StringUnknown()}
		p := m.updatePayload()
		if _, present := p["preferred_pool"]; present {
			t.Errorf("preferred_pool should be omitted when unknown, got %#v", p["preferred_pool"])
		}
	})
	t.Run("null sends nil", func(t *testing.T) {
		m := &LXCConfigModel{PreferredPool: types.StringNull()}
		p := m.updatePayload()
		v, present := p["preferred_pool"]
		if !present {
			t.Fatal("preferred_pool should be present (explicit null) when the model holds null")
		}
		if v != nil {
			t.Errorf("preferred_pool = %#v, want nil", v)
		}
	})
	t.Run("value sends string", func(t *testing.T) {
		m := &LXCConfigModel{PreferredPool: types.StringValue("tank")}
		p := m.updatePayload()
		if p["preferred_pool"] != "tank" {
			t.Errorf("preferred_pool = %#v, want \"tank\"", p["preferred_pool"])
		}
	})
}

// TestUpdatePayload_BridgeThreeWay verifies the three-way nullable
// convention for bridge, mirroring TestUpdatePayload_PreferredPoolThreeWay.
func TestUpdatePayload_BridgeThreeWay(t *testing.T) {
	t.Run("unknown omits", func(t *testing.T) {
		m := &LXCConfigModel{Bridge: types.StringUnknown()}
		p := m.updatePayload()
		if _, present := p["bridge"]; present {
			t.Errorf("bridge should be omitted when unknown, got %#v", p["bridge"])
		}
	})
	t.Run("null sends nil", func(t *testing.T) {
		m := &LXCConfigModel{Bridge: types.StringNull()}
		p := m.updatePayload()
		v, present := p["bridge"]
		if !present {
			t.Fatal("bridge should be present (explicit null) when the model holds null")
		}
		if v != nil {
			t.Errorf("bridge = %#v, want nil", v)
		}
	})
	t.Run("value sends string", func(t *testing.T) {
		m := &LXCConfigModel{Bridge: types.StringValue("br0")}
		p := m.updatePayload()
		if p["bridge"] != "br0" {
			t.Errorf("bridge = %#v, want \"br0\"", p["bridge"])
		}
	})
}

// TestUpdatePayload_V4V6NetworkNullOmitted verifies v4_network/v6_network
// use a plain (not three-way) guard: null behaves the same as unknown
// (omitted), since lxc.config never returns null for either field.
func TestUpdatePayload_V4V6NetworkNullOmitted(t *testing.T) {
	m := &LXCConfigModel{
		PreferredPool: types.StringUnknown(),
		Bridge:        types.StringUnknown(),
		V4Network:     types.StringNull(),
		V6Network:     types.StringNull(),
	}
	p := m.updatePayload()
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (null v4/v6_network omitted like unknown)", len(p), p)
	}
}

// --- responseToModel / responseToDataSourceModel ----------------------------

// TestResponseToModel verifies responseToModel against the shape probed
// from a live lxc.config call on TrueNAS 26.0.
func TestResponseToModel(t *testing.T) {
	pool := "tank"
	bridge := "br0"
	api := &lxcConfigAPI{
		ID:            1,
		PreferredPool: &pool,
		Bridge:        &bridge,
		V4Network:     "172.200.0.0/24",
		V6Network:     "fd42:4c58:43ae::/64",
	}

	m := &LXCConfigModel{}
	responseToModel(api, m)

	if m.ID.ValueString() != lxcConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), lxcConfigResourceID)
	}
	if m.PreferredPool.ValueString() != "tank" {
		t.Errorf("PreferredPool = %q, want \"tank\"", m.PreferredPool.ValueString())
	}
	if m.Bridge.ValueString() != "br0" {
		t.Errorf("Bridge = %q, want \"br0\"", m.Bridge.ValueString())
	}
	if m.V4Network.ValueString() != "172.200.0.0/24" {
		t.Errorf("V4Network = %q, want \"172.200.0.0/24\"", m.V4Network.ValueString())
	}
	if m.V6Network.ValueString() != "fd42:4c58:43ae::/64" {
		t.Errorf("V6Network = %q, want \"fd42:4c58:43ae::/64\"", m.V6Network.ValueString())
	}
}

// TestResponseToModel_NilPreferredPoolAndBridgeBecomeNull verifies a nil
// *string from the API (both probed live: lxc.config's preferred_pool and
// bridge are null on an unconfigured LXC install) maps to a null
// types.String, not an empty string.
func TestResponseToModel_NilPreferredPoolAndBridgeBecomeNull(t *testing.T) {
	api := &lxcConfigAPI{
		ID:        1,
		V4Network: "172.200.0.0/24",
		V6Network: "fd42:4c58:43ae::/64",
	}

	m := &LXCConfigModel{}
	responseToModel(api, m)

	if !m.PreferredPool.IsNull() {
		t.Errorf("PreferredPool = %#v, want null", m.PreferredPool)
	}
	if !m.Bridge.IsNull() {
		t.Errorf("Bridge = %#v, want null", m.Bridge)
	}
}

// TestResponseToDataSourceModel verifies responseToDataSourceModel maps the
// same shape as responseToModel.
func TestResponseToDataSourceModel(t *testing.T) {
	api := &lxcConfigAPI{
		ID:        1,
		V4Network: "172.200.0.0/24",
		V6Network: "fd42:4c58:43ae::/64",
	}

	m := &LXCConfigDataSourceModel{}
	responseToDataSourceModel(api, m)

	if m.ID.ValueString() != lxcConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), lxcConfigResourceID)
	}
	if !m.PreferredPool.IsNull() {
		t.Errorf("PreferredPool = %#v, want null", m.PreferredPool)
	}
	if !m.Bridge.IsNull() {
		t.Errorf("Bridge = %#v, want null", m.Bridge)
	}
	if m.V4Network.ValueString() != "172.200.0.0/24" {
		t.Errorf("V4Network = %q, want \"172.200.0.0/24\"", m.V4Network.ValueString())
	}
	if m.V6Network.ValueString() != "fd42:4c58:43ae::/64" {
		t.Errorf("V6Network = %q, want \"fd42:4c58:43ae::/64\"", m.V6Network.ValueString())
	}
}

// --- buildUpdatePayload (resource.go) ---------------------------------------
//
// buildUpdatePayload is what makes updatePayload's split CALLERS MUST
// contract (see model.go) real: v4_network/v6_network must come from
// req.Config (fixing the same stale-echo bug webshare_config/twofactor_auth/
// audit_config had), while preferred_pool/bridge must stay Plan-sourced to
// preserve their three-way clear/leave-unchanged/set semantics, which
// req.Config cannot express (see the doc comment for why). These tests pin
// that split directly, independent of any live client or the Terraform
// framework's request plumbing.

// TestBuildUpdatePayload_V4V6SourcedFromConfigNotPlan verifies that an
// unconfigured v4_network/v6_network (null in config) is omitted from the
// payload even when plan carries a stale non-null value forward via
// UseStateForUnknown -- the exact scenario that would silently resend a
// prior value on every apply if these two fields were sourced from plan.
func TestBuildUpdatePayload_V4V6SourcedFromConfigNotPlan(t *testing.T) {
	plan := &LXCConfigModel{
		PreferredPool: types.StringNull(),
		Bridge:        types.StringNull(),
		V4Network:     types.StringValue("172.200.0.0/24"), // stale, carried forward by UseStateForUnknown
		V6Network:     types.StringValue("fd42:4c58:43ae::/64"),
	}
	config := &LXCConfigModel{
		PreferredPool: types.StringNull(),
		Bridge:        types.StringNull(),
		V4Network:     types.StringNull(), // user never configured it
		V6Network:     types.StringNull(),
	}

	p := buildUpdatePayload(plan, config)
	if _, ok := p["v4_network"]; ok {
		t.Errorf(`payload contains "v4_network" = %v, want omitted (config-sourced, unconfigured)`, p["v4_network"])
	}
	if _, ok := p["v6_network"]; ok {
		t.Errorf(`payload contains "v6_network" = %v, want omitted (config-sourced, unconfigured)`, p["v6_network"])
	}
}

// TestBuildUpdatePayload_V4V6IncludedWhenExplicitlyConfigured verifies the
// config-driven guard does not suppress a value the user actually set.
func TestBuildUpdatePayload_V4V6IncludedWhenExplicitlyConfigured(t *testing.T) {
	plan := &LXCConfigModel{
		PreferredPool: types.StringNull(),
		Bridge:        types.StringNull(),
		V4Network:     types.StringValue("172.200.0.0/24"),
		V6Network:     types.StringValue("fd42:4c58:43ae::/64"),
	}
	config := &LXCConfigModel{
		PreferredPool: types.StringNull(),
		Bridge:        types.StringNull(),
		V4Network:     types.StringValue("172.200.0.0/24"),
		V6Network:     types.StringValue("fd42:4c58:43ae::/64"),
	}

	p := buildUpdatePayload(plan, config)
	if p["v4_network"] != "172.200.0.0/24" {
		t.Errorf(`payload["v4_network"] = %v, want "172.200.0.0/24"`, p["v4_network"])
	}
	if p["v6_network"] != "fd42:4c58:43ae::/64" {
		t.Errorf(`payload["v6_network"] = %v, want "fd42:4c58:43ae::/64"`, p["v6_network"])
	}
}

// TestBuildUpdatePayload_PreferredPoolBridgeSourcedFromPlanNotConfig
// verifies that preferred_pool/bridge are read from plan, NOT config: an
// explicit clear (plan null, e.g. the user wrote preferred_pool = null and
// Plan's UseStateForUnknown modifier left it as the explicitly-configured
// null since the framework's own default planned value was already known)
// must still send JSON nil even though config alone could never distinguish
// that from "unconfigured" — proving buildUpdatePayload does NOT overwrite
// these two fields with config's (necessarily ambiguous) value.
func TestBuildUpdatePayload_PreferredPoolBridgeSourcedFromPlanNotConfig(t *testing.T) {
	plan := &LXCConfigModel{
		PreferredPool: types.StringNull(), // explicit clear, per Plan's disambiguation
		Bridge:        types.StringValue("br0"),
		V4Network:     types.StringNull(),
		V6Network:     types.StringNull(),
	}
	// config is indistinguishable from "unconfigured" for preferred_pool
	// (both null) -- if buildUpdatePayload sourced preferred_pool/bridge
	// from config too, this test would still pass for preferred_pool by
	// coincidence but fail for bridge, since config never saw "br0" at all.
	config := &LXCConfigModel{
		PreferredPool: types.StringNull(),
		Bridge:        types.StringNull(),
		V4Network:     types.StringNull(),
		V6Network:     types.StringNull(),
	}

	p := buildUpdatePayload(plan, config)
	v, ok := p["preferred_pool"]
	if !ok {
		t.Fatal(`expected "preferred_pool" present (explicit clear), got omitted`)
	}
	if v != nil {
		t.Errorf(`payload["preferred_pool"] = %v, want nil (explicit clear)`, v)
	}
	if p["bridge"] != "br0" {
		t.Errorf(`payload["bridge"] = %v, want "br0" (sourced from plan, not config)`, p["bridge"])
	}
}

// --- deleteWarningDiagnostics -----------------------------------------------

// TestDeleteWarningDiagnostics verifies Delete's diagnostic builder returns
// exactly one warning and never touches a client.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "LXC configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}
