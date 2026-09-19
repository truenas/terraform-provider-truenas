// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- versionGateDiagnostics ------------------------------------------------

func TestVersionGateDiagnostics_BelowFloor(t *testing.T) {
	for _, version := range []string{"25.10.3.1", "25.10", "24.10.2", "1.0", ""} {
		diags := versionGateDiagnostics(version)
		if !diags.HasError() {
			t.Fatalf("version %q: expected an error diagnostic, got none", version)
		}
		if len(diags) != 1 {
			t.Fatalf("version %q: expected exactly 1 diagnostic, got %d: %v", version, len(diags), diags)
		}
		if diags[0].Detail() != "truenas_webshare_config requires TrueNAS 26.0 or later" {
			t.Errorf("version %q: detail = %q, want the documented message", version, diags[0].Detail())
		}
	}
}

func TestVersionGateDiagnostics_AtOrAboveFloor(t *testing.T) {
	for _, version := range []string{"26.0.0", "26.0.0-BETA.2", "26.0", "26.1.0", "27.0.0"} {
		diags := versionGateDiagnostics(version)
		if diags.HasError() {
			t.Errorf("version %q: unexpected error diagnostic: %v", version, diags)
		}
	}
}

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

// --- responseToModel / responseToDataSourceModel ------------------------------

func fullAPI() *webshareConfigAPI {
	return &webshareConfigAPI{
		ID:      1,
		BindIP:  []string{"192.168.1.68"},
		Search:  true,
		Passkey: "REQUIRED",
		Groups:  []string{"webshare_users"},
	}
}

func TestResponseToModel(t *testing.T) {
	api := fullAPI()
	m := &WebshareConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != webshareConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), webshareConfigResourceID)
	}
	if !m.Search.ValueBool() {
		t.Error("Search should be true")
	}
	if m.Passkey.ValueString() != "REQUIRED" {
		t.Errorf("Passkey = %q, want REQUIRED", m.Passkey.ValueString())
	}

	var bindip []string
	m.BindIP.ElementsAs(context.Background(), &bindip, false)
	if len(bindip) != 1 || bindip[0] != "192.168.1.68" {
		t.Errorf("BindIP = %#v, want [192.168.1.68]", bindip)
	}

	var groups []string
	m.Groups.ElementsAs(context.Background(), &groups, false)
	if len(groups) != 1 || groups[0] != "webshare_users" {
		t.Errorf("Groups = %#v, want [webshare_users]", groups)
	}
}

