// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestSMBSchema verifies that the resource schema has the expected attributes
// and that key attributes have the correct types.
func TestSMBSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}

	// path must be StringAttribute and Required.
	pathAttr, ok := s.Attributes["path"]
	if !ok {
		t.Fatal("schema missing 'path' attribute")
	}
	pathStr, ok := pathAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'path' attribute is %T, want schema.StringAttribute", pathAttr)
	}
	if !pathStr.IsRequired() {
		t.Error("'path' should be Required")
	}

	// hostsallow and hostsdeny must be ListAttribute.
	for _, listField := range []string{"hostsallow", "hostsdeny"} {
		attr, ok := s.Attributes[listField]
		if !ok {
			t.Fatalf("schema missing %q attribute", listField)
		}
		if _, ok := attr.(schema.ListAttribute); !ok {
			t.Errorf("%q attribute is %T, want schema.ListAttribute", listField, attr)
		}
	}

	// The Terraform attribute names stay flat/unchanged (ro, abe, etc.) even
	// though the wire format renames/nests them — this is what keeps
	// existing configs backward compatible.
	for _, field := range []string{"ro", "abe", "recyclebin", "hostsallow", "hostsdeny", "guestok", "acl", "durablehandle", "streams", "timemachine", "timemachine_quota", "home", "purpose"} {
		if _, ok := s.Attributes[field]; !ok {
			t.Errorf("schema missing %q attribute (flat schema must stay backward compatible)", field)
		}
	}

	// vuid and locked must be Computed-only.
	for _, field := range []string{"vuid", "locked"} {
		a, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		switch v := a.(type) {
		case schema.StringAttribute:
			if !v.IsComputed() {
				t.Errorf("%q should be Computed", field)
			}
			if v.IsOptional() || v.IsRequired() {
				t.Errorf("%q should be Computed-only (not Optional/Required)", field)
			}
		case schema.BoolAttribute:
			if !v.IsComputed() {
				t.Errorf("%q should be Computed", field)
			}
			if v.IsOptional() || v.IsRequired() {
				t.Errorf("%q should be Computed-only (not Optional/Required)", field)
			}
		default:
			t.Errorf("%q has unexpected attribute type %T", field, a)
		}
	}
}

// baseLegacyModel returns an SMBModel with every field set to a
// non-null/non-unknown value and Purpose unset, matching the common case of
// a share that relies on the LEGACY_SHARE default.
func baseLegacyModel() SMBModel {
	return SMBModel{
		Path:             types.StringValue("/mnt/tank/share"),
		Name:             types.StringValue("myshare"),
		Comment:          types.StringValue("test comment"),
		ReadOnly:         types.BoolValue(false),
		Browsable:        types.BoolValue(true),
		Recyclebin:       types.BoolValue(false),
		GuestOK:          types.BoolValue(false),
		HostsAllow:       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("192.168.1.0/24")}),
		HostsDeny:        types.ListValueMust(types.StringType, []attr.Value{}),
		ABE:              types.BoolValue(false),
		ACL:              types.BoolValue(true),
		DurableHandle:    types.BoolValue(true),
		Streams:          types.BoolValue(true),
		TimeMachine:      types.BoolValue(false),
		TimeMachineQuota: types.Int64Value(0),
		Enabled:          types.BoolValue(true),
		Home:             types.BoolValue(false),
		Purpose:          types.StringNull(),
		VUID:             types.StringValue(""),
		Locked:           types.BoolValue(false),
	}
}

// TestSMBApiPayload_TopLevel verifies apiPayload produces the TrueNAS 26.0
// top-level keys: path/name/comment/enabled/browsable/readonly/
// access_based_share_enumeration/purpose/options, and that the legacy 24.x
// top-level keys (ro, abe, hostsallow, etc.) are gone from the top level.
func TestSMBApiPayload_TopLevel(t *testing.T) {
	ctx := context.Background()

	m := baseLegacyModel()
	m.ReadOnly = types.BoolValue(true)
	m.ABE = types.BoolValue(true)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	expectedKeys := []string{
		"path", "name", "comment", "enabled", "browsable",
		"readonly", "access_based_share_enumeration", "purpose", "options",
	}
	if len(payload) != len(expectedKeys) {
		t.Errorf("payload has %d keys, want %d: %v", len(payload), len(expectedKeys), payload)
	}
	for _, k := range expectedKeys {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing top-level key %q", k)
		}
	}

	// The pre-26.0 top-level keys must not be present at the top level
	// anymore — they've either moved into options (legacy flags) or been
	// renamed (ro -> readonly, abe -> access_based_share_enumeration).
	forbidden := []string{"ro", "abe", "hostsallow", "hostsdeny", "recyclebin", "guestok", "acl", "durablehandle", "streams", "timemachine", "timemachine_quota", "home", "vuid", "locked", "id"}
	for _, k := range forbidden {
		if _, ok := payload[k]; ok {
			t.Errorf("payload should not contain top-level key %q (TrueNAS 26.0 nests/renames it)", k)
		}
	}

	if payload["path"] != "/mnt/tank/share" {
		t.Errorf("payload[path] = %v, want /mnt/tank/share", payload["path"])
	}
	if payload["readonly"] != true {
		t.Errorf("payload[readonly] = %v, want true", payload["readonly"])
	}
	if payload["access_based_share_enumeration"] != true {
		t.Errorf("payload[access_based_share_enumeration] = %v, want true", payload["access_based_share_enumeration"])
	}
	if payload["purpose"] != legacySharePurpose {
		t.Errorf("payload[purpose] = %v, want %v", payload["purpose"], legacySharePurpose)
	}
}

