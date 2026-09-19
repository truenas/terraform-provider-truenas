// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// baseModel returns a SystemAdvancedModel with every field null/unknown,
// convenient as a starting point for tests that only care about a handful
// of fields.
func baseModel() *SystemAdvancedModel {
	return &SystemAdvancedModel{
		Advancedmode:       types.BoolNull(),
		Anonstats:          types.BoolNull(),
		Autotune:           types.BoolNull(),
		BootScrub:          types.Int64Null(),
		Consolemenu:        types.BoolNull(),
		Consolemsg:         types.BoolNull(),
		Debugkernel:        types.BoolNull(),
		FqdnSyslog:         types.BoolNull(),
		KdumpEnabled:       types.BoolNull(),
		KernelExtraOptions: types.StringNull(),
		LoginBanner:        types.StringNull(),
		Motd:               types.StringNull(),
		Nvidia:             types.BoolNull(),
		Overprovision:      types.Int64Null(),
		Powerdaemon:        types.BoolNull(),
		SedPasswd:          types.StringNull(),
		SedUser:            types.StringNull(),
		Serialconsole:      types.BoolNull(),
		Serialport:         types.StringNull(),
		Serialspeed:        types.StringNull(),
		SyslogAudit:        types.BoolNull(),
		Sysloglevel:        types.StringNull(),
		Syslogservers:      types.ListNull(types.StringType),
		Traceback:          types.BoolNull(),
		Uploadcrash:        types.BoolNull(),
	}
}

