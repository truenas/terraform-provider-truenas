// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// stringPtr is a small test helper for building *string API values.
func stringPtr(s string) *string { return &s }

// baseModel returns a fully-populated SMBConfigModel with known (non-null,
// non-unknown) values for every field, used as a starting point by tests
// that only want to vary one field at a time.
func baseModel(ctx context.Context, t *testing.T) *SMBConfigModel {
	t.Helper()

	netbiosalias, diags := types.ListValueFrom(ctx, types.StringType, []string{"ALIAS1"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}
	bindip, diags := types.ListValueFrom(ctx, types.StringType, []string{"192.0.2.10"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}
	searchProtocols, diags := types.ListValueFrom(ctx, types.StringType, []string{"WSD"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}

	return &SMBConfigModel{
		NetBIOSName:      types.StringValue("truenas"),
		NetBIOSAlias:     netbiosalias,
		Workgroup:        types.StringValue("WORKGROUP"),
		Description:      types.StringValue("TrueNAS Server"),
		UnixCharset:      types.StringValue("UTF-8"),
		LocalMaster:      types.BoolValue(true),
		Syslog:           types.BoolValue(false),
		AAPLExtensions:   types.BoolValue(true),
		AdminGroup:       types.StringValue("smb_admins"),
		Guest:            types.StringValue("nobody"),
		FileMask:         types.StringValue("DEFAULT"),
		DirMask:          types.StringValue("DEFAULT"),
		NTLMv1Auth:       types.BoolValue(false),
		Multichannel:     types.BoolValue(false),
		Encryption:       types.StringValue("DEFAULT"),
		BindIP:           bindip,
		SMBOptions:       types.StringValue(""),
		Debug:            types.BoolValue(false),
		StatefulFailover: types.BoolValue(false),
		MinimumProtocol:  types.StringValue("SMB2"),
		SearchProtocols:  searchProtocols,
		ServerSID:        types.StringValue("S-1-5-21-1111111111-2222222222-3333333333"),
	}
}

// TestSMBConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestSMBConfigSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idStr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !idStr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idStr.IsRequired() || idStr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
	if len(idStr.PlanModifiers) == 0 {
		t.Error("'id' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestSMBConfigSchema_ServerSIDIsComputedOnly verifies that server_sid is
// Computed-only (never Optional): it is a stable server-assigned value that
// must never be part of the update payload.
func TestSMBConfigSchema_ServerSIDIsComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["server_sid"]
	if !ok {
		t.Fatal("schema missing 'server_sid' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'server_sid' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsComputed() {
		t.Error("'server_sid' should be Computed")
	}
	if strAttr.IsOptional() || strAttr.IsRequired() {
		t.Error("'server_sid' should be Computed-only (not Optional/Required)")
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("'server_sid' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestSMBConfigSchema_AllOtherFieldsOptionalComputed verifies that every
// writable field is Optional+Computed with a plan modifier, matching the "all
// writable fields Optional+Computed + UseStateForUnknown" contract in the
// task brief.
func TestSMBConfigSchema_AllOtherFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	stringFields := []string{
		"netbiosname", "workgroup", "description", "unixcharset",
		"admin_group", "guest", "filemask", "dirmask", "encryption",
		"smb_options", "minimum_protocol",
	}
	for _, name := range stringFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	boolFields := []string{
		"localmaster", "syslog", "aapl_extensions", "ntlmv1_auth",
		"multichannel", "debug", "stateful_failover",
	}
	for _, name := range boolFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(boolAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	listFields := []string{"netbiosalias", "bindip", "search_protocols"}
	for _, name := range listFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListAttribute", name, attr)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(listAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
		if listAttr.ElementType != types.StringType {
			t.Errorf("%q ElementType = %v, want types.StringType", name, listAttr.ElementType)
		}
	}
}

// TestResponseToModel_ListsNilBecomeEmptyList verifies that nil list fields
// from the API map to empty (non-null) lists in the model, for all three
// list fields.
func TestResponseToModel_ListsNilBecomeEmptyList(t *testing.T) {
	api := &smbConfigAPI{
		ID:              1,
		NetBIOSAlias:    nil,
		BindIP:          nil,
		SearchProtocols: nil,
		FileMask:        "DEFAULT",
		DirMask:         "DEFAULT",
		MinimumProtocol: "SMB2",
	}

	m := &SMBConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for name, l := range map[string]types.List{
		"netbiosalias":     m.NetBIOSAlias,
		"bindip":           m.BindIP,
		"search_protocols": m.SearchProtocols,
	} {
		if l.IsNull() {
			t.Errorf("%s should not be null when API returns nil, want empty list", name)
		}
		var out []string
		d := l.ElementsAs(context.Background(), &out, false)
		if d.HasError() {
			t.Fatalf("unexpected error extracting %s elements: %v", name, d)
		}
		if len(out) != 0 {
			t.Errorf("%s = %v, want empty slice", name, out)
		}
	}

	if m.ID.ValueString() != smbConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), smbConfigResourceID)
	}
}

// TestResponseToModel_ListsSet verifies that non-nil list fields from the API
// are carried through as-is.
func TestResponseToModel_ListsSet(t *testing.T) {
	api := &smbConfigAPI{
		ID:              1,
		NetBIOSAlias:    []string{"ALIAS1", "ALIAS2"},
		BindIP:          []string{"192.0.2.10"},
		SearchProtocols: []string{"WSD", "NSD"},
	}

	m := &SMBConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var alias []string
	if d := m.NetBIOSAlias.ElementsAs(context.Background(), &alias, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(alias) != 2 || alias[0] != "ALIAS1" || alias[1] != "ALIAS2" {
		t.Errorf("NetBIOSAlias = %v, want [ALIAS1 ALIAS2]", alias)
	}

	var bindip []string
	if d := m.BindIP.ElementsAs(context.Background(), &bindip, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(bindip) != 1 || bindip[0] != "192.0.2.10" {
		t.Errorf("BindIP = %v, want [192.0.2.10]", bindip)
	}

	var search []string
	if d := m.SearchProtocols.ElementsAs(context.Background(), &search, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(search) != 2 || search[0] != "WSD" || search[1] != "NSD" {
		t.Errorf("SearchProtocols = %v, want [WSD NSD]", search)
	}
}

// TestResponseToModel_AdminGroupNilBecomesEmptyString verifies that API nil
// (JSON null) for admin_group maps to an empty string in state.
func TestResponseToModel_AdminGroupNilBecomesEmptyString(t *testing.T) {
	api := &smbConfigAPI{ID: 1, AdminGroup: nil}
	m := &SMBConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.AdminGroup.IsNull() {
		t.Error("AdminGroup should not be null when API returns nil, want empty string")
	}
	if m.AdminGroup.ValueString() != "" {
		t.Errorf("AdminGroup = %q, want empty string", m.AdminGroup.ValueString())
	}
}

// TestResponseToModel_AdminGroupSet verifies that a non-nil API value for
// admin_group is carried through as-is.
func TestResponseToModel_AdminGroupSet(t *testing.T) {
	api := &smbConfigAPI{ID: 1, AdminGroup: stringPtr("smb_admins")}
	m := &SMBConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.AdminGroup.ValueString() != "smb_admins" {
		t.Errorf("AdminGroup = %q, want %q", m.AdminGroup.ValueString(), "smb_admins")
	}
}

// TestResponseToDataSourceModel_ListsNilBecomeEmptyList mirrors the same
// nil-to-empty-list handling for the datasource model.
func TestResponseToDataSourceModel_ListsNilBecomeEmptyList(t *testing.T) {
	api := &smbConfigAPI{
		ID:              1,
		NetBIOSAlias:    nil,
		BindIP:          nil,
		SearchProtocols: nil,
	}

	m := &SMBConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.NetBIOSAlias.IsNull() || m.BindIP.IsNull() || m.SearchProtocols.IsNull() {
		t.Error("list fields should not be null when API returns nil, want empty lists")
	}
}

// TestResponseToDataSourceModel_AdminGroupNilBecomesEmptyString mirrors the
// admin_group nil handling for the datasource model.
func TestResponseToDataSourceModel_AdminGroupNilBecomesEmptyString(t *testing.T) {
	api := &smbConfigAPI{ID: 1, AdminGroup: nil}
	m := &SMBConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.AdminGroup.ValueString() != "" {
		t.Errorf("AdminGroup = %q, want empty string", m.AdminGroup.ValueString())
	}
}

// TestUpdatePayload_ServerSIDNeverInPayload verifies that server_sid is never
// included in the update payload, even though it holds a known value: it is
// Computed-only and TrueNAS does not accept it as an smb.update argument.
func TestUpdatePayload_ServerSIDNeverInPayload(t *testing.T) {
	m := baseModel(context.Background(), t)
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["server_sid"]; ok {
		t.Error("'server_sid' should never be present in the update payload")
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any
// field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &SMBConfigModel{
		NetBIOSName:      types.StringNull(),
		NetBIOSAlias:     types.ListNull(types.StringType),
		Workgroup:        types.StringUnknown(),
		Description:      types.StringNull(),
		UnixCharset:      types.StringNull(),
		LocalMaster:      types.BoolNull(),
		Syslog:           types.BoolUnknown(),
		AAPLExtensions:   types.BoolNull(),
		AdminGroup:       types.StringNull(),
		Guest:            types.StringNull(),
		FileMask:         types.StringNull(),
		DirMask:          types.StringNull(),
		NTLMv1Auth:       types.BoolNull(),
		Multichannel:     types.BoolNull(),
		Encryption:       types.StringNull(),
		BindIP:           types.ListUnknown(types.StringType),
		SMBOptions:       types.StringNull(),
		Debug:            types.BoolNull(),
		StatefulFailover: types.BoolNull(),
		MinimumProtocol:  types.StringValue("SMB2"),
		SearchProtocols:  types.ListNull(types.StringType),
		ServerSID:        types.StringValue("S-1-5-21-1"),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	omitted := []string{
		"netbiosname", "netbiosalias", "workgroup", "description", "unixcharset",
		"localmaster", "syslog", "aapl_extensions", "admin_group", "guest",
		"filemask", "dirmask", "ntlmv1_auth", "multichannel", "encryption",
		"bindip", "smb_options", "debug", "server_sid",
		// stateful_failover, minimum_protocol, and search_protocols are
		// never in updatePayload's output regardless of null/known status:
		// they are handled entirely by resource.go's
		// applyPost2600FieldsSupport (see updatePayload's doc comment).
		// The model here sets minimum_protocol to a known "SMB2" value to
		// prove that too is still excluded.
		"stateful_failover", "minimum_protocol", "search_protocols",
	}
	for _, k := range omitted {
		if _, ok := p[k]; ok {
			t.Errorf("expected %q to be omitted from updatePayload's output", k)
		}
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known, and that server_sid
// and the three post-26.0 fields (handled separately by
// applyPost2600FieldsSupport) are excluded even though the model carries
// known values for all of them.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	ctx := context.Background()
	m := baseModel(ctx, t)

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"netbiosname":     "truenas",
		"workgroup":       "WORKGROUP",
		"description":     "TrueNAS Server",
		"unixcharset":     "UTF-8",
		"localmaster":     true,
		"syslog":          false,
		"aapl_extensions": true,
		"admin_group":     "smb_admins",
		"guest":           "nobody",
		"filemask":        "DEFAULT",
		"dirmask":         "DEFAULT",
		"ntlmv1_auth":     false,
		"multichannel":    false,
		"encryption":      "DEFAULT",
		"smb_options":     "",
		"debug":           false,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}

	for name, wantVal := range map[string][]string{
		"netbiosalias": {"ALIAS1"},
		"bindip":       {"192.0.2.10"},
	} {
		got, ok := p[name].([]string)
		if !ok {
			t.Fatalf("payload[%s] is %T, want []string", name, p[name])
		}
		if !reflect.DeepEqual(got, wantVal) {
			t.Errorf("payload[%s] = %v, want %v", name, got, wantVal)
		}
	}

	for _, k := range []string{"server_sid", "stateful_failover", "minimum_protocol", "search_protocols"} {
		if _, ok := p[k]; ok {
			t.Errorf("%q should never be present in updatePayload's output", k)
		}
	}

	// want + 2 lists, server_sid and the three post-26.0 fields excluded.
	if len(p) != len(want)+2 {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want)+2)
	}
}

// TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice verifies that a known but
// empty list is sent as an empty slice, not nil/null, for the two list
// fields updatePayload still handles directly (search_protocols moved to
// applyPost2600FieldsSupport).
func TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice(t *testing.T) {
	ctx := context.Background()
	emptyList, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("unexpected error building empty list: %v", diags)
	}

	m := &SMBConfigModel{
		NetBIOSName:      types.StringNull(),
		NetBIOSAlias:     emptyList,
		Workgroup:        types.StringNull(),
		Description:      types.StringNull(),
		UnixCharset:      types.StringNull(),
		LocalMaster:      types.BoolNull(),
		Syslog:           types.BoolNull(),
		AAPLExtensions:   types.BoolNull(),
		AdminGroup:       types.StringNull(),
		Guest:            types.StringNull(),
		FileMask:         types.StringNull(),
		DirMask:          types.StringNull(),
		NTLMv1Auth:       types.BoolNull(),
		Multichannel:     types.BoolNull(),
		Encryption:       types.StringNull(),
		BindIP:           emptyList,
		SMBOptions:       types.StringNull(),
		Debug:            types.BoolNull(),
		StatefulFailover: types.BoolNull(),
		MinimumProtocol:  types.StringNull(),
		SearchProtocols:  emptyList,
		ServerSID:        types.StringNull(),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, name := range []string{"netbiosalias", "bindip"} {
		got, ok := p[name].([]string)
		if !ok {
			t.Fatalf("payload[%s] is %T, want []string", name, p[name])
		}
		if len(got) != 0 {
			t.Errorf("payload[%s] = %v, want empty slice", name, got)
		}
	}
	if _, ok := p["search_protocols"]; ok {
		t.Error("'search_protocols' should never be present in updatePayload's output (handled by applyPost2600FieldsSupport)")
	}
}

// TestPost2600FieldsSupported verifies the pure version-comparison gate for
// smb.update's stateful_failover/minimum_protocol/search_protocols fields,
// including the exact boundary and malformed-input edge cases.
func TestPost2600FieldsSupported(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"25.10.3.1", false},
		{"25.10.4", false},
		{"25.10.4.1", false},
		{"25.04.0", false},
		{"26.0.0", true},
		{"26.0.0-BETA.2", true},
		{"26.1.0", true},
		{"27.0.0", true},
		{"", false},
		{"not-a-version", false},
	}
	for _, c := range cases {
		if got := post2600FieldsSupported(c.version); got != c.want {
			t.Errorf("post2600FieldsSupported(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

// TestApplyPost2600FieldsSupport_ProbeErrorStripsFields verifies the
// fail-closed direction of applyPost2600FieldsSupport: when the version
// probe itself errors (ServerVersion returns err != nil), the gate must
// strip stateful_failover/minimum_protocol/search_protocols from the
// payload rather than keep them, since keeping them on an unprobed
// (majority pre-26.0) target would send fields smb.update rejects with its
// generic "Extra inputs are not permitted" error. A client that was
// constructed but never Connect()-ed has a nil underlying connection, so
// ServerVersion's CallRead fails immediately with "not connected"; the
// context passed in is pre-cancelled so CallRead's retry loop exits on
// ctx.Done() instead of sleeping through real backoff delays or attempting
// a real network dial.
func TestApplyPost2600FieldsSupport_ProbeErrorStripsFields(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	searchProtocols, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"WSD"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}

	r := &SMBConfigResource{client: client.New("ws://127.0.0.1:0", nil)}
	cfg := &SMBConfigModel{
		StatefulFailover: types.BoolValue(true),
		MinimumProtocol:  types.StringValue("SMB2"),
		SearchProtocols:  searchProtocols,
	}
	payload := map[string]any{}
	var applyDiags diag.Diagnostics
	r.applyPost2600FieldsSupport(ctx, payload, cfg, &applyDiags)

	for _, k := range []string{"stateful_failover", "minimum_protocol", "search_protocols"} {
		if _, ok := payload[k]; ok {
			t.Errorf("payload[%q] present after a version-probe error; gate should fail closed and strip it", k)
		}
	}
	if !applyDiags.HasError() {
		t.Error("expected an apply-time error when the version probe fails, got none")
	}
}

// TestUpdatePayload_AdminGroupThreeWay verifies the three-way nullable
// handling for admin_group: omitted when null/unknown, sent as nil when
// explicitly set to "", and sent as the value otherwise.
func TestUpdatePayload_AdminGroupThreeWay(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.AdminGroup = types.StringNull()
	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["admin_group"]; ok {
		t.Error("expected 'admin_group' to be omitted when null")
	}

	m2 := baseModel(ctx, t)
	m2.AdminGroup = types.StringUnknown()
	p2, diags := m2.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p2["admin_group"]; ok {
		t.Error("expected 'admin_group' to be omitted when unknown")
	}

	m3 := baseModel(ctx, t)
	m3.AdminGroup = types.StringValue("")
	p3, diags := m3.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p3["admin_group"]; !ok {
		t.Error("expected 'admin_group' to be present (nil) when explicitly set to empty string")
	} else if v != nil {
		t.Errorf("expected 'admin_group' = nil when explicitly set to empty string, got %v", v)
	}

	m4 := baseModel(ctx, t)
	m4.AdminGroup = types.StringValue("smb_admins")
	p4, diags := m4.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p4["admin_group"]; !ok || v != "smb_admins" {
		t.Errorf("expected 'admin_group' = %q, got %v (present=%v)", "smb_admins", v, ok)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// SMBConfigResource.Delete calls only this pure function.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	summary := diags[0].Summary()
	detail := diags[0].Detail()
	if summary == "" || detail == "" {
		t.Error("expected non-empty summary and detail")
	}
	if detail != "SMB configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestSMBConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// SMBConfigDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa. terraform-plugin-framework requires an exact
// field/attribute match: any mismatch causes every datasource Read to fail
// with "Struct defines fields not found in object: ...".
func TestSMBConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &SMBConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(SMBConfigDataSourceModel{})
	modelFields := make([]string, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "" {
			t.Fatalf("field %q has no tfsdk tag", modelType.Field(i).Name)
		}
		modelFields = append(modelFields, tag)
	}
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("SMBConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
