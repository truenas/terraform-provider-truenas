package smb

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// TestSMBApiPayload verifies that apiPayload produces a map with the expected
// 19 keys and correct values.
func TestSMBApiPayload(t *testing.T) {
	ctx := context.Background()

	m := SMBModel{
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
		Purpose:          types.StringValue("DEFAULT_SHARE"),
		VUID:             types.StringValue(""),
		Locked:           types.BoolValue(false),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	expectedKeys := []string{
		"path", "name", "comment", "ro", "browsable", "recyclebin",
		"guestok", "hostsallow", "hostsdeny", "abe", "acl", "durablehandle",
		"streams", "timemachine", "timemachine_quota", "enabled", "home", "purpose",
	}
	if len(payload) != len(expectedKeys) {
		t.Errorf("payload has %d keys, want %d", len(payload), len(expectedKeys))
	}
	for _, k := range expectedKeys {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}

	// vuid and locked must NOT be in the payload.
	for _, forbidden := range []string{"vuid", "locked", "id"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("payload should not contain key %q", forbidden)
		}
	}

	if payload["path"] != "/mnt/tank/share" {
		t.Errorf("payload[path] = %v, want /mnt/tank/share", payload["path"])
	}
	if payload["name"] != "myshare" {
		t.Errorf("payload[name] = %v, want myshare", payload["name"])
	}
	if payload["purpose"] != "DEFAULT_SHARE" {
		t.Errorf("payload[purpose] = %v, want DEFAULT_SHARE", payload["purpose"])
	}
	if payload["timemachine_quota"] != int64(0) {
		t.Errorf("payload[timemachine_quota] = %v, want 0", payload["timemachine_quota"])
	}

	hostsAllow, ok := payload["hostsallow"].([]string)
	if !ok {
		t.Fatalf("payload[hostsallow] is %T, want []string", payload["hostsallow"])
	}
	if len(hostsAllow) != 1 || hostsAllow[0] != "192.168.1.0/24" {
		t.Errorf("payload[hostsallow] = %v, want [192.168.1.0/24]", hostsAllow)
	}

	hostsDeny, ok := payload["hostsdeny"].([]string)
	if !ok {
		t.Fatalf("payload[hostsdeny] is %T, want []string", payload["hostsdeny"])
	}
	if len(hostsDeny) != 0 {
		t.Errorf("payload[hostsdeny] = %v, want []", hostsDeny)
	}
}