// TestSystemAdvancedSchema_IDIsComputed verifies that "id" is a
// Computed-only StringAttribute with UseStateForUnknown, since it's a fixed
// singleton value never supplied by the user.
func TestSystemAdvancedSchema_IDIsComputed(t *testing.T) {
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

// TestSystemAdvancedSchema_WritableFieldsOptionalComputed spot-checks that
// writable fields across each type are Optional+Computed with a
// UseStateForUnknown plan modifier.
func TestSystemAdvancedSchema_WritableFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	boolFields := []string{
		"advancedmode", "anonstats", "autotune", "consolemenu", "consolemsg",
		"debugkernel", "fqdn_syslog", "kdump_enabled", "nvidia", "powerdaemon",
		"serialconsole", "syslog_audit", "traceback", "uploadcrash",
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

	stringFields := []string{
		"kernel_extra_options", "login_banner", "motd", "sed_user",
		"serialport", "serialspeed", "sysloglevel",
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

	int64Fields := []string{"boot_scrub", "overprovision"}
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

	listAttr, ok := s.Attributes["syslogservers"].(schema.ListAttribute)
	if !ok {
		t.Fatal("schema missing 'syslogservers' ListAttribute")
	}
	if !listAttr.IsOptional() || !listAttr.IsComputed() {
		t.Error("'syslogservers' should be Optional+Computed")
	}
	if len(listAttr.PlanModifiers) == 0 {
		t.Error("'syslogservers' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestSystemAdvancedSchema_SedPasswdIsWriteOnlySecret verifies that
// sed_passwd is Optional+Sensitive ONLY: never Computed (there is no server
// value to compute from) and carrying no plan modifiers.
func TestSystemAdvancedSchema_SedPasswdIsWriteOnlySecret(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["sed_passwd"]
	if !ok {
		t.Fatal("schema missing 'sed_passwd' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'sed_passwd' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() {
		t.Error("'sed_passwd' should be Optional")
	}
	if strAttr.IsComputed() {
		t.Error("'sed_passwd' should NOT be Computed (write-only secret)")
	}
	if !strAttr.IsSensitive() {
		t.Error("'sed_passwd' should be Sensitive")
	}
	if !strAttr.IsWriteOnly() {
		t.Error("'sed_passwd' should be WriteOnly")
	}
}

// TestSystemAdvancedSchema_ComputedOnlyPair verifies that anonstats_token
// and isolated_gpu_pci_ids are Computed-only (not Optional) and carry NO
// plan modifiers.
func TestSystemAdvancedSchema_ComputedOnlyPair(t *testing.T) {
	s := resourceSchema()

	tokenAttr, ok := s.Attributes["anonstats_token"].(schema.StringAttribute)
	if !ok {
		t.Fatal("schema missing 'anonstats_token' StringAttribute")
	}
	if !tokenAttr.IsComputed() || tokenAttr.IsOptional() {
		t.Error("'anonstats_token' should be Computed-only")
	}
	if len(tokenAttr.PlanModifiers) != 0 {
		t.Errorf("'anonstats_token' should have NO plan modifiers, got %d", len(tokenAttr.PlanModifiers))
	}

	gpuAttr, ok := s.Attributes["isolated_gpu_pci_ids"].(schema.ListAttribute)
	if !ok {
		t.Fatal("schema missing 'isolated_gpu_pci_ids' ListAttribute")
	}
	if !gpuAttr.IsComputed() || gpuAttr.IsOptional() {
		t.Error("'isolated_gpu_pci_ids' should be Computed-only")
	}
	if len(gpuAttr.PlanModifiers) != 0 {
		t.Errorf("'isolated_gpu_pci_ids' should have NO plan modifiers, got %d", len(gpuAttr.PlanModifiers))
	}
}

// TestResponseToModel_OverprovisionMapsToNull verifies that a nil
// overprovision on the wire maps to Int64Null (not a zero value), and that a
// non-nil value maps through unchanged.
func TestResponseToModel_OverprovisionMapsToNull(t *testing.T) {
	api := &systemAdvancedAPI{SedUser: "USER", Serialspeed: "9600", Sysloglevel: "F_INFO", Overprovision: nil}

	m := &SystemAdvancedModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.Overprovision.IsNull() {
		t.Errorf("Overprovision = %v, want null", m.Overprovision)
	}

	v := int64(10)
	api.Overprovision = &v
	m2 := &SystemAdvancedModel{}
	diags = responseToModel(context.Background(), api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m2.Overprovision.IsNull() || m2.Overprovision.ValueInt64() != 10 {
		t.Errorf("Overprovision = %v, want 10", m2.Overprovision)
	}
}

// TestResponseToModel_SedPasswdNeverSet verifies that responseToModel never
// touches SedPasswd, even when the raw JSON response contains a
// "sed_passwd" key: systemAdvancedAPI has no field for it at all, so it is
// structurally impossible to decode into the model. A pre-existing model
// value (e.g. from the plan) is left completely untouched by
// responseToModel.
func TestResponseToModel_SedPasswdNeverSet(t *testing.T) {
	raw := []byte(`{"id":1,"sed_user":"USER","sed_passwd":"supersecret","serialspeed":"9600","sysloglevel":"F_INFO","overprovision":null}`)

	var api systemAdvancedAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	m := &SystemAdvancedModel{SedPasswd: types.StringValue("plan-supplied-secret")}
	diags := responseToModel(context.Background(), &api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.SedPasswd.ValueString() != "plan-supplied-secret" {
		t.Errorf("SedPasswd = %q, want untouched value %q", m.SedPasswd.ValueString(), "plan-supplied-secret")
	}
}

// TestResponseToDataSourceModel_NoSedPasswdField verifies (via reflection)
// that SystemAdvancedDataSourceModel has no sed_passwd field at all.
func TestResponseToDataSourceModel_NoSedPasswdField(t *testing.T) {
	modelType := reflect.TypeOf(SystemAdvancedDataSourceModel{})
	for i := 0; i < modelType.NumField(); i++ {
		if tag := modelType.Field(i).Tag.Get("tfsdk"); tag == "sed_passwd" {
			t.Fatal("SystemAdvancedDataSourceModel must not have a sed_passwd field")
		}
	}
}

// TestSystemAdvancedAPI_NoSedPasswdField verifies (via reflection) that
// systemAdvancedAPI has no sed_passwd field at all, so a response
// containing a "sed_passwd" key is structurally undecodable into it.
func TestSystemAdvancedAPI_NoSedPasswdField(t *testing.T) {
	apiType := reflect.TypeOf(systemAdvancedAPI{})
	for i := 0; i < apiType.NumField(); i++ {
		tag := apiType.Field(i).Tag.Get("json")
		if tag == "sed_passwd" {
			t.Fatal("systemAdvancedAPI must not have a sed_passwd field")
		}
	}
}

// TestResponseToModel_ListsNilMapToEmpty verifies that nil list fields from
// the API (syslogservers, isolated_gpu_pci_ids) map to empty (non-null)
// Terraform lists.
func TestResponseToModel_ListsNilMapToEmpty(t *testing.T) {
	api := &systemAdvancedAPI{
		SedUser:           "USER",
		Serialspeed:       "9600",
		Sysloglevel:       "F_INFO",
		Syslogservers:     nil,
		IsolatedGpuPciIds: nil,
	}

	m := &SystemAdvancedModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for name, l := range map[string]types.List{
		"syslogservers":        m.Syslogservers,
		"isolated_gpu_pci_ids": m.IsolatedGpuPciIds,
	} {
		if l.IsNull() {
			t.Errorf("%s should be an empty list, not null", name)
		}
		if len(l.Elements()) != 0 {
			t.Errorf("%s should have 0 elements, got %d", name, len(l.Elements()))
		}
	}
}

// TestResponseToModel_MapsPlainFields verifies that non-nullable,
// non-pointer fields are copied through unchanged, including the two
// Computed-only fields anonstats_token and isolated_gpu_pci_ids.
func TestResponseToModel_MapsPlainFields(t *testing.T) {
	api := &systemAdvancedAPI{
		ID:                 1,
		Advancedmode:       true,
		Anonstats:          true,
		AnonstatsToken:     "tok123",
		Autotune:           true,
		BootScrub:          7,
		Consolemenu:        true,
		Consolemsg:         true,
		Debugkernel:        true,
		FqdnSyslog:         true,
		IsolatedGpuPciIds:  []string{"0000:01:00.0"},
		KdumpEnabled:       true,
		KernelExtraOptions: "opt1",
		LoginBanner:        "banner",
		Motd:               "Welcome",
		Nvidia:             true,
		Powerdaemon:        true,
		SedUser:            "MASTER",
		Serialconsole:      true,
		Serialport:         "ttyS0",
		Serialspeed:        "115200",
		SyslogAudit:        true,
		Sysloglevel:        "F_INFO",
		Syslogservers:      []string{"syslog.example.com"},
		Traceback:          true,
		Uploadcrash:        true,
	}

	m := &SystemAdvancedModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != systemAdvancedResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), systemAdvancedResourceID)
	}
	if m.AnonstatsToken.ValueString() != api.AnonstatsToken {
		t.Errorf("AnonstatsToken = %q, want %q", m.AnonstatsToken.ValueString(), api.AnonstatsToken)
	}
	if m.BootScrub.ValueInt64() != api.BootScrub {
		t.Errorf("BootScrub = %d, want %d", m.BootScrub.ValueInt64(), api.BootScrub)
	}
	if m.SedUser.ValueString() != api.SedUser {
		t.Errorf("SedUser = %q, want %q", m.SedUser.ValueString(), api.SedUser)
	}
	if m.Serialspeed.ValueString() != api.Serialspeed {
		t.Errorf("Serialspeed = %q, want %q", m.Serialspeed.ValueString(), api.Serialspeed)
	}
	if m.Sysloglevel.ValueString() != api.Sysloglevel {
		t.Errorf("Sysloglevel = %q, want %q", m.Sysloglevel.ValueString(), api.Sysloglevel)
	}
	if m.Motd.ValueString() != api.Motd {
		t.Errorf("Motd = %q, want %q", m.Motd.ValueString(), api.Motd)
	}
	gpuIds := m.IsolatedGpuPciIds.Elements()
	if len(gpuIds) != 1 {
		t.Fatalf("IsolatedGpuPciIds has %d elements, want 1", len(gpuIds))
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// any field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := baseModel()
	m.Motd = types.StringValue("hello")

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{
		"advancedmode", "anonstats", "autotune", "boot_scrub", "consolemenu",
		"consolemsg", "debugkernel", "fqdn_syslog", "kdump_enabled",
		"kernel_extra_options", "login_banner", "nvidia", "overprovision",
		"powerdaemon", "sed_passwd", "sed_user", "serialconsole",
		"serialport", "serialspeed", "syslog_audit", "sysloglevel",
		"syslogservers", "traceback", "uploadcrash",
	} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to be omitted (null/unknown)", key)
		}
	}
	if v, ok := p["motd"]; !ok || v != "hello" {
		t.Errorf("expected 'motd' = hello, got %v (present=%v)", v, ok)
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known, including a non-zero
// overprovision and a non-empty syslogservers list.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	ctx := context.Background()
	syslogservers, _ := types.ListValueFrom(ctx, types.StringType, []string{"syslog.example.com"})

	m := &SystemAdvancedModel{
		Advancedmode:       types.BoolValue(true),
		Anonstats:          types.BoolValue(true),
		Autotune:           types.BoolValue(true),
		BootScrub:          types.Int64Value(7),
		Consolemenu:        types.BoolValue(true),
		Consolemsg:         types.BoolValue(true),
		Debugkernel:        types.BoolValue(true),
		FqdnSyslog:         types.BoolValue(true),
		KdumpEnabled:       types.BoolValue(true),
		KernelExtraOptions: types.StringValue("opt1"),
		LoginBanner:        types.StringValue("banner"),
		Motd:               types.StringValue("Welcome"),
		Nvidia:             types.BoolValue(true),
		Overprovision:      types.Int64Value(10),
		Powerdaemon:        types.BoolValue(true),
		SedPasswd:          types.StringValue("s3kr1t"),
		SedUser:            types.StringValue("MASTER"),
		Serialconsole:      types.BoolValue(true),
		Serialport:         types.StringValue("ttyS0"),
		Serialspeed:        types.StringValue("115200"),
		SyslogAudit:        types.BoolValue(true),
		Sysloglevel:        types.StringValue("F_WARNING"),
		Syslogservers:      syslogservers,
		Traceback:          types.BoolValue(true),
		Uploadcrash:        types.BoolValue(true),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if v, ok := p["overprovision"]; !ok || v != int64(10) {
		t.Errorf("payload[overprovision] = %v (present=%v), want 10", v, ok)
	}
	if v, ok := p["sed_passwd"]; !ok || v != "s3kr1t" {
		t.Errorf("payload[sed_passwd] = %v (present=%v), want s3kr1t", v, ok)
	}
	if v, ok := p["syslogservers"]; !ok {
		t.Error("expected 'syslogservers' present")
	} else if s, ok := v.([]string); !ok || len(s) != 1 || s[0] != "syslog.example.com" {
		t.Errorf("payload[syslogservers] = %v, want [syslog.example.com]", v)
	}

	want := map[string]any{
		"advancedmode":         true,
		"anonstats":            true,
		"autotune":             true,
		"boot_scrub":           int64(7),
		"consolemenu":          true,
		"consolemsg":           true,
		"debugkernel":          true,
		"fqdn_syslog":          true,
		"kdump_enabled":        true,
		"kernel_extra_options": "opt1",
		"login_banner":         "banner",
		"motd":                 "Welcome",
		"powerdaemon":          true,
		"sed_user":             "MASTER",
		"serialconsole":        true,
		"serialport":           "ttyS0",
		"serialspeed":          "115200",
		"syslog_audit":         true,
		"sysloglevel":          "F_WARNING",
		"traceback":            true,
		"uploadcrash":          true,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}

	// nvidia is handled entirely by resource.go's applyNvidiaSupport
	// (Config-driven, version-gated below TrueNAS 26.0), never by
	// updatePayload — see updatePayload's doc comment.
	if _, ok := p["nvidia"]; ok {
		t.Error("'nvidia' should never be present in updatePayload's output")
	}
}

// TestNvidiaSupported verifies the pure version-comparison gate for
// system.advanced.update's "nvidia" field, including the exact boundary and
// malformed-input edge cases.
func TestNvidiaSupported(t *testing.T) {
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
		if got := nvidiaSupported(c.version); got != c.want {
			t.Errorf("nvidiaSupported(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

// TestApplyNvidiaSupport_ProbeErrorStripsField verifies the fail-closed
// direction of applyNvidiaSupport: when the version probe itself errors
// (ServerVersion returns err != nil), the gate must strip "nvidia" from the
// payload rather than keep it, since keeping it on an unprobed (majority
// pre-26.0) target would send a field system.advanced.update rejects with
// its generic "Extra inputs are not permitted" error. A client that was
// constructed but never Connect()-ed has a nil underlying connection, so
// ServerVersion's CallRead fails immediately with "not connected"; the
// context passed in is pre-cancelled so CallRead's retry loop exits on
// ctx.Done() instead of sleeping through real backoff delays or attempting
// a real network dial.
func TestApplyNvidiaSupport_ProbeErrorStripsField(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &SystemAdvancedResource{client: client.New("ws://127.0.0.1:0", nil)}
	payload := map[string]any{}
	var applyDiags diag.Diagnostics
	r.applyNvidiaSupport(ctx, payload, types.BoolValue(true), &applyDiags)

	if _, ok := payload["nvidia"]; ok {
		t.Error("payload[\"nvidia\"] present after a version-probe error; gate should fail closed and strip it")
	}
	if !applyDiags.HasError() {
		t.Error("expected an apply-time error when the version probe fails, got none")
	}
}

// TestUpdatePayload_SedPasswdOnlyWhenSet verifies that sed_passwd is omitted
// from the payload whenever its model value is null or unknown, and is
// included only when a value has actually been set.
func TestUpdatePayload_SedPasswdOnlyWhenSet(t *testing.T) {
	nullModel := baseModel()
	p, diags := nullModel.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["sed_passwd"]; ok {
		t.Error("expected 'sed_passwd' to be omitted when null")
	}

	unknownModel := baseModel()
	unknownModel.SedPasswd = types.StringUnknown()
	p, diags = unknownModel.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["sed_passwd"]; ok {
		t.Error("expected 'sed_passwd' to be omitted when unknown")
	}

	setModel := baseModel()
	setModel.SedPasswd = types.StringValue("newpassword")
	p, diags = setModel.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["sed_passwd"]; !ok || v != "newpassword" {
		t.Errorf("payload[sed_passwd] = %v (present=%v), want newpassword", v, ok)
	}
}

// TestUpdatePayload_OverprovisionThreeWay verifies the three-way behavior
// for overprovision: null/unknown omits the key entirely, an explicit 0
// sends JSON nil (clearing overprovision), and any other value sends that
// value.
func TestUpdatePayload_OverprovisionThreeWay(t *testing.T) {
	withOverprovision := func(v types.Int64) *SystemAdvancedModel {
		m := baseModel()
		m.Overprovision = v
		return m
	}

	// null -> omitted
	p, diags := withOverprovision(types.Int64Null()).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["overprovision"]; ok {
		t.Error("expected 'overprovision' to be omitted when null")
	}

	// unknown -> omitted
	p, diags = withOverprovision(types.Int64Unknown()).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["overprovision"]; ok {
		t.Error("expected 'overprovision' to be omitted when unknown")
	}

	// explicit 0 -> nil
	p, diags = withOverprovision(types.Int64Value(0)).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	v, ok := p["overprovision"]
	if !ok {
		t.Fatal("expected 'overprovision' to be present when explicitly 0")
	}
	if v != nil {
		t.Errorf("overprovision = %v, want nil", v)
	}

	// N -> N
	p, diags = withOverprovision(types.Int64Value(25)).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["overprovision"]; !ok || v != int64(25) {
		t.Errorf("overprovision = %v (present=%v), want 25", v, ok)
	}
}

// TestUpdatePayload_ComputedOnlyPairNeverSent verifies that
// anonstats_token and isolated_gpu_pci_ids never appear in the update
// payload (they have no corresponding updatePayload field at all, so this
// documents the contract by construction).
func TestUpdatePayload_ComputedOnlyPairNeverSent(t *testing.T) {
	m := baseModel()
	m.AnonstatsToken = types.StringValue("tok123")
	m.IsolatedGpuPciIds, _ = types.ListValueFrom(context.Background(), types.StringType, []string{"0000:01:00.0"})

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{"anonstats_token", "isolated_gpu_pci_ids"} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to never be present in the update payload", key)
		}
	}
}

// TestUpdatePayload_SyslogserversNilMapsToEmptyList verifies that an
// explicitly-set-but-empty syslogservers list is sent as an empty slice
// (clearing the value on TrueNAS) rather than a nil ElementsAs result
// panicking or being omitted.
func TestUpdatePayload_SyslogserversNilMapsToEmptyList(t *testing.T) {
	ctx := context.Background()
	empty, _ := types.ListValueFrom(ctx, types.StringType, []string{})

	m := baseModel()
	m.Syslogservers = empty

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	v, ok := p["syslogservers"]
	if !ok {
		t.Fatal("expected 'syslogservers' present")
	}
	s, ok := v.([]string)
	if !ok {
		t.Fatalf("payload[syslogservers] is %T, want []string", v)
	}
	if len(s) != 0 {
		t.Errorf("payload[syslogservers] = %v, want []", s)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// SystemAdvancedResource.Delete calls only this pure function.
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
	if detail != "System advanced configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestSystemAdvancedDataSourceModel_MatchesSchema verifies that every tfsdk
// tag on SystemAdvancedDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestSystemAdvancedDataSourceModel_MatchesSchema(t *testing.T) {
	d := &SystemAdvancedDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(SystemAdvancedDataSourceModel{})
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
		t.Fatalf("SystemAdvancedDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

// TestSystemAdvancedDataSource_NoSedPasswdAttribute verifies that the
// datasource schema itself has no sed_passwd attribute.
func TestSystemAdvancedDataSource_NoSedPasswdAttribute(t *testing.T) {
	d := &SystemAdvancedDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	if _, ok := resp.Schema.Attributes["sed_passwd"]; ok {
		t.Error("datasource schema must not have a 'sed_passwd' attribute")
	}
}