func TestResponseToModel_EmptyArraysBecomeKnownEmptyLists(t *testing.T) {
	// Probed live: bindip/groups default to [] (never null/omitted) — the
	// mapped list must be a known empty list, not null, matching that.
	api := &webshareConfigAPI{ID: 1, BindIP: nil, Search: false, Passkey: "DISABLED", Groups: nil}
	m := &WebshareConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.BindIP.IsNull() {
		t.Error("BindIP should be a known empty list, not null")
	}
	if m.Groups.IsNull() {
		t.Error("Groups should be a known empty list, not null")
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := fullAPI()
	m := &WebshareConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Passkey.ValueString() != "REQUIRED" {
		t.Errorf("Passkey = %q, want REQUIRED", m.Passkey.ValueString())
	}
}

// --- updatePayload -------------------------------------------------------
//
// updatePayload's CALLERS MUST contract (see model.go) requires callers to
// invoke it on a model populated from req.Config, never req.Plan: for an
// Optional+Computed field the user never set in HCL, req.Config leaves it
// null, while req.Plan (via UseStateForUnknown) echoes the prior state's
// value. The tests below therefore exercise the config-driven shapes —
// null-in-config means omitted, explicitly-set means included — mirroring
// twofactor_auth/model_test.go's precedent, plus an Unknown case as a
// defensive mirror (Unknown never actually appears in req.Config, only in
// req.Plan, but updatePayload guards it anyway).

func TestUpdatePayload_AllUnknownOmitsEverything(t *testing.T) {
	m := &WebshareConfigModel{
		BindIP:  types.ListUnknown(types.StringType),
		Search:  types.BoolUnknown(),
		Passkey: types.StringUnknown(),
		Groups:  types.ListUnknown(types.StringType),
	}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(p) != 0 {
		t.Errorf("expected empty payload, got %#v", p)
	}
}

// TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is
// omitted when null (the real-world req.Config shape for an attribute the
// user never set in HCL), so the current TrueNAS-side value is left
// unchanged rather than overwritten with a zero value.
func TestUpdatePayload_UnsetOptionalsOmitted(t *testing.T) {
	m := &WebshareConfigModel{
		BindIP:  types.ListNull(types.StringType),
		Search:  types.BoolNull(),
		Passkey: types.StringNull(),
		Groups:  types.ListNull(types.StringType),
	}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// TestUpdatePayload_SearchOnly verifies the shape updatePayload actually
// sees on the real Create/Update path when the caller passes a model built
// from req.Config: an HCL config that sets only "search" leaves "bindip",
// "passkey", and "groups" null in config — NOT Unknown (Unknown never
// occurs in req.Config; Terraform resolves config to either a concrete
// value or null before the provider ever sees it). So this test asserts
// "bindip", "passkey", and "groups" are omitted and "search" is included,
// matching the committed acceptance test's mutate-search-only path.
func TestUpdatePayload_SearchOnly(t *testing.T) {
	m := &WebshareConfigModel{
		BindIP:  types.ListNull(types.StringType),
		Search:  types.BoolValue(true),
		Passkey: types.StringNull(),
		Groups:  types.ListNull(types.StringType),
	}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["bindip"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "bindip", p["bindip"])
	}
	if _, ok := p["passkey"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "passkey", p["passkey"])
	}
	if _, ok := p["groups"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "groups", p["groups"])
	}
	if p["search"] != true {
		t.Errorf("payload[%q] = %v, want true", "search", p["search"])
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_PasskeyExplicitlySet verifies that a user who DOES set
// "passkey" in their HCL config still gets it included in the payload: the
// config-driven guard in updatePayload must not suppress
// explicitly-configured values, only unconfigured (null-in-config) ones.
func TestUpdatePayload_PasskeyExplicitlySet(t *testing.T) {
	m := &WebshareConfigModel{
		BindIP:  types.ListNull(types.StringType),
		Search:  types.BoolNull(),
		Passkey: types.StringValue("REQUIRED"),
		Groups:  types.ListNull(types.StringType),
	}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if p["passkey"] != "REQUIRED" {
		t.Errorf("payload[%q] = %v, want REQUIRED", "passkey", p["passkey"])
	}
	if _, ok := p["search"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "search", p["search"])
	}
	if _, ok := p["bindip"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "bindip", p["bindip"])
	}
	if _, ok := p["groups"]; ok {
		t.Errorf("payload contains %q = %v, want omitted", "groups", p["groups"])
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

func TestUpdatePayload_KnownFieldsIncluded(t *testing.T) {
	bindip, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"10.0.0.1"})
	if diags.HasError() {
		t.Fatalf("building bindip: %v", diags)
	}
	groups, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"grp1"})
	if diags.HasError() {
		t.Fatalf("building groups: %v", diags)
	}

	m := &WebshareConfigModel{
		BindIP:  bindip,
		Search:  types.BoolValue(true),
		Passkey: types.StringValue("ENABLED"),
		Groups:  groups,
	}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	bindipOut, ok := p["bindip"].([]string)
	if !ok || len(bindipOut) != 1 || bindipOut[0] != "10.0.0.1" {
		t.Errorf(`p["bindip"] = %#v`, p["bindip"])
	}
	if p["search"] != true {
		t.Errorf(`p["search"] = %#v`, p["search"])
	}
	if p["passkey"] != "ENABLED" {
		t.Errorf(`p["passkey"] = %#v`, p["passkey"])
	}
	groupsOut, ok := p["groups"].([]string)
	if !ok || len(groupsOut) != 1 || groupsOut[0] != "grp1" {
		t.Errorf(`p["groups"] = %#v`, p["groups"])
	}
}

func TestUpdatePayload_NullListsSendEmptySlice(t *testing.T) {
	// A known-but-empty list (e.g. bindip = []) must round-trip as an empty
	// slice, not be dropped — ElementsAs on an empty (but known) list
	// produces a nil Go slice, which nonNilStrings-equivalent handling
	// inside updatePayload must convert back to [].
	bindip, diags := types.ListValueFrom(context.Background(), types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("building bindip: %v", diags)
	}
	m := &WebshareConfigModel{
		BindIP:  bindip,
		Search:  types.BoolUnknown(),
		Passkey: types.StringUnknown(),
		Groups:  types.ListUnknown(types.StringType),
	}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	bindipOut, ok := p["bindip"].([]string)
	if !ok || len(bindipOut) != 0 {
		t.Errorf(`p["bindip"] = %#v, want empty non-nil slice`, p["bindip"])
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
