// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUPSConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestUPSConfigSchema_IDIsComputed(t *testing.T) {
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

// TestUPSConfigSchema_MonPwdIsSensitiveWriteOnly verifies that "monpwd" is
// Optional + Sensitive, and specifically NOT Computed (write-only: never
// read back from TrueNAS, so it must not participate in drift detection).
func TestUPSConfigSchema_MonPwdIsSensitiveWriteOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["monpwd"]
	if !ok {
		t.Fatal("schema missing 'monpwd' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'monpwd' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() {
		t.Error("'monpwd' should be Optional")
	}
	if !strAttr.IsSensitive() {
		t.Error("'monpwd' should be Sensitive")
	}
	if strAttr.IsComputed() {
		t.Error("'monpwd' should NOT be Computed (write-only, never returned by the API)")
	}
	if strAttr.IsRequired() {
		t.Error("'monpwd' should not be Required")
	}
	if !strAttr.IsWriteOnly() {
		t.Error("'monpwd' should be WriteOnly")
	}
}

// TestUPSConfigSchema_CompleteIdentifierIsComputedOnly verifies that
// "complete_identifier" is Computed-only with UseStateForUnknown plan modifiers
// (it is server-derived and should never be sent to ups.update).
func TestUPSConfigSchema_CompleteIdentifierIsComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["complete_identifier"]
	if !ok {
		t.Fatal("schema missing 'complete_identifier' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'complete_identifier' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsComputed() {
		t.Error("'complete_identifier' should be Computed")
	}
	if strAttr.IsOptional() || strAttr.IsRequired() {
		t.Error("'complete_identifier' should be Computed-only (not Optional/Required)")
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("'complete_identifier' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestUPSConfigSchema_OtherFieldsAreOptionalComputed verifies that every
// non-secret, non-id, non-complete_identifier field is Optional+Computed
// with UseStateForUnknown plan modifiers.
func TestUPSConfigSchema_OtherFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	stringFields := []string{
		"identifier", "mode", "remotehost", "driver", "port", "options",
		"optionsupsd", "description", "shutdown", "shutdowncmd", "monuser",
		"extrausers",
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
		if strAttr.IsSensitive() {
			t.Errorf("%q should not be Sensitive", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	int64Fields := []string{"remoteport", "shutdowntimer", "hostsync", "nocommwarntime"}
	for _, name := range int64Fields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.Int64Attribute", name, attr)
		}
		if !intAttr.IsOptional() || !intAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(intAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	for _, name := range []string{"rmonitor", "powerdown"} {
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
}

// TestResponseToModel_MonPwdNeverSet verifies that responseToModel never
// writes to m.MonPwd, regardless of its prior value: monpwd is write-only
// and the API never returns a usable value for it.
func TestResponseToModel_MonPwdNeverSet(t *testing.T) {
	api := &upsConfigAPI{
		ID:         1,
		Identifier: "ups",
		Mode:       "MASTER",
		MonUser:    "monuser",
	}

	m := &UPSConfigModel{MonPwd: types.StringValue("super-secret")}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.MonPwd.ValueString() != "super-secret" {
		t.Errorf("MonPwd = %q, want unchanged %q", m.MonPwd.ValueString(), "super-secret")
	}

	m2 := &UPSConfigModel{MonPwd: types.StringNull()}
	diags = responseToModel(api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m2.MonPwd.IsNull() {
		t.Errorf("MonPwd = %v, want still null", m2.MonPwd)
	}
}

// TestResponseToModel_ShutdownCmdNilBecomesEmptyString verifies that a nil
// shutdowncmd from the API maps to an empty string in the model.
func TestResponseToModel_ShutdownCmdNilBecomesEmptyString(t *testing.T) {
	api := &upsConfigAPI{ID: 1, ShutdownCmd: nil}

	m := &UPSConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ShutdownCmd.IsNull() {
		t.Error("ShutdownCmd should not be null when API returns nil, want empty string")
	}
	if m.ShutdownCmd.ValueString() != "" {
		t.Errorf("ShutdownCmd = %q, want empty string", m.ShutdownCmd.ValueString())
	}
}

// TestResponseToModel_ShutdownCmdSet verifies that a non-nil shutdowncmd
// from the API is carried through as-is.
func TestResponseToModel_ShutdownCmdSet(t *testing.T) {
	cmd := "/sbin/shutdown -h now"
	api := &upsConfigAPI{ID: 1, ShutdownCmd: &cmd}

	m := &UPSConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ShutdownCmd.ValueString() != cmd {
		t.Errorf("ShutdownCmd = %q, want %q", m.ShutdownCmd.ValueString(), cmd)
	}
	if m.ID.ValueString() != upsConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), upsConfigResourceID)
	}
}

// TestResponseToModel_NoCommWarnTimeNilBecomesZero verifies that a nil
// nocommwarntime from the API maps to 0 in the model.
func TestResponseToModel_NoCommWarnTimeNilBecomesZero(t *testing.T) {
	api := &upsConfigAPI{ID: 1, NoCommWarnTime: nil}

	m := &UPSConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.NoCommWarnTime.IsNull() {
		t.Error("NoCommWarnTime should not be null when API returns nil, want 0")
	}
	if m.NoCommWarnTime.ValueInt64() != 0 {
		t.Errorf("NoCommWarnTime = %d, want 0", m.NoCommWarnTime.ValueInt64())
	}
}

// TestResponseToModel_NoCommWarnTimeSet verifies that a non-nil
// nocommwarntime from the API is carried through as-is.
func TestResponseToModel_NoCommWarnTimeSet(t *testing.T) {
	v := int64(300)
	api := &upsConfigAPI{ID: 1, NoCommWarnTime: &v}

	m := &UPSConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.NoCommWarnTime.ValueInt64() != 300 {
		t.Errorf("NoCommWarnTime = %d, want 300", m.NoCommWarnTime.ValueInt64())
	}
}

// TestResponseToModel_CompleteIdentifierSet verifies that complete_identifier
// is carried through from the API response as-is.
func TestResponseToModel_CompleteIdentifierSet(t *testing.T) {
	api := &upsConfigAPI{ID: 1, CompleteIdentifier: "ups@localhost"}

	m := &UPSConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.CompleteIdentifier.ValueString() != "ups@localhost" {
		t.Errorf("CompleteIdentifier = %q, want %q", m.CompleteIdentifier.ValueString(), "ups@localhost")
	}
}

// TestResponseToDataSourceModel_NilPointersBecomeZeroValues mirrors the same
// nil-to-zero-value handling for the datasource model.
func TestResponseToDataSourceModel_NilPointersBecomeZeroValues(t *testing.T) {
	api := &upsConfigAPI{ID: 1, ShutdownCmd: nil, NoCommWarnTime: nil}

	m := &UPSConfigDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ShutdownCmd.ValueString() != "" {
		t.Errorf("ShutdownCmd = %q, want empty string", m.ShutdownCmd.ValueString())
	}
	if m.NoCommWarnTime.ValueInt64() != 0 {
		t.Errorf("NoCommWarnTime = %d, want 0", m.NoCommWarnTime.ValueInt64())
	}
	if m.ID.ValueString() != upsConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), upsConfigResourceID)
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// any field whose model value is null or unknown, for every guarded
// non-nullable, non-secret field.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &UPSConfigModel{
		Identifier:     types.StringNull(),
		Mode:           types.StringValue("MASTER"),
		RemoteHost:     types.StringUnknown(),
		RemotePort:     types.Int64Value(3493),
		Driver:         types.StringNull(),
		Port:           types.StringUnknown(),
		Options:        types.StringNull(),
		OptionsUPSD:    types.StringNull(),
		Description:    types.StringNull(),
		Shutdown:       types.StringValue("LOWBATT"),
		ShutdownTimer:  types.Int64Unknown(),
		MonUser:        types.StringNull(),
		MonPwd:         types.StringNull(),
		ExtraUsers:     types.StringNull(),
		RMonitor:       types.BoolUnknown(),
		PowerDown:      types.BoolNull(),
		HostSync:       types.Int64Value(15),
		ShutdownCmd:    types.StringNull(),
		NoCommWarnTime: types.Int64Null(),
	}

	p := m.updatePayload()

	if _, ok := p["identifier"]; ok {
		t.Error("expected 'identifier' to be omitted (null)")
	}
	if v, ok := p["mode"]; !ok || v != "MASTER" {
		t.Errorf("expected 'mode' = 'MASTER', got %v (present=%v)", v, ok)
	}
	if _, ok := p["remotehost"]; ok {
		t.Error("expected 'remotehost' to be omitted (unknown)")
	}
	if v, ok := p["remoteport"]; !ok || v != int64(3493) {
		t.Errorf("expected 'remoteport' = 3493, got %v (present=%v)", v, ok)
	}
	if _, ok := p["driver"]; ok {
		t.Error("expected 'driver' to be omitted (null)")
	}
	if _, ok := p["port"]; ok {
		t.Error("expected 'port' to be omitted (unknown)")
	}
	if _, ok := p["options"]; ok {
		t.Error("expected 'options' to be omitted (null)")
	}
	if _, ok := p["optionsupsd"]; ok {
		t.Error("expected 'optionsupsd' to be omitted (null)")
	}
	if _, ok := p["description"]; ok {
		t.Error("expected 'description' to be omitted (null)")
	}
	if v, ok := p["shutdown"]; !ok || v != "LOWBATT" {
		t.Errorf("expected 'shutdown' = 'LOWBATT', got %v (present=%v)", v, ok)
	}
	if _, ok := p["shutdowntimer"]; ok {
		t.Error("expected 'shutdowntimer' to be omitted (unknown)")
	}
	if _, ok := p["monuser"]; ok {
		t.Error("expected 'monuser' to be omitted (null)")
	}
	if _, ok := p["extrausers"]; ok {
		t.Error("expected 'extrausers' to be omitted (null)")
	}
	if _, ok := p["rmonitor"]; ok {
		t.Error("expected 'rmonitor' to be omitted (unknown)")
	}
	if _, ok := p["powerdown"]; ok {
		t.Error("expected 'powerdown' to be omitted (null)")
	}
	if v, ok := p["hostsync"]; !ok || v != int64(15) {
		t.Errorf("expected 'hostsync' = 15, got %v (present=%v)", v, ok)
	}
	if _, ok := p["shutdowncmd"]; ok {
		t.Error("expected 'shutdowncmd' to be omitted (null)")
	}
	if _, ok := p["nocommwarntime"]; ok {
		t.Error("expected 'nocommwarntime' to be omitted (null)")
	}
	if _, ok := p["monpwd"]; ok {
		t.Error("expected 'monpwd' to be omitted (null)")
	}
}

// TestUpdatePayload_ShutdownCmdThreeWay verifies the three-way nullable
// handling for shutdowncmd: omitted when null/unknown, sent as nil when
// explicitly set to "", and sent as the value otherwise.
func TestUpdatePayload_ShutdownCmdThreeWay(t *testing.T) {
	base := func() *UPSConfigModel {
		return &UPSConfigModel{
			Mode:           types.StringValue("MASTER"),
			NoCommWarnTime: types.Int64Null(),
		}
	}

	m := base()
	m.ShutdownCmd = types.StringNull()
	p := m.updatePayload()
	if _, ok := p["shutdowncmd"]; ok {
		t.Error("expected 'shutdowncmd' to be omitted when null")
	}

	m2 := base()
	m2.ShutdownCmd = types.StringUnknown()
	p2 := m2.updatePayload()
	if _, ok := p2["shutdowncmd"]; ok {
		t.Error("expected 'shutdowncmd' to be omitted when unknown")
	}

	m3 := base()
	m3.ShutdownCmd = types.StringValue("")
	p3 := m3.updatePayload()
	if v, ok := p3["shutdowncmd"]; !ok {
		t.Error("expected 'shutdowncmd' to be present (nil) when explicitly set to empty string")
	} else if v != nil {
		t.Errorf("expected 'shutdowncmd' = nil when explicitly set to empty string, got %v", v)
	}

	m4 := base()
	m4.ShutdownCmd = types.StringValue("/sbin/shutdown -h now")
	p4 := m4.updatePayload()
	if v, ok := p4["shutdowncmd"]; !ok || v != "/sbin/shutdown -h now" {
		t.Errorf("expected 'shutdowncmd' = '/sbin/shutdown -h now', got %v (present=%v)", v, ok)
	}
}

// TestUpdatePayload_NoCommWarnTimeThreeWay verifies the three-way nullable
// handling for nocommwarntime: omitted when null/unknown, sent as nil when
// explicitly set to 0, and sent as the value otherwise.
func TestUpdatePayload_NoCommWarnTimeThreeWay(t *testing.T) {
	base := func() *UPSConfigModel {
		return &UPSConfigModel{
			Mode:        types.StringValue("MASTER"),
			ShutdownCmd: types.StringNull(),
		}
	}

	m := base()
	m.NoCommWarnTime = types.Int64Null()
	p := m.updatePayload()
	if _, ok := p["nocommwarntime"]; ok {
		t.Error("expected 'nocommwarntime' to be omitted when null")
	}

	m2 := base()
	m2.NoCommWarnTime = types.Int64Unknown()
	p2 := m2.updatePayload()
	if _, ok := p2["nocommwarntime"]; ok {
		t.Error("expected 'nocommwarntime' to be omitted when unknown")
	}

	m3 := base()
	m3.NoCommWarnTime = types.Int64Value(0)
	p3 := m3.updatePayload()
	if v, ok := p3["nocommwarntime"]; !ok {
		t.Error("expected 'nocommwarntime' to be present (nil) when explicitly set to 0")
	} else if v != nil {
		t.Errorf("expected 'nocommwarntime' = nil when explicitly set to 0, got %v", v)
	}

	m4 := base()
	m4.NoCommWarnTime = types.Int64Value(300)
	p4 := m4.updatePayload()
	if v, ok := p4["nocommwarntime"]; !ok || v != int64(300) {
		t.Errorf("expected 'nocommwarntime' = 300, got %v (present=%v)", v, ok)
	}
}

// TestUpdatePayload_MonPwdOnlyWhenSet verifies that monpwd is included only
// when it has a known, non-null value, and is otherwise omitted entirely
// (never sent as an empty string or null).
func TestUpdatePayload_MonPwdOnlyWhenSet(t *testing.T) {
	base := func() *UPSConfigModel {
		return &UPSConfigModel{
			Mode:           types.StringValue("MASTER"),
			ShutdownCmd:    types.StringNull(),
			NoCommWarnTime: types.Int64Null(),
		}
	}

	m := base()
	m.MonPwd = types.StringValue("hunter2")
	p := m.updatePayload()
	if v, ok := p["monpwd"]; !ok || v != "hunter2" {
		t.Errorf("expected 'monpwd' = 'hunter2', got %v (present=%v)", v, ok)
	}

	m2 := base()
	m2.MonPwd = types.StringNull()
	p2 := m2.updatePayload()
	if _, ok := p2["monpwd"]; ok {
		t.Error("expected 'monpwd' to be omitted when null")
	}

	m3 := base()
	m3.MonPwd = types.StringUnknown()
	p3 := m3.updatePayload()
	if _, ok := p3["monpwd"]; ok {
		t.Error("expected 'monpwd' to be omitted when unknown")
	}
}

// TestUpdatePayload_CompleteIdentifierNeverSent verifies that
// complete_identifier is never included in the update payload, regardless of
// its model value: it is server-derived and read-only.
func TestUpdatePayload_CompleteIdentifierNeverSent(t *testing.T) {
	m := &UPSConfigModel{
		Mode:               types.StringValue("MASTER"),
		ShutdownCmd:        types.StringNull(),
		NoCommWarnTime:     types.Int64Null(),
		CompleteIdentifier: types.StringValue("ups@localhost"),
	}

	p := m.updatePayload()
	if _, ok := p["complete_identifier"]; ok {
		t.Error("expected 'complete_identifier' to never be present in the payload")
	}
}

// TestBasePayloadFromConfig_IncludesAllWritableFields verifies that
// basePayloadFromConfig carries every writable, non-secret field from a live
// upsConfigAPI response into the base payload, including nil ShutdownCmd/
// NoCommWarnTime mapping to nil (not omitted) entries, and that
// complete_identifier/monpwd are never included.
func TestBasePayloadFromConfig_IncludesAllWritableFields(t *testing.T) {
	api := &upsConfigAPI{
		ID:                 1,
		Identifier:         "ups",
		Mode:               "MASTER",
		RemoteHost:         "",
		RemotePort:         3493,
		Driver:             "usbhid-ups",
		Port:               "auto",
		Options:            "",
		OptionsUPSD:        "",
		Description:        "old-description",
		Shutdown:           "LOWBATT",
		ShutdownTimer:      30,
		ShutdownCmd:        nil,
		MonUser:            "monuser",
		ExtraUsers:         "",
		RMonitor:           false,
		PowerDown:          true,
		HostSync:           15,
		NoCommWarnTime:     nil,
		CompleteIdentifier: "ups@localhost",
	}

	p := basePayloadFromConfig(api)

	want := map[string]any{
		"identifier":    "ups",
		"mode":          "MASTER",
		"remotehost":    "",
		"remoteport":    int64(3493),
		"driver":        "usbhid-ups",
		"port":          "auto",
		"options":       "",
		"optionsupsd":   "",
		"description":   "old-description",
		"shutdown":      "LOWBATT",
		"shutdowntimer": int64(30),
		"monuser":       "monuser",
		"extrausers":    "",
		"rmonitor":      false,
		"powerdown":     true,
		"hostsync":      int64(15),
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if v, ok := p["shutdowncmd"]; !ok || v != nil {
		t.Errorf(`payload["shutdowncmd"] = %v (present=%v), want nil (present)`, v, ok)
	}
	if v, ok := p["nocommwarntime"]; !ok || v != nil {
		t.Errorf(`payload["nocommwarntime"] = %v (present=%v), want nil (present)`, v, ok)
	}
	if _, ok := p["complete_identifier"]; ok {
		t.Error(`payload["complete_identifier"] should never be present (server-derived)`)
	}
	if _, ok := p["monpwd"]; ok {
		t.Error(`payload["monpwd"] should never be present (secret, absent from upsConfigAPI)`)
	}
	if len(p) != len(want)+2 {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want)+2)
	}

	cmd := "/sbin/shutdown -h now"
	warn := int64(300)
	api.ShutdownCmd = &cmd
	api.NoCommWarnTime = &warn
	p2 := basePayloadFromConfig(api)
	if v, ok := p2["shutdowncmd"]; !ok || v != cmd {
		t.Errorf(`payload["shutdowncmd"] = %v (present=%v), want %q`, v, ok, cmd)
	}
	if v, ok := p2["nocommwarntime"]; !ok || v != warn {
		t.Errorf(`payload["nocommwarntime"] = %v (present=%v), want %d`, v, ok, warn)
	}
}

