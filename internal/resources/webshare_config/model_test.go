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
		if diags[0].Detail() != "truenas_webshare_config requires TrueNAS SCALE 26.0 or later" {
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