// TestSMBApiPayload_PurposeDefaultsToLegacy verifies that apiPayload defaults
// purpose to LEGACY_SHARE (both top-level and options.purpose) when the
// caller hasn't set a purpose, and that the legacy flags are nested under
// options with the discriminator.
func TestSMBApiPayload_PurposeDefaultsToLegacy(t *testing.T) {
	ctx := context.Background()

	m := baseLegacyModel()
	m.Purpose = types.StringNull()

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if payload["purpose"] != legacySharePurpose {
		t.Errorf("payload[purpose] = %v, want %v", payload["purpose"], legacySharePurpose)
	}

	options, ok := payload["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] is %T, want map[string]any", payload["options"])
	}
	if options["purpose"] != legacySharePurpose {
		t.Errorf("payload[options][purpose] = %v, want %v", options["purpose"], legacySharePurpose)
	}

	for _, k := range []string{"recyclebin", "guestok", "streams", "durablehandle", "home", "acl", "timemachine", "timemachine_quota", "hostsallow", "hostsdeny"} {
		if _, ok := options[k]; !ok {
			t.Errorf("options missing legacy key %q for LEGACY_SHARE purpose", k)
		}
	}

	hostsAllow, ok := options["hostsallow"].([]string)
	if !ok {
		t.Fatalf("options[hostsallow] is %T, want []string", options["hostsallow"])
	}
	if len(hostsAllow) != 1 || hostsAllow[0] != "192.168.1.0/24" {
		t.Errorf("options[hostsallow] = %v, want [192.168.1.0/24]", hostsAllow)
	}
}

// TestSMBApiPayload_PurposeDefaultsToLegacyWhenUnrecognized verifies that an
// unrecognized (e.g. stale 24.x) purpose value also falls back to the
// LEGACY_SHARE default, rather than being passed through verbatim.
func TestSMBApiPayload_PurposeDefaultsToLegacyWhenUnrecognized(t *testing.T) {
	ctx := context.Background()

	m := baseLegacyModel()
	m.Purpose = types.StringValue("NO_PRESET")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if payload["purpose"] != legacySharePurpose {
		t.Errorf("payload[purpose] = %v, want %v (fallback for unrecognized value)", payload["purpose"], legacySharePurpose)
	}
}

// TestSMBApiPayload_NonLegacyPurposeOmitsLegacyFlags verifies that when the
// caller sets purpose to a known non-LEGACY_SHARE value, options only
// contains purpose (variant defaults apply server-side) and none of the
// legacy flags are sent, either at the top level or in options.
func TestSMBApiPayload_NonLegacyPurposeOmitsLegacyFlags(t *testing.T) {
	ctx := context.Background()

	m := baseLegacyModel()
	m.Purpose = types.StringValue("TIMEMACHINE_SHARE")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if payload["purpose"] != "TIMEMACHINE_SHARE" {
		t.Errorf("payload[purpose] = %v, want TIMEMACHINE_SHARE", payload["purpose"])
	}

	options, ok := payload["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] is %T, want map[string]any", payload["options"])
	}
	if len(options) != 1 {
		t.Errorf("options has %d keys, want 1 (purpose only): %v", len(options), options)
	}
	if options["purpose"] != "TIMEMACHINE_SHARE" {
		t.Errorf("options[purpose] = %v, want TIMEMACHINE_SHARE", options["purpose"])
	}

	for _, k := range []string{"recyclebin", "guestok", "streams", "durablehandle", "home", "acl", "timemachine", "timemachine_quota", "hostsallow", "hostsdeny"} {
		if _, ok := options[k]; ok {
			t.Errorf("options should not contain legacy key %q for non-LEGACY_SHARE purpose", k)
		}
		if _, ok := payload[k]; ok {
			t.Errorf("payload should not contain top-level legacy key %q", k)
		}
	}
}