// TestMergedPayload_PlanOverlaysLive verifies that mergedPayload starts from
// the live config's fields and that any field the plan knows about
// overrides the live value, while fields the plan doesn't know about keep
// their live value — this is the fix for "ups_update.port: This field is
// required" / "ups_update.driver: This field is required" when a config
// sets only description.
func TestMergedPayload_PlanOverlaysLive(t *testing.T) {
	live := &upsConfigAPI{
		ID:          1,
		Identifier:  "ups",
		Mode:        "MASTER",
		Driver:      "usbhid-ups",
		Port:        "auto",
		Description: "old-description",
		Shutdown:    "LOWBATT",
		HostSync:    15,
	}

	// Plan only sets description; everything else is null/unknown, as
	// Optional+Computed fields the user's config didn't set.
	m := &UPSConfigModel{
		Identifier:     types.StringNull(),
		Mode:           types.StringNull(),
		RemoteHost:     types.StringNull(),
		RemotePort:     types.Int64Null(),
		Driver:         types.StringNull(),
		Port:           types.StringNull(),
		Options:        types.StringNull(),
		OptionsUPSD:    types.StringNull(),
		Description:    types.StringValue("new-description"),
		Shutdown:       types.StringNull(),
		ShutdownTimer:  types.Int64Null(),
		MonUser:        types.StringNull(),
		MonPwd:         types.StringNull(),
		ExtraUsers:     types.StringNull(),
		RMonitor:       types.BoolNull(),
		PowerDown:      types.BoolNull(),
		HostSync:       types.Int64Null(),
		ShutdownCmd:    types.StringNull(),
		NoCommWarnTime: types.Int64Null(),
	}

	p := mergedPayload(live, m)

	// port/driver are required by ups.update but not set in the plan: they
	// must still be present, carried from the live config.
	if v, ok := p["port"]; !ok || v != "auto" {
		t.Errorf(`payload["port"] = %v (present=%v), want "auto" from live config`, v, ok)
	}
	if v, ok := p["driver"]; !ok || v != "usbhid-ups" {
		t.Errorf(`payload["driver"] = %v (present=%v), want "usbhid-ups" from live config`, v, ok)
	}
	// description is set in the plan: it must override the live value.
	if v, ok := p["description"]; !ok || v != "new-description" {
		t.Errorf(`payload["description"] = %v (present=%v), want "new-description" (plan wins over live)`, v, ok)
	}
	// Another live field not touched by the plan is still carried through.
	if v, ok := p["hostsync"]; !ok || v != int64(15) {
		t.Errorf(`payload["hostsync"] = %v (present=%v), want live value`, v, ok)
	}
}