// TestSMBApiPayload_NullLists verifies that nil hostsallow/hostsdeny default to
// empty slices (not nil) in the payload.
func TestSMBApiPayload_NullLists(t *testing.T) {
	ctx := context.Background()

	m := SMBModel{
		Path:             types.StringValue("/mnt/tank/share"),
		Name:             types.StringValue("share"),
		Comment:          types.StringValue(""),
		ReadOnly:         types.BoolValue(false),
		Browsable:        types.BoolValue(true),
		Recyclebin:       types.BoolValue(false),
		GuestOK:          types.BoolValue(false),
		HostsAllow:       types.ListNull(types.StringType),
		HostsDeny:        types.ListNull(types.StringType),
		ABE:              types.BoolValue(false),
		ACL:              types.BoolValue(true),
		DurableHandle:    types.BoolValue(true),
		Streams:          types.BoolValue(true),
		TimeMachine:      types.BoolValue(false),
		TimeMachineQuota: types.Int64Value(0),
		Enabled:          types.BoolValue(true),
		Home:             types.BoolValue(false),
		Purpose:          types.StringValue("NO_PRESET"),
		VUID:             types.StringValue(""),
		Locked:           types.BoolValue(false),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	ha, ok := payload["hostsallow"].([]string)
	if !ok {
		t.Fatalf("payload[hostsallow] is %T, want []string", payload["hostsallow"])
	}
	if ha == nil {
		t.Error("payload[hostsallow] must not be nil, want empty slice")
	}

	hd, ok := payload["hostsdeny"].([]string)
	if !ok {
		t.Fatalf("payload[hostsdeny] is %T, want []string", payload["hostsdeny"])
	}
	if hd == nil {
		t.Error("payload[hostsdeny] must not be nil, want empty slice")
	}
}

// TestSMBApiPayload_PurposeOmittedWhenNull verifies that apiPayload omits the
// "purpose" key entirely when Purpose is null (unset in config), rather than
// sending an empty string. This is a regression test for a live failure on
// SCALE 26.0: sharing.smb.create/update returns EINVAL when purpose is "",
// since 26.0 requires purpose to be one of a known, non-empty enum.
func TestSMBApiPayload_PurposeOmittedWhenNull(t *testing.T) {
	ctx := context.Background()

	m := SMBModel{
		Path:             types.StringValue("/mnt/tank/share"),
		Name:             types.StringValue("share"),
		Comment:          types.StringValue(""),
		ReadOnly:         types.BoolValue(false),
		Browsable:        types.BoolValue(true),
		Recyclebin:       types.BoolValue(false),
		GuestOK:          types.BoolValue(false),
		HostsAllow:       types.ListValueMust(types.StringType, []attr.Value{}),
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

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if v, ok := payload["purpose"]; ok {
		t.Errorf("payload should omit 'purpose' when unset, got %v", v)
	}
}

// TestSMBApiPayload_PurposeOmittedWhenUnrecognized verifies that apiPayload
// omits "purpose" when set to a value outside the current (26.0) enum, e.g.
// a stale 24.x preset like NO_PRESET, rather than sending it unconditionally
// and tripping EINVAL server-side.
func TestSMBApiPayload_PurposeOmittedWhenUnrecognized(t *testing.T) {
	ctx := context.Background()

	m := SMBModel{
		Path:             types.StringValue("/mnt/tank/share"),
		Name:             types.StringValue("share"),
		Comment:          types.StringValue(""),
		ReadOnly:         types.BoolValue(false),
		Browsable:        types.BoolValue(true),
		Recyclebin:       types.BoolValue(false),
		GuestOK:          types.BoolValue(false),
		HostsAllow:       types.ListValueMust(types.StringType, []attr.Value{}),
		HostsDeny:        types.ListValueMust(types.StringType, []attr.Value{}),
		ABE:              types.BoolValue(false),
		ACL:              types.BoolValue(true),
		DurableHandle:    types.BoolValue(true),
		Streams:          types.BoolValue(true),
		TimeMachine:      types.BoolValue(false),
		TimeMachineQuota: types.Int64Value(0),
		Enabled:          types.BoolValue(true),
		Home:             types.BoolValue(false),
		Purpose:          types.StringValue("NO_PRESET"),
		VUID:             types.StringValue(""),
		Locked:           types.BoolValue(false),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if v, ok := payload["purpose"]; ok {
		t.Errorf("payload should omit 'purpose' for unrecognized value NO_PRESET, got %v", v)
	}
}

// TestSMBApiPayload_PurposeIncludedWhenKnown verifies apiPayload includes a
// valid 26.0 purpose value, e.g. TIMEMACHINE_SHARE.
func TestSMBApiPayload_PurposeIncludedWhenKnown(t *testing.T) {
	ctx := context.Background()

	m := SMBModel{
		Path:             types.StringValue("/mnt/tank/share"),
		Name:             types.StringValue("share"),
		Comment:          types.StringValue(""),
		ReadOnly:         types.BoolValue(false),
		Browsable:        types.BoolValue(true),
		Recyclebin:       types.BoolValue(false),
		GuestOK:          types.BoolValue(false),
		HostsAllow:       types.ListValueMust(types.StringType, []attr.Value{}),
		HostsDeny:        types.ListValueMust(types.StringType, []attr.Value{}),
		ABE:              types.BoolValue(false),
		ACL:              types.BoolValue(true),
		DurableHandle:    types.BoolValue(true),
		Streams:          types.BoolValue(true),
		TimeMachine:      types.BoolValue(false),
		TimeMachineQuota: types.Int64Value(0),
		Enabled:          types.BoolValue(true),
		Home:             types.BoolValue(false),
		Purpose:          types.StringValue("TIMEMACHINE_SHARE"),
		VUID:             types.StringValue(""),
		Locked:           types.BoolValue(false),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if v, ok := payload["purpose"]; !ok {
		t.Error("payload missing 'purpose' for known value TIMEMACHINE_SHARE")
	} else if v != "TIMEMACHINE_SHARE" {
		t.Errorf("payload[purpose] = %v, want TIMEMACHINE_SHARE", v)
	}
}

// TestResponseToModel verifies that responseToModel populates all fields from
// the smbAPI struct correctly.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &smbAPI{
		ID:               42,
		Path:             "/mnt/tank/share",
		Name:             "myshare",
		Comment:          "test",
		ReadOnly:         true,
		Browsable:        true,
		Recyclebin:       false,
		GuestOK:          false,
		HostsAllow:       []string{"10.0.0.1"},
		HostsDeny:        []string{},
		ABE:              false,
		ACL:              true,
		DurableHandle:    true,
		Streams:          true,
		TimeMachine:      false,
		TimeMachineQuota: 100,
		Enabled:          true,
		Home:             false,
		Purpose:          "DEFAULT_SHARE",
		VUID:             "abc-123",
		Locked:           false,
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
	if m.VUID.ValueString() != "abc-123" {
		t.Errorf("VUID = %v, want abc-123", m.VUID.ValueString())
	}
	if m.TimeMachineQuota.ValueInt64() != 100 {
		t.Errorf("TimeMachineQuota = %v, want 100", m.TimeMachineQuota.ValueInt64())
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