// TestSMBApiPayload_LegacyFlagsGuardedByNullUnknown verifies that legacy
// flags are only added to options when they are neither null nor unknown in
// the model (guards against clobbering unrelated server-side defaults), and
// that null hostsallow/hostsdeny are simply omitted (not sent as nil).
func TestSMBApiPayload_LegacyFlagsGuardedByNullUnknown(t *testing.T) {
	ctx := context.Background()

	m := baseLegacyModel()
	m.Recyclebin = types.BoolNull()
	m.GuestOK = types.BoolUnknown()
	m.HostsAllow = types.ListNull(types.StringType)
	m.HostsDeny = types.ListUnknown(types.StringType)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	options, ok := payload["options"].(map[string]any)
	if !ok {
		t.Fatalf("payload[options] is %T, want map[string]any", payload["options"])
	}

	for _, k := range []string{"recyclebin", "guestok", "hostsallow", "hostsdeny"} {
		if _, ok := options[k]; ok {
			t.Errorf("options should omit key %q when null/unknown in model", k)
		}
	}

	// Fields that are still set concretely must still appear.
	for _, k := range []string{"streams", "durablehandle", "home", "acl", "timemachine", "timemachine_quota"} {
		if _, ok := options[k]; !ok {
			t.Errorf("options missing key %q that was set in model", k)
		}
	}
}

// TestResponseToModel_LegacyShare verifies that responseToModel decodes the
// nested TrueNAS 26.0 response shape (top-level readonly/
// access_based_share_enumeration, options.* for legacy flags) into the flat
// SMBModel fields.
func TestResponseToModel_LegacyShare(t *testing.T) {
	ctx := context.Background()

	locked := false
	vuid := "abc-123"
	api := &smbAPI{
		ID:        42,
		Path:      "/mnt/tank/share",
		Name:      "myshare",
		Comment:   "test",
		ReadOnly:  true,
		Browsable: true,
		ABE:       false,
		Enabled:   true,
		Purpose:   legacySharePurpose,
		Locked:    &locked,
		Options: &smbOptionsAPI{
			Purpose:          legacySharePurpose,
			Recyclebin:       false,
			HostsAllow:       []string{"10.0.0.1"},
			HostsDeny:        []string{},
			GuestOK:          false,
			Streams:          true,
			DurableHandle:    true,
			Home:             false,
			ACL:              true,
			TimeMachine:      false,
			TimeMachineQuota: 100,
			VUID:             &vuid,
		},
	}

	var m SMBModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("ID = %v, want 42", m.ID.ValueInt64())
	}
	if m.Path.ValueString() != "/mnt/tank/share" {
		t.Errorf("Path = %v, want /mnt/tank/share", m.Path.ValueString())
	}
	if !m.ReadOnly.ValueBool() {
		t.Error("ReadOnly = false, want true (from top-level readonly)")
	}
	if m.ABE.ValueBool() {
		t.Error("ABE = true, want false (from top-level access_based_share_enumeration)")
	}
	if m.VUID.ValueString() != "abc-123" {
		t.Errorf("VUID = %v, want abc-123 (from options.vuid)", m.VUID.ValueString())
	}
	if m.TimeMachineQuota.ValueInt64() != 100 {
		t.Errorf("TimeMachineQuota = %v, want 100 (from options.timemachine_quota)", m.TimeMachineQuota.ValueInt64())
	}
	if !m.Streams.ValueBool() {
		t.Error("Streams = false, want true (from options.streams)")
	}
	if m.Locked.ValueBool() {
		t.Error("Locked = true, want false")
	}
	if m.Purpose.ValueString() != legacySharePurpose {
		t.Errorf("Purpose = %v, want %v", m.Purpose.ValueString(), legacySharePurpose)
	}

	var ha []string
	if diags := m.HostsAllow.ElementsAs(ctx, &ha, false); diags.HasError() {
		t.Fatalf("HostsAllow.ElementsAs failed: %v", diags)
	}
	if len(ha) != 1 || ha[0] != "10.0.0.1" {
		t.Errorf("HostsAllow = %v, want [10.0.0.1]", ha)
	}

	var hd []string
	if diags := m.HostsDeny.ElementsAs(ctx, &hd, false); diags.HasError() {
		t.Fatalf("HostsDeny.ElementsAs failed: %v", diags)
	}
	if len(hd) != 0 {
		t.Errorf("HostsDeny = %v, want []", hd)
	}
}

