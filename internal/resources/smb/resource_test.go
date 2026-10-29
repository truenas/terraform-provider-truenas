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

func smbBptr(b bool) *bool          { return &b }
func smbIptr(i int64) *int64        { return &i }
func smbSlptr(s []string) *[]string { return &s }

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
			Recyclebin:       smbBptr(false),
			HostsAllow:       smbSlptr([]string{"10.0.0.1"}),
			HostsDeny:        smbSlptr([]string{}),
			GuestOK:          smbBptr(false),
			Streams:          smbBptr(true),
			DurableHandle:    smbBptr(true),
			Home:             smbBptr(false),
			ACL:              smbBptr(true),
			TimeMachine:      smbBptr(false),
			TimeMachineQuota: smbIptr(100),
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
			AutoDatasetCreation: smbBptr(true),
			HostsAllow:          smbSlptr([]string{}),
			HostsDeny:           smbSlptr([]string{}),
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

// --- options (purpose-specific) tests -------------------------------------

func optsObject(t *testing.T, m SMBOptionsModel) types.Object {
	t.Helper()
	o, d := types.ObjectValueFrom(context.Background(), smbOptionsAttrTypes, m)
	if d.HasError() {
		t.Fatalf("build options object: %v", d)
	}
	return o
}

// nullOpts returns an SMBOptionsModel with every field null (nothing set).
func nullOpts() SMBOptionsModel {
	return SMBOptionsModel{
		Recyclebin: types.BoolNull(), PathSuffix: types.StringNull(),
		HostsAllow: types.ListNull(types.StringType), HostsDeny: types.ListNull(types.StringType),
		GuestOK: types.BoolNull(), Streams: types.BoolNull(), DurableHandle: types.BoolNull(),
		Shadowcopy: types.BoolNull(), FSRVP: types.BoolNull(), Home: types.BoolNull(),
		ACL: types.BoolNull(), AFP: types.BoolNull(), TimeMachine: types.BoolNull(),
		TimeMachineQuota: types.Int64Null(), AaplNameMangling: types.BoolNull(),
		VUID: types.StringNull(), AuxSMBConf: types.StringNull(), AutoSnapshot: types.BoolNull(),
		AutoDatasetCreation: types.BoolNull(), DatasetNamingSchema: types.StringNull(),
		GracePeriod: types.Int64Null(), AutoQuota: types.Int64Null(),
		RemotePath: types.ListNull(types.StringType),
	}
}

func optionsMap(t *testing.T, m SMBModel) map[string]any {
	t.Helper()
	p, d := m.apiPayload(context.Background())
	if d.HasError() {
		t.Fatalf("apiPayload: %v", d)
	}
	o, ok := p["options"].(map[string]any)
	if !ok {
		t.Fatalf("options is %T", p["options"])
	}
	return o
}

// TIMEMACHINE options are sent; fields not in the TIMEMACHINE variant are not.
func TestApiPayload_TimeMachineOptions(t *testing.T) {
	o := nullOpts()
	o.AutoDatasetCreation = types.BoolValue(true)
	o.AutoSnapshot = types.BoolValue(true)
	o.TimeMachineQuota = types.Int64Value(100)
	o.Recyclebin = types.BoolValue(true) // NOT valid for TIMEMACHINE — must be dropped
	m := baseLegacyModel()
	m.Purpose = types.StringValue("TIMEMACHINE_SHARE")
	m.Options = optsObject(t, o)

	opts := optionsMap(t, m)
	if opts["purpose"] != "TIMEMACHINE_SHARE" {
		t.Errorf("purpose = %v", opts["purpose"])
	}
	if opts["auto_dataset_creation"] != true {
		t.Errorf("auto_dataset_creation = %v, want true", opts["auto_dataset_creation"])
	}
	if opts["timemachine_quota"] != int64(100) {
		t.Errorf("timemachine_quota = %v, want 100", opts["timemachine_quota"])
	}
	if _, ok := opts["recyclebin"]; ok {
		t.Error("recyclebin must NOT be sent for TIMEMACHINE_SHARE")
	}
}

// DEFAULT_SHARE can carry hostsallow + aapl_name_mangling (the issue's point 2).
func TestApiPayload_DefaultShareHostRestriction(t *testing.T) {
	o := nullOpts()
	o.HostsAllow, _ = types.ListValueFrom(context.Background(), types.StringType, []string{"192.168.1.0/24"})
	o.AaplNameMangling = types.BoolValue(true)
	o.Recyclebin = types.BoolValue(true) // not valid for DEFAULT — dropped
	m := baseLegacyModel()
	m.Purpose = types.StringValue("DEFAULT_SHARE")
	m.Options = optsObject(t, o)

	opts := optionsMap(t, m)
	ha, ok := opts["hostsallow"].([]string)
	if !ok || len(ha) != 1 || ha[0] != "192.168.1.0/24" {
		t.Errorf("hostsallow = %v", opts["hostsallow"])
	}
	if opts["aapl_name_mangling"] != true {
		t.Errorf("aapl_name_mangling = %v", opts["aapl_name_mangling"])
	}
	if _, ok := opts["recyclebin"]; ok {
		t.Error("recyclebin must NOT be sent for DEFAULT_SHARE")
	}
}

