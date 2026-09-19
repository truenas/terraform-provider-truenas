// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ssh_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSSHConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestSSHConfigSchema_IDIsComputed(t *testing.T) {
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

// TestSSHConfigSchema_AllFieldsOptionalComputed verifies that every non-id
// field is Optional+Computed with a plan modifier, matching the "all fields
// Optional+Computed + UseStateForUnknown" contract in the task brief.
func TestSSHConfigSchema_AllFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"options", "sftp_log_facility", "sftp_log_level"} {
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

	for _, name := range []string{"compression", "kerberosauth", "passwordauth", "tcpfwd"} {
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

	tcpportAttr, ok := s.Attributes["tcpport"]
	if !ok {
		t.Fatal("schema missing 'tcpport' attribute")
	}
	tcpportInt, ok := tcpportAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'tcpport' attribute is %T, want schema.Int64Attribute", tcpportAttr)
	}
	if !tcpportInt.IsOptional() || !tcpportInt.IsComputed() {
		t.Error("'tcpport' should be Optional+Computed")
	}
	if len(tcpportInt.PlanModifiers) == 0 {
		t.Error("'tcpport' should have plan modifiers (UseStateForUnknown)")
	}

	for _, name := range []string{"bindiface", "password_login_groups", "weak_ciphers"} {
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
	api := &sshConfigAPI{
		ID:                  1,
		BindIface:           nil,
		Compression:         false,
		KerberosAuth:        false,
		Options:             "",
		PasswordLoginGroups: nil,
		PasswordAuth:        true,
		SFTPLogFacility:     "",
		SFTPLogLevel:        "",
		TCPFwd:              false,
		TCPPort:             22,
		WeakCiphers:         nil,
	}

	m := &SSHConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for name, l := range map[string]types.List{
		"bindiface":             m.BindIface,
		"password_login_groups": m.PasswordLoginGroups,
		"weak_ciphers":          m.WeakCiphers,
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

	if m.ID.ValueString() != sshConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), sshConfigResourceID)
	}
}

