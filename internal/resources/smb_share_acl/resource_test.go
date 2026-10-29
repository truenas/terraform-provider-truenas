// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

func strptr(s string) *string { return &s }

// mkEntry builds one share_acl list-element object. Pass a nil selector to
// leave it unset; whoID as a nil pointer leaves ae_who_id null.
func mkEntry(perm, atype string, sid *string, whoID *smbAclWhoIDAPI, str *string) types.Object {
	sidVal := normalize.NewCaseInsensitivePointerValue(sid)
	strVal := types.StringNull()
	if str != nil {
		strVal = types.StringValue(*str)
	}
	idVal := types.ObjectNull(smbAclWhoIDAttrTypes)
	if whoID != nil {
		idVal = types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
			"id_type": types.StringValue(whoID.IDType),
			"id":      types.Int64Value(whoID.ID),
		})
	}
	return types.ObjectValueMust(smbAclEntryAttrTypes, map[string]attr.Value{
		"ae_perm":    types.StringValue(perm),
		"ae_type":    types.StringValue(atype),
		"ae_who_sid": sidVal,
		"ae_who_id":  idVal,
		"ae_who_str": strVal,
	})
}

func mkModel(shareName string, entries ...types.Object) *SMBShareACLModel {
	elems := make([]attr.Value, len(entries))
	for i, e := range entries {
		elems[i] = e
	}
	return &SMBShareACLModel{
		ShareName: types.StringValue(shareName),
		ShareACL:  types.ListValueMust(smbAclEntryObjectType(), elems),
	}
}

func TestSetaclPayload_SIDOnly(t *testing.T) {
	m := mkModel("share1", mkEntry("FULL", "ALLOWED", strptr("S-1-1-0"), nil, nil))
	payload, diags := setaclPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	if payload["share_name"] != "share1" {
		t.Errorf("share_name = %v, want share1", payload["share_name"])
	}
	acl := payload["share_acl"].([]map[string]any)
	if len(acl) != 1 {
		t.Fatalf("len(acl) = %d, want 1", len(acl))
	}
	e := acl[0]
	if e["ae_perm"] != "FULL" || e["ae_type"] != "ALLOWED" || e["ae_who_sid"] != "S-1-1-0" {
		t.Errorf("entry = %+v", e)
	}
	if _, ok := e["ae_who_id"]; ok {
		t.Errorf("ae_who_id should be omitted, got %+v", e)
	}
	if _, ok := e["ae_who_str"]; ok {
		t.Errorf("ae_who_str should be omitted, got %+v", e)
	}
}

func TestSetaclPayload_WhoID(t *testing.T) {
	m := mkModel("share1", mkEntry("CHANGE", "ALLOWED", nil, &smbAclWhoIDAPI{IDType: "GROUP", ID: 1000}, nil))
	payload, diags := setaclPayload(context.Background(), m)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	acl := payload["share_acl"].([]map[string]any)
	who, ok := acl[0]["ae_who_id"].(map[string]any)
	if !ok {
		t.Fatalf("ae_who_id missing/wrong type: %+v", acl[0])
	}
	if who["id_type"] != "GROUP" || who["id"] != int64(1000) {
		t.Errorf("ae_who_id = %+v", who)
	}
	if _, ok := acl[0]["ae_who_sid"]; ok {
		t.Errorf("ae_who_sid should be omitted")
	}
}

// The server reports SIDs upper-case: a configured lower-case spelling of the
// same SID keeps its spelling (B-074), and the server-resolved name is stored.
func TestMergeEntries_SIDCaseKept(t *testing.T) {
	prior := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: normalize.NewCaseInsensitiveValue("s-1-1-0"), AeWhoID: types.ObjectUnknown(smbAclWhoIDAttrTypes), AeWhoStr: types.StringUnknown(),
	}}
	api := []smbAclEntryAPI{{AePerm: "FULL", AeType: "ALLOWED", AeWhoSID: strptr("S-1-1-0"), AeWhoStr: strptr("everyone@")}}
	got := mergeEntries(prior, api, true)
	if got[0].AeWhoSID.ValueString() != "s-1-1-0" {
		t.Errorf("ae_who_sid = %q, want the configured s-1-1-0", got[0].AeWhoSID.ValueString())
	}
	if got[0].AeWhoStr.ValueString() != "everyone@" {
		t.Errorf("ae_who_str = %q, want everyone@", got[0].AeWhoStr.ValueString())
	}
	if !got[0].AeWhoID.IsNull() {
		t.Errorf("ae_who_id = %v, want null", got[0].AeWhoID)
	}
}