// LEGACY back-compat: a flat attr is used when the options block leaves it unset.
func TestApiPayload_LegacyFlatFallback(t *testing.T) {
	m := baseLegacyModel() // Purpose null -> LEGACY; Recyclebin=false flat
	m.Recyclebin = types.BoolValue(true)
	m.Options = types.ObjectNull(smbOptionsAttrTypes) // no options block

	opts := optionsMap(t, m)
	if opts["recyclebin"] != true {
		t.Errorf("recyclebin (flat fallback) = %v, want true", opts["recyclebin"])
	}
}

// options block wins over the flat attr for LEGACY.
func TestApiPayload_OptionsBeatsFlat(t *testing.T) {
	o := nullOpts()
	o.Recyclebin = types.BoolValue(false)
	m := baseLegacyModel()
	m.Recyclebin = types.BoolValue(true) // flat says true
	m.Options = optsObject(t, o)         // options says false -> wins

	opts := optionsMap(t, m)
	if opts["recyclebin"] != false {
		t.Errorf("recyclebin = %v, want false (options block authoritative)", opts["recyclebin"])
	}
}

// responseToModel populates the typed options for a non-legacy purpose.
func TestResponseToModel_OptionsForNonLegacy(t *testing.T) {
	ctx := context.Background()
	api := &smbAPI{
		ID: 9, Path: "/mnt/tank/tm", Name: "tm", Purpose: "TIMEMACHINE_SHARE",
		Options: &smbOptionsAPI{
			AutoDatasetCreation: smbBptr(true),
			TimeMachineQuota:    smbIptr(50),
			HostsAllow:          smbSlptr([]string{}),
			HostsDeny:           smbSlptr([]string{}),
		},
	}
	var m SMBModel
	if d := responseToModel(ctx, api, &m); d.HasError() {
		t.Fatalf("responseToModel: %v", d)
	}
	if m.Options.IsNull() {
		t.Fatal("options should be populated for TIMEMACHINE_SHARE")
	}
	var o SMBOptionsModel
	m.Options.As(ctx, &o, basetypes.ObjectAsOptions{})
	if !o.AutoDatasetCreation.ValueBool() {
		t.Error("options.auto_dataset_creation should be true")
	}
	if o.TimeMachineQuota.ValueInt64() != 50 {
		t.Errorf("options.timemachine_quota = %d, want 50", o.TimeMachineQuota.ValueInt64())
	}
	// a field not in the TIMEMACHINE variant reads null
	if !o.Recyclebin.IsNull() {
		t.Error("options.recyclebin should be null for TIMEMACHINE_SHARE")
	}
}

// invalidOptionKeys flags options set for the wrong purpose.
func TestInvalidOptionKeys_MisfiledTimeMachine(t *testing.T) {
	o := nullOpts()
	o.AutoDatasetCreation = types.BoolValue(true) // valid for TIMEMACHINE
	o.Recyclebin = types.BoolValue(true)          // NOT valid for TIMEMACHINE
	o.GuestOK = types.BoolValue(true)             // NOT valid for TIMEMACHINE
	m := SMBModel{Purpose: types.StringValue("TIMEMACHINE_SHARE"), Options: optsObject(t, o)}

	purpose, bad, diags := m.invalidOptionKeys(context.Background())
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if purpose != "TIMEMACHINE_SHARE" {
		t.Errorf("purpose = %s", purpose)
	}
	got := map[string]bool{}
	for _, k := range bad {
		got[k] = true
	}
	if !got["recyclebin"] || !got["guestok"] {
		t.Errorf("expected recyclebin+guestok flagged, got %v", bad)
	}
	if got["auto_dataset_creation"] {
		t.Errorf("auto_dataset_creation is valid for TIMEMACHINE, should not be flagged")
	}
}

// All-valid options for a purpose produce no findings.
func TestInvalidOptionKeys_AllValid(t *testing.T) {
	o := nullOpts()
	o.HostsAllow = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("10.0.0.0/8")})
	o.AaplNameMangling = types.BoolValue(true)
	m := SMBModel{Purpose: types.StringValue("DEFAULT_SHARE"), Options: optsObject(t, o)}

	_, bad, diags := m.invalidOptionKeys(context.Background())
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if len(bad) != 0 {
		t.Errorf("expected no findings, got %v", bad)
	}
}