// TestResponseToModel_NonLegacyShare verifies that for a non-LEGACY_SHARE
// purpose, responseToModel zeroes out the legacy fields (since they are not
// present/meaningful in the options variant for e.g. TIMEMACHINE_SHARE) and
// still correctly maps the top-level fields, locked (including the null
// case), and purpose.
func TestResponseToModel_NonLegacyShare(t *testing.T) {
	ctx := context.Background()

	api := &smbAPI{
		ID:        7,
		Path:      "/mnt/tank/tm",
		Name:      "tmshare",
		Comment:   "",
		ReadOnly:  false,
		Browsable: true,
		ABE:       true,
		Enabled:   true,
		Purpose:   "TIMEMACHINE_SHARE",
		Locked:    nil, // lock status not requested/available
		Options: &smbOptionsAPI{
			Purpose: "TIMEMACHINE_SHARE",
		},
	}

	var m SMBModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.Purpose.ValueString() != "TIMEMACHINE_SHARE" {
		t.Errorf("Purpose = %v, want TIMEMACHINE_SHARE", m.Purpose.ValueString())
	}
	if m.Locked.ValueBool() {
		t.Error("Locked = true, want false (zero value when API returns null)")
	}
	if m.VUID.ValueString() != "" {
		t.Errorf("VUID = %v, want \"\" (not present outside LEGACY_SHARE options)", m.VUID.ValueString())
	}
	if m.Recyclebin.ValueBool() {
		t.Error("Recyclebin = true, want false (zero value, not modeled by TIMEMACHINE_SHARE options)")
	}

	var ha []string
	if diags := m.HostsAllow.ElementsAs(ctx, &ha, false); diags.HasError() {
		t.Fatalf("HostsAllow.ElementsAs failed: %v", diags)
	}
	if len(ha) != 0 {
		t.Errorf("HostsAllow = %v, want [] (not modeled by TIMEMACHINE_SHARE options)", ha)
	}
}

// TestResponseToModel_LegacyShare_RawJSONFixture decodes a raw JSON payload
// shaped exactly like the sharing.smb.create / get_instance response on
// TrueNAS 26.0 (per the middleware schema dump: top-level readonly/
// access_based_share_enumeration/purpose, and legacy flags nested under
// options for the LEGACY_SHARE variant). Unlike TestResponseToModel_LegacyShare
// (which builds the smbAPI struct directly in Go and so can't catch a wrong
// or missing `json` struct tag), this test goes through json.Unmarshal so a
// tag typo or a field TrueNAS nests differently than expected would make the
// decoded hostsallow/hostsdeny come back empty here, reproducing the
// "element 0 has vanished" inconsistent-apply-result failure directly.
func TestResponseToModel_LegacyShare_RawJSONFixture(t *testing.T) {
	ctx := context.Background()

	raw := []byte(`{
		"id": 55,
		"purpose": "LEGACY_SHARE",
		"name": "myshare",
		"path": "/mnt/tank/share",
		"dataset": "tank/share",
		"relative_path": "",
		"enabled": true,
		"comment": "test",
		"readonly": false,
		"browsable": true,
		"access_based_share_enumeration": false,
		"locked": false,
		"audit": {"enable": false, "watch_list": [], "ignore_list": []},
		"options": {
			"purpose": "LEGACY_SHARE",
			"recyclebin": false,
			"path_suffix": null,
			"hostsallow": ["192.168.1.0/24", "10.0.0.5"],
			"hostsdeny": ["ALL"],
			"guestok": false,
			"streams": true,
			"durablehandle": true,
			"shadowcopy": true,
			"fsrvp": false,
			"home": false,
			"acl": true,
			"afp": false,
			"timemachine": false,
			"timemachine_quota": 0,
			"aapl_name_mangling": false,
			"vuid": null,
			"auxsmbconf": ""
		}
	}`)

	var api smbAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		t.Fatalf("json.Unmarshal into smbAPI failed: %v", err)
	}

	var m SMBModel
	diags := responseToModel(ctx, &api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	var ha []string
	if diags := m.HostsAllow.ElementsAs(ctx, &ha, false); diags.HasError() {
		t.Fatalf("HostsAllow.ElementsAs failed: %v", diags)
	}
	if len(ha) != 2 || ha[0] != "192.168.1.0/24" || ha[1] != "10.0.0.5" {
		t.Errorf("HostsAllow = %v, want [192.168.1.0/24 10.0.0.5] (must survive raw-JSON decode of nested options)", ha)
	}

	var hd []string
	if diags := m.HostsDeny.ElementsAs(ctx, &hd, false); diags.HasError() {
		t.Fatalf("HostsDeny.ElementsAs failed: %v", diags)
	}
	if len(hd) != 1 || hd[0] != "ALL" {
		t.Errorf("HostsDeny = %v, want [ALL]", hd)
	}

	if !m.Streams.ValueBool() {
		t.Error("Streams = false, want true (from raw JSON options.streams)")
	}
	if !m.DurableHandle.ValueBool() {
		t.Error("DurableHandle = false, want true (from raw JSON options.durablehandle)")
	}
	if m.ID.ValueInt64() != 55 {
		t.Errorf("ID = %v, want 55", m.ID.ValueInt64())
	}
}