// ae_perm/ae_type and a different principal always come from the server, so
// drift shows in the plan.
func TestMergeEntries_DriftFromServer(t *testing.T) {
	prior := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: normalize.NewCaseInsensitiveValue("S-1-1-0"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes), AeWhoStr: types.StringValue("everyone@"),
	}}
	api := []smbAclEntryAPI{
		{AePerm: "CHANGE", AeType: "DENIED", AeWhoSID: strptr("S-1-5-32-544")},
		{AePerm: "READ", AeType: "ALLOWED", AeWhoSID: strptr("S-1-5-32-546")},
	}
	got := mergeEntries(prior, api, false)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].AePerm.ValueString() != "CHANGE" || got[0].AeType.ValueString() != "DENIED" {
		t.Errorf("perm/type = %s/%s, want CHANGE/DENIED", got[0].AePerm.ValueString(), got[0].AeType.ValueString())
	}
	if got[0].AeWhoSID.ValueString() != "S-1-5-32-544" || !got[0].AeWhoStr.IsNull() {
		t.Errorf("principal = %q/%v, want the server's S-1-5-32-544 and a null name", got[0].AeWhoSID.ValueString(), got[0].AeWhoStr)
	}
}

// An entry configured by Unix ID keeps it; the resolved SID and name are
// stored too, so an import (all three fields) matches (B-123). The server may
// report the id_type as BOTH, and a null ID while winbindd is down.
func TestMergeEntries_WhoIDKept(t *testing.T) {
	cfgID := types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
		"id_type": types.StringValue("GROUP"), "id": types.Int64Value(545),
	})
	prior := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: normalize.NewCaseInsensitiveUnknown(), AeWhoID: cfgID, AeWhoStr: types.StringUnknown(),
	}}
	for _, who := range []*smbAclWhoIDAPI{{IDType: "BOTH", ID: 545}, nil} {
		api := []smbAclEntryAPI{{AePerm: "FULL", AeType: "ALLOWED", AeWhoSID: strptr("S-1-5-32-545"), AeWhoID: who, AeWhoStr: strptr("builtin_users")}}
		got := mergeEntries(prior, api, true)
		if !got[0].AeWhoID.Equal(cfgID) {
			t.Errorf("api id %v: ae_who_id = %v, want the configured GROUP 545", who, got[0].AeWhoID)
		}
		if got[0].AeWhoSID.ValueString() != "S-1-5-32-545" || got[0].AeWhoStr.ValueString() != "builtin_users" {
			t.Errorf("api id %v: resolved sid/name = %q/%q", who, got[0].AeWhoSID.ValueString(), got[0].AeWhoStr.ValueString())
		}
	}
	// On Read (trustIndex false) an unresolved API ID is matched through the
	// stored SID.
	state := []smbAclEntryModel{{
		AePerm: types.StringValue("FULL"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: normalize.NewCaseInsensitiveValue("S-1-5-32-545"), AeWhoID: cfgID, AeWhoStr: types.StringValue("builtin_users"),
	}}
	api := []smbAclEntryAPI{{AePerm: "FULL", AeType: "ALLOWED", AeWhoSID: strptr("S-1-5-32-545")}}
	got := mergeEntries(state, api, false)
	if !got[0].AeWhoID.Equal(cfgID) || got[0].AeWhoStr.ValueString() != "builtin_users" {
		t.Errorf("read with winbindd down: id=%v str=%v, want state values kept", got[0].AeWhoID, got[0].AeWhoStr)
	}
	// A different ID at the same index is not the same principal.
	other := []smbAclEntryAPI{{AePerm: "FULL", AeType: "ALLOWED", AeWhoSID: strptr("S-1-5-21-1-2-3-1000"), AeWhoID: &smbAclWhoIDAPI{IDType: "USER", ID: 3000}}}
	got = mergeEntries(prior, other, true)
	if _, id, _ := whoIDOf(got[0].AeWhoID); id != 3000 {
		t.Errorf("different principal: id = %d, want the server's 3000", id)
	}
}