// Unset purpose defaults to LEGACY_SHARE, where recyclebin IS valid.
func TestInvalidOptionKeys_LegacyDefault(t *testing.T) {
	o := nullOpts()
	o.Recyclebin = types.BoolValue(true)
	m := SMBModel{Purpose: types.StringNull(), Options: optsObject(t, o)}

	purpose, bad, diags := m.invalidOptionKeys(context.Background())
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if purpose != "LEGACY_SHARE" || len(bad) != 0 {
		t.Errorf("purpose=%s bad=%v; want LEGACY_SHARE with no findings", purpose, bad)
	}
}

// Unknown (interpolated) purpose skips validation.
func TestInvalidOptionKeys_UnknownPurposeSkips(t *testing.T) {
	o := nullOpts()
	o.Recyclebin = types.BoolValue(true)
	m := SMBModel{Purpose: types.StringUnknown(), Options: optsObject(t, o)}

	_, bad, diags := m.invalidOptionKeys(context.Background())
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if len(bad) != 0 {
		t.Errorf("unknown purpose should skip; got %v", bad)
	}
}

// overlayFlatOnOptions copies known flat legacy values onto the options object
// (pinned from state) so the payload sends them instead of the stale pinned
// values (B-011). Unknown flat values leave the pinned value alone.
func TestOverlayFlatOnOptions(t *testing.T) {
	ctx := context.Background()
	o := nullOpts()
	o.GuestOK = types.BoolValue(false)
	o.Streams = types.BoolValue(true)
	o.HostsAllow = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("10.0.0.0/8")})
	o.VUID = types.StringValue("abc")

	m := baseLegacyModel()
	m.Purpose = types.StringValue(legacySharePurpose)
	m.Options = optsObject(t, o)
	m.GuestOK = types.BoolValue(true)
	m.Streams = types.BoolUnknown()
	m.HostsAllow = types.ListValueMust(types.StringType, []attr.Value{types.StringValue("192.168.0.0/16")})

	obj, changed, diags := m.overlayFlatOnOptions(ctx)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	var got SMBOptionsModel
	obj.As(ctx, &got, basetypes.ObjectAsOptions{})
	if !got.GuestOK.ValueBool() {
		t.Error("options.guestok not overlaid from flat guestok")
	}
	if !got.Streams.ValueBool() {
		t.Error("options.streams changed although flat streams is unknown")
	}
	if got.VUID.ValueString() != "abc" {
		t.Errorf("options.vuid = %q, want abc (kept from state)", got.VUID.ValueString())
	}

	m.Options = obj
	payload, d := m.apiPayload(ctx)
	if d.HasError() {
		t.Fatalf("apiPayload: %v", d)
	}
	opts := payload["options"].(map[string]any)
	if opts["guestok"] != true {
		t.Errorf("payload options.guestok = %v, want true", opts["guestok"])
	}
	if ha := opts["hostsallow"].([]string); len(ha) != 1 || ha[0] != "192.168.0.0/16" {
		t.Errorf("payload options.hostsallow = %v, want [192.168.0.0/16]", ha)
	}

	// Overlaying again is a no-op.
	if _, changed, _ := m.overlayFlatOnOptions(ctx); changed {
		t.Error("second overlay reported a change")
	}
}

// configuredFlatLegacyKeys reports every non-null flat legacy attribute,
// including unknown (interpolated) ones.
func TestConfiguredFlatLegacyKeys(t *testing.T) {
	m := SMBModel{
		Recyclebin:       types.BoolNull(),
		GuestOK:          types.BoolValue(true),
		HostsAllow:       types.ListUnknown(types.StringType),
		HostsDeny:        types.ListNull(types.StringType),
		ACL:              types.BoolNull(),
		DurableHandle:    types.BoolNull(),
		Streams:          types.BoolNull(),
		TimeMachine:      types.BoolNull(),
		TimeMachineQuota: types.Int64Value(0),
		Home:             types.BoolNull(),
	}
	got := m.configuredFlatLegacyKeys()
	want := []string{"guestok", "hostsallow", "timemachine_quota"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

// Unset (unknown) top-level Optional+Computed fields are omitted, not sent as
// their zero value: sending enabled=false/browsable=false created every share
// without an explicit `enabled = true` disabled and hidden.
func TestSMBApiPayload_UnknownTopLevelOmitted(t *testing.T) {
	m := baseLegacyModel()
	m.Comment = types.StringUnknown()
	m.Enabled = types.BoolUnknown()
	m.Browsable = types.BoolUnknown()
	m.ReadOnly = types.BoolUnknown()
	m.ABE = types.BoolNull()
	payload, diags := m.apiPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("apiPayload: %v", diags)
	}
	for _, k := range []string{"comment", "enabled", "browsable", "readonly", "access_based_share_enumeration"} {
		if v, ok := payload[k]; ok {
			t.Errorf("payload[%s] = %v, want omitted", k, v)
		}
	}
}