// TestSMBApiPayload_Audit verifies the audit block is emitted at the top level
// (not nested under options) with its sub-fields when set.
func TestSMBApiPayload_Audit(t *testing.T) {
	ctx := context.Background()

	m := baseLegacyModel()
	auditObj, d := types.ObjectValueFrom(ctx, smbAuditAttrTypes, SMBAuditModel{
		Enable:     types.BoolValue(true),
		WatchList:  types.ListValueMust(types.StringType, []attr.Value{types.StringValue("grp")}),
		IgnoreList: types.ListValueMust(types.StringType, []attr.Value{}),
	})
	if d.HasError() {
		t.Fatalf("build audit object: %v", d)
	}
	m.Audit = auditObj

	payload, pd := m.apiPayload(ctx)
	if pd.HasError() {
		t.Fatalf("apiPayload: %v", pd)
	}
	audit, ok := payload["audit"].(map[string]any)
	if !ok {
		t.Fatalf("payload[audit] is %T, want map[string]any", payload["audit"])
	}
	if audit["enable"] != true {
		t.Errorf("audit.enable = %v, want true", audit["enable"])
	}
	wl, ok := audit["watch_list"].([]string)
	if !ok || len(wl) != 1 || wl[0] != "grp" {
		t.Errorf("audit.watch_list = %v, want [grp]", audit["watch_list"])
	}
	// audit lives at the top level, never inside options.
	if opts, ok := payload["options"].(map[string]any); ok {
		if _, bad := opts["audit"]; bad {
			t.Error("audit must not be nested under options")
		}
	}
}

// TestSMBApiPayload_AuditOmittedWhenNull verifies an unset audit block is not
// sent (baseLegacyModel leaves Audit null).
func TestSMBApiPayload_AuditOmittedWhenNull(t *testing.T) {
	ctx := context.Background()
	m := baseLegacyModel()
	payload, _ := m.apiPayload(ctx)
	if _, ok := payload["audit"]; ok {
		t.Errorf("unset audit should be omitted, got %v", payload["audit"])
	}
}

// TestSMBResponseToModel_Audit verifies audit decodes to a populated object when
// present and to the default (enable=false, empty lists) object when absent.
func TestSMBResponseToModel_Audit(t *testing.T) {
	ctx := context.Background()

	api := &smbAPI{ID: 1, Path: "/mnt/tank/s", Name: "s", Purpose: legacySharePurpose}
	api.Audit = &struct {
		Enable     bool     `json:"enable"`
		WatchList  []string `json:"watch_list"`
		IgnoreList []string `json:"ignore_list"`
	}{Enable: true, WatchList: []string{"grp"}, IgnoreList: nil}

	var m SMBModel
	if d := responseToModel(ctx, api, &m); d.HasError() {
		t.Fatalf("responseToModel: %v", d)
	}
	var a SMBAuditModel
	if d := m.Audit.As(ctx, &a, basetypes.ObjectAsOptions{}); d.HasError() {
		t.Fatalf("Audit.As: %v", d)
	}
	if !a.Enable.ValueBool() {
		t.Error("audit.enable should be true")
	}
	var wl []string
	_ = a.WatchList.ElementsAs(ctx, &wl, false)
	if len(wl) != 1 || wl[0] != "grp" {
		t.Errorf("watch_list = %v, want [grp]", wl)
	}

	// Absent audit → default object, not null.
	apiNo := &smbAPI{ID: 2, Path: "/mnt/tank/s2", Name: "s2", Purpose: legacySharePurpose}
	var m2 SMBModel
	if d := responseToModel(ctx, apiNo, &m2); d.HasError() {
		t.Fatalf("responseToModel: %v", d)
	}
	if m2.Audit.IsNull() {
		t.Fatal("audit should be a default object, not null")
	}
	var a2 SMBAuditModel
	_ = m2.Audit.As(ctx, &a2, basetypes.ObjectAsOptions{})
	if a2.Enable.ValueBool() {
		t.Error("default audit.enable should be false")
	}
}