// TestMergedPayload_SecretAbsentUnlessSet verifies that mergedPayload never
// includes "monpwd" unless the plan explicitly sets it: the base built from
// the live config has no secret fields, and updatePayload only contributes
// monpwd when it's known and non-null.
func TestMergedPayload_SecretAbsentUnlessSet(t *testing.T) {
	live := &upsConfigAPI{ID: 1, Driver: "usbhid-ups", Port: "auto"}

	m := &UPSConfigModel{
		Driver:         types.StringNull(),
		Port:           types.StringNull(),
		ShutdownCmd:    types.StringNull(),
		NoCommWarnTime: types.Int64Null(),
		MonPwd:         types.StringNull(),
	}
	p := mergedPayload(live, m)
	if _, ok := p["monpwd"]; ok {
		t.Error(`payload["monpwd"] should be absent when the plan doesn't set it`)
	}

	m2 := &UPSConfigModel{
		Driver:         types.StringNull(),
		Port:           types.StringNull(),
		ShutdownCmd:    types.StringNull(),
		NoCommWarnTime: types.Int64Null(),
		MonPwd:         types.StringValue("hunter2"),
	}
	p2 := mergedPayload(live, m2)
	if v, ok := p2["monpwd"]; !ok || v != "hunter2" {
		t.Errorf(`payload["monpwd"] = %v (present=%v), want "hunter2" when the plan sets it`, v, ok)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// UPSConfigResource.Delete calls only this pure function.
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
	if detail != "UPS configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestUPSConfigDataSourceModel_NoMonPwdField verifies that
// UPSConfigDataSourceModel has no monpwd field: the API never returns a
// usable value for it, so a datasource attribute for it would always read
// as empty/unknown.
func TestUPSConfigDataSourceModel_NoMonPwdField(t *testing.T) {
	modelType := reflect.TypeOf(UPSConfigDataSourceModel{})
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "monpwd" {
			t.Errorf("UPSConfigDataSourceModel must not have a %q field (write-only secret)", tag)
		}
	}
}

// TestUPSConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag
// on UPSConfigDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestUPSConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &UPSConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(UPSConfigDataSourceModel{})
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
		t.Fatalf("UPSConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