func TestConfiguredPrincipalEqual(t *testing.T) {
	gid := func(n int64) types.Object {
		return types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
			"id_type": types.StringValue("GROUP"), "id": types.Int64Value(n),
		})
	}
	state := smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveValue("S-1-5-32-545"), AeWhoID: gid(545), AeWhoStr: types.StringValue("builtin_users")}
	cases := []struct {
		name string
		cfg  smbAclEntryModel
		want bool
	}{
		{"same sid", smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveValue("S-1-5-32-545"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes)}, true},
		{"same id", smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveNull(), AeWhoID: gid(545)}, true},
		{"other id", smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveNull(), AeWhoID: gid(546)}, false},
		{"other sid", smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveValue("S-1-1-0"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes)}, false},
		{"unknown sid", smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveUnknown(), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes)}, false},
	}
	for _, c := range cases {
		if got := configuredPrincipalEqual(c.cfg, state); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestApiEntriesToList_AllResolvedFields(t *testing.T) {
	api := []smbAclEntryAPI{{
		AePerm: "FULL", AeType: "ALLOWED",
		AeWhoSID: strptr("S-1-1-0"), AeWhoStr: strptr("everyone@"),
	}}
	list, diags := apiEntriesToList(context.Background(), api)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	var got []smbAclEntryModel
	if d := list.ElementsAs(context.Background(), &got, false); d.HasError() {
		t.Fatalf("ElementsAs: %s", diagText(d))
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].AeWhoSID.ValueString() != "S-1-1-0" || got[0].AeWhoStr.ValueString() != "everyone@" {
		t.Errorf("sid/str = %q/%q, want S-1-1-0/everyone@", got[0].AeWhoSID.ValueString(), got[0].AeWhoStr.ValueString())
	}
}

func TestApiEntriesToList_WhoIDOnly(t *testing.T) {
	api := []smbAclEntryAPI{{
		AePerm: "READ", AeType: "DENIED",
		AeWhoID: &smbAclWhoIDAPI{IDType: "USER", ID: 3001},
	}}
	list, diags := apiEntriesToList(context.Background(), api)
	if diags.HasError() {
		t.Fatalf("unexpected diags: %s", diagText(diags))
	}
	var got []smbAclEntryModel
	if d := list.ElementsAs(context.Background(), &got, false); d.HasError() {
		t.Fatalf("ElementsAs: %s", diagText(d))
	}
	if got[0].AeWhoID.IsNull() {
		t.Fatal("ae_who_id should be set")
	}
	attrs := got[0].AeWhoID.Attributes()
	if attrs["id_type"].(types.String).ValueString() != "USER" || attrs["id"].(types.Int64).ValueInt64() != 3001 {
		t.Errorf("ae_who_id = %+v", attrs)
	}
}

func TestCountSelectors(t *testing.T) {
	none := smbAclEntryModel{AeWhoSID: normalize.NewCaseInsensitiveNull(), AeWhoStr: types.StringNull(), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes)}
	if got := countSelectors(none); got != 0 {
		t.Errorf("no selector: got %d, want 0", got)
	}
	sid := none
	sid.AeWhoSID = normalize.NewCaseInsensitiveValue("S-1-1-0")
	if got := countSelectors(sid); got != 1 {
		t.Errorf("sid only: got %d, want 1", got)
	}
	two := sid
	two.AeWhoID = types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
		"id_type": types.StringValue("GROUP"), "id": types.Int64Value(545),
	})
	if got := countSelectors(two); got != 2 {
		t.Errorf("two selectors: got %d, want 2", got)
	}
	// unknown counts as set (cross-resource reference)
	unk := none
	unk.AeWhoID = types.ObjectUnknown(smbAclWhoIDAttrTypes)
	if got := countSelectors(unk); got != 1 {
		t.Errorf("unknown selector: got %d, want 1", got)
	}
	// ae_who_str is read-only and never counts.
	str := none
	str.AeWhoStr = types.StringValue("everyone@")
	if got := countSelectors(str); got != 0 {
		t.Errorf("name only: got %d, want 0", got)
	}
}

func TestDefaultShareACLPayload(t *testing.T) {
	p := defaultShareACLPayload("share1")
	if p["share_name"] != "share1" {
		t.Errorf("share_name = %v", p["share_name"])
	}
	acl := p["share_acl"].([]map[string]any)
	if len(acl) != 1 || acl[0]["ae_perm"] != "FULL" || acl[0]["ae_type"] != "ALLOWED" || acl[0]["ae_who_sid"] != "S-1-1-0" {
		t.Errorf("default acl = %+v", acl)
	}
}

// A planned null ID/name (everyone@ has none) is kept after the apply even if
// the server resolves one now; the plan promised null (import with a
// lower-case SID planned no change only once the null ID was planned).
func TestMergeEntries_PlannedNullKept(t *testing.T) {
	prior := []smbAclEntryModel{{
		AePerm: types.StringValue("READ"), AeType: types.StringValue("ALLOWED"),
		AeWhoSID: normalize.NewCaseInsensitiveValue("S-1-1-0"), AeWhoID: types.ObjectNull(smbAclWhoIDAttrTypes), AeWhoStr: types.StringNull(),
	}}
	api := []smbAclEntryAPI{{AePerm: "READ", AeType: "ALLOWED", AeWhoSID: strptr("S-1-1-0"),
		AeWhoID: &smbAclWhoIDAPI{IDType: "GROUP", ID: 5}, AeWhoStr: strptr("everyone@")}}
	got := mergeEntries(prior, api, true)
	if !got[0].AeWhoID.IsNull() || !got[0].AeWhoStr.IsNull() {
		t.Errorf("apply: id=%v str=%v, want the planned nulls", got[0].AeWhoID, got[0].AeWhoStr)
	}
	got = mergeEntries(prior, api, false)
	if got[0].AeWhoID.IsNull() || got[0].AeWhoStr.ValueString() != "everyone@" {
		t.Errorf("read: id=%v str=%v, want the server values", got[0].AeWhoID, got[0].AeWhoStr)
	}
}