// TestResponseToModel_ListsSet verifies that non-nil list fields from the
// API are carried through as-is.
func TestResponseToModel_ListsSet(t *testing.T) {
	api := &sshConfigAPI{
		ID:                  1,
		BindIface:           []string{"eth0", "eth1"},
		PasswordLoginGroups: []string{"wheel"},
		WeakCiphers:         []string{"AES128-CBC"},
		TCPPort:             22,
	}

	m := &SSHConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var bindiface []string
	if d := m.BindIface.ElementsAs(context.Background(), &bindiface, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(bindiface) != 2 || bindiface[0] != "eth0" || bindiface[1] != "eth1" {
		t.Errorf("BindIface = %v, want [eth0 eth1]", bindiface)
	}

	var groups []string
	if d := m.PasswordLoginGroups.ElementsAs(context.Background(), &groups, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(groups) != 1 || groups[0] != "wheel" {
		t.Errorf("PasswordLoginGroups = %v, want [wheel]", groups)
	}

	var weak []string
	if d := m.WeakCiphers.ElementsAs(context.Background(), &weak, false); d.HasError() {
		t.Fatalf("unexpected error: %v", d)
	}
	if len(weak) != 1 || weak[0] != "AES128-CBC" {
		t.Errorf("WeakCiphers = %v, want [AES128-CBC]", weak)
	}
}

// TestResponseToDataSourceModel_ListsNilBecomeEmptyList mirrors the same
// nil-to-empty-list handling for the datasource model.
func TestResponseToDataSourceModel_ListsNilBecomeEmptyList(t *testing.T) {
	api := &sshConfigAPI{
		ID:                  1,
		BindIface:           nil,
		PasswordLoginGroups: nil,
		WeakCiphers:         nil,
		TCPPort:             22,
	}

	m := &SSHConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.BindIface.IsNull() || m.PasswordLoginGroups.IsNull() || m.WeakCiphers.IsNull() {
		t.Error("list fields should not be null when API returns nil, want empty lists")
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any
// field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &SSHConfigModel{
		BindIface:           types.ListNull(types.StringType),
		Compression:         types.BoolUnknown(),
		KerberosAuth:        types.BoolNull(),
		Options:             types.StringNull(),
		PasswordLoginGroups: types.ListUnknown(types.StringType),
		PasswordAuth:        types.BoolValue(true),
		SFTPLogFacility:     types.StringNull(),
		SFTPLogLevel:        types.StringUnknown(),
		TCPFwd:              types.BoolNull(),
		TCPPort:             types.Int64Value(22),
		WeakCiphers:         types.ListNull(types.StringType),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, k := range []string{
		"bindiface", "compression", "kerberosauth", "options",
		"password_login_groups", "sftp_log_facility", "sftp_log_level",
		"tcpfwd", "weak_ciphers",
	} {
		if _, ok := p[k]; ok {
			t.Errorf("expected %q to be omitted (null/unknown)", k)
		}
	}
	if v, ok := p["passwordauth"]; !ok || v != true {
		t.Errorf("expected 'passwordauth' = true, got %v (present=%v)", v, ok)
	}
	if v, ok := p["tcpport"]; !ok || v != int64(22) {
		t.Errorf("expected 'tcpport' = 22, got %v (present=%v)", v, ok)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	bindiface, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"eth0"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}
	groups, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"wheel"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}
	weak, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"AES128-CBC"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}

	m := &SSHConfigModel{
		BindIface:           bindiface,
		Compression:         types.BoolValue(true),
		KerberosAuth:        types.BoolValue(false),
		Options:             types.StringValue("MaxAuthTries 3"),
		PasswordLoginGroups: groups,
		PasswordAuth:        types.BoolValue(true),
		SFTPLogFacility:     types.StringValue("AUTH"),
		SFTPLogLevel:        types.StringValue("INFO"),
		TCPFwd:              types.BoolValue(true),
		TCPPort:             types.Int64Value(2222),
		WeakCiphers:         weak,
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"compression":       true,
		"kerberosauth":      false,
		"options":           "MaxAuthTries 3",
		"passwordauth":      true,
		"sftp_log_facility": "AUTH",
		"sftp_log_level":    "INFO",
		"tcpfwd":            true,
		"tcpport":           int64(2222),
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}

	for name, wantVal := range map[string][]string{
		"bindiface":             {"eth0"},
		"password_login_groups": {"wheel"},
		"weak_ciphers":          {"AES128-CBC"},
	} {
		got, ok := p[name].([]string)
		if !ok {
			t.Fatalf("payload[%s] is %T, want []string", name, p[name])
		}
		if !reflect.DeepEqual(got, wantVal) {
			t.Errorf("payload[%s] = %v, want %v", name, got, wantVal)
		}
	}

	if len(p) != len(want)+3 {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want)+3)
	}
}

// TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice verifies that a known but
// empty list is sent as an empty slice, not nil/null, for all three list
// fields.
func TestUpdatePayload_ListsKnownEmptyBecomeEmptySlice(t *testing.T) {
	emptyList, diags := types.ListValueFrom(context.Background(), types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("unexpected error building empty list: %v", diags)
	}

	m := &SSHConfigModel{
		BindIface:           emptyList,
		Compression:         types.BoolNull(),
		KerberosAuth:        types.BoolNull(),
		Options:             types.StringNull(),
		PasswordLoginGroups: emptyList,
		PasswordAuth:        types.BoolNull(),
		SFTPLogFacility:     types.StringNull(),
		SFTPLogLevel:        types.StringNull(),
		TCPFwd:              types.BoolNull(),
		TCPPort:             types.Int64Null(),
		WeakCiphers:         emptyList,
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, name := range []string{"bindiface", "password_login_groups", "weak_ciphers"} {
		got, ok := p[name].([]string)
		if !ok {
			t.Fatalf("payload[%s] is %T, want []string", name, p[name])
		}
		if len(got) != 0 {
			t.Errorf("payload[%s] = %v, want empty slice", name, got)
		}
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// SSHConfigResource.Delete calls only this pure function.
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
	if detail != "SSH configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestSSHConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// SSHConfigDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa. terraform-plugin-framework requires an exact
// field/attribute match: any mismatch causes every datasource Read to fail
// with "Struct defines fields not found in object: ...".
func TestSSHConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &SSHConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(SSHConfigDataSourceModel{})
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
		t.Fatalf("SSHConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
