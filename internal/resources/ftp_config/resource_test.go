// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestFTPConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestFTPConfigSchema_IDIsComputed(t *testing.T) {
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

// TestFTPConfigSchema_AllFieldsOptionalComputed verifies that every non-id
// field is Optional+Computed with a plan modifier, matching the "all fields
// Optional+Computed + UseStateForUnknown" contract in the task brief. This
// includes the two nullable fields (ssltls_certificate, anonpath), which are
// modeled the same way as any other Optional+Computed field at the schema
// level; their nullability is handled in updatePayload/responseToModel.
func TestFTPConfigSchema_AllFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	int64Fields := []string{
		"port", "clients", "ipconnections", "loginattempt", "timeout",
		"timeout_notransfer", "localuserbw", "localuserdlbw", "anonuserbw",
		"anonuserdlbw", "passiveportsmin", "passiveportsmax", "ssltls_certificate",
	}
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

	boolFields := []string{
		"defaultroot", "onlyanonymous", "onlylocal", "ident", "fxp", "resume",
		"reversedns", "tls",
		"tls_opt_allow_client_renegotiations", "tls_opt_allow_dot_login",
		"tls_opt_allow_per_user", "tls_opt_common_name_required",
		"tls_opt_dns_name_required", "tls_opt_enable_diags",
		"tls_opt_export_cert_data", "tls_opt_ip_address_required",
		"tls_opt_no_empty_fragments", "tls_opt_no_session_reuse_required",
		"tls_opt_stdenvvars",
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
		"masqaddress", "banner", "options", "dirmask", "filemask", "tls_policy", "anonpath",
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
}

// baseAPI returns a fully-populated ftpConfigAPI matching the live config
// sanity sample in the task brief, useful as a starting point for tests that
// only care about a subset of fields.
func baseAPI() *ftpConfigAPI {
	return &ftpConfigAPI{
		ID:                1,
		Port:              21,
		Clients:           5,
		IPConnections:     2,
		LoginAttempt:      1,
		Timeout:           600,
		TimeoutNoTransfer: 300,
		DefaultRoot:       true,
		DirMask:           "022",
		FileMask:          "077",
		AnonPath:          nil,
		SSLTLSCertificate: nil,
		TLSPolicy:         "on",
	}
}

// TestResponseToModel_NullableFieldsNilBecomeZero verifies that nil
// ssltls_certificate/anonpath from the API map to the zero value (0 / "")
// in the model, matching the live config sanity sample where both are null.
func TestResponseToModel_NullableFieldsNilBecomeZero(t *testing.T) {
	api := baseAPI()

	m := &FTPConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.SSLTLSCertificate.ValueInt64() != 0 {
		t.Errorf("SSLTLSCertificate = %v, want 0", m.SSLTLSCertificate.ValueInt64())
	}
	if m.SSLTLSCertificate.IsNull() {
		t.Error("SSLTLSCertificate should not be null (zero value, not null)")
	}
	if m.AnonPath.ValueString() != "" {
		t.Errorf("AnonPath = %q, want \"\"", m.AnonPath.ValueString())
	}
	if m.AnonPath.IsNull() {
		t.Error("AnonPath should not be null (zero value, not null)")
	}

	if m.ID.ValueString() != ftpConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), ftpConfigResourceID)
	}
}

// TestResponseToModel_NullableFieldsSet verifies that non-nil
// ssltls_certificate/anonpath from the API are carried through as-is.
func TestResponseToModel_NullableFieldsSet(t *testing.T) {
	cert := int64(7)
	path := "/mnt/pool/ftp"
	api := baseAPI()
	api.SSLTLSCertificate = &cert
	api.AnonPath = &path

	m := &FTPConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.SSLTLSCertificate.ValueInt64() != 7 {
		t.Errorf("SSLTLSCertificate = %v, want 7", m.SSLTLSCertificate.ValueInt64())
	}
	if m.AnonPath.ValueString() != "/mnt/pool/ftp" {
		t.Errorf("AnonPath = %q, want \"/mnt/pool/ftp\"", m.AnonPath.ValueString())
	}
}

// TestResponseToDataSourceModel_NullableFieldsNilBecomeZero mirrors the same
// nil-to-zero handling for the datasource model.
func TestResponseToDataSourceModel_NullableFieldsNilBecomeZero(t *testing.T) {
	api := baseAPI()

	m := &FTPConfigDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.SSLTLSCertificate.ValueInt64() != 0 {
		t.Errorf("SSLTLSCertificate = %v, want 0", m.SSLTLSCertificate.ValueInt64())
	}
	if m.AnonPath.ValueString() != "" {
		t.Errorf("AnonPath = %q, want \"\"", m.AnonPath.ValueString())
	}
}

// baseModel returns a FTPConfigModel with every field set to a known value,
// used as a starting point for updatePayload tests that only care about a
// subset of fields.
func baseModel() *FTPConfigModel {
	return &FTPConfigModel{
		Port:              types.Int64Value(21),
		Clients:           types.Int64Value(5),
		IPConnections:     types.Int64Value(2),
		LoginAttempt:      types.Int64Value(1),
		Timeout:           types.Int64Value(600),
		TimeoutNoTransfer: types.Int64Value(300),
		LocalUserBW:       types.Int64Value(0),
		LocalUserDLBW:     types.Int64Value(0),
		AnonUserBW:        types.Int64Value(0),
		AnonUserDLBW:      types.Int64Value(0),
		PassivePortsMin:   types.Int64Value(0),
		PassivePortsMax:   types.Int64Value(0),

		DefaultRoot:                     types.BoolValue(true),
		OnlyAnonymous:                   types.BoolValue(false),
		OnlyLocal:                       types.BoolValue(false),
		Ident:                           types.BoolValue(false),
		FXP:                             types.BoolValue(false),
		Resume:                          types.BoolValue(true),
		ReverseDNS:                      types.BoolValue(false),
		TLS:                             types.BoolValue(false),
		TLSOptAllowClientRenegotiations: types.BoolValue(false),
		TLSOptAllowDotLogin:             types.BoolValue(false),
		TLSOptAllowPerUser:              types.BoolValue(false),
		TLSOptCommonNameRequired:        types.BoolValue(false),
		TLSOptDNSNameRequired:           types.BoolValue(false),
		TLSOptEnableDiags:               types.BoolValue(false),
		TLSOptExportCertData:            types.BoolValue(false),
		TLSOptIPAddressRequired:         types.BoolValue(false),
		TLSOptNoEmptyFragments:          types.BoolValue(false),
		TLSOptNoSessionReuseRequired:    types.BoolValue(false),
		TLSOptStdEnvVars:                types.BoolValue(false),

		MasqAddress: types.StringValue(""),
		Banner:      types.StringValue(""),
		Options:     types.StringValue(""),
		DirMask:     types.StringValue("022"),
		FileMask:    types.StringValue("077"),
		TLSPolicy:   types.StringValue("on"),

		SSLTLSCertificate: types.Int64Value(0),
		AnonPath:          types.StringValue(""),
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// every field whose model value is null or unknown, across all field types
// including the two nullable fields.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &FTPConfigModel{
		Port:              types.Int64Null(),
		Clients:           types.Int64Unknown(),
		IPConnections:     types.Int64Null(),
		LoginAttempt:      types.Int64Null(),
		Timeout:           types.Int64Null(),
		TimeoutNoTransfer: types.Int64Null(),
		LocalUserBW:       types.Int64Null(),
		LocalUserDLBW:     types.Int64Null(),
		AnonUserBW:        types.Int64Null(),
		AnonUserDLBW:      types.Int64Null(),
		PassivePortsMin:   types.Int64Null(),
		PassivePortsMax:   types.Int64Null(),

		DefaultRoot:                     types.BoolNull(),
		OnlyAnonymous:                   types.BoolUnknown(),
		OnlyLocal:                       types.BoolNull(),
		Ident:                           types.BoolNull(),
		FXP:                             types.BoolNull(),
		Resume:                          types.BoolNull(),
		ReverseDNS:                      types.BoolNull(),
		TLS:                             types.BoolNull(),
		TLSOptAllowClientRenegotiations: types.BoolNull(),
		TLSOptAllowDotLogin:             types.BoolNull(),
		TLSOptAllowPerUser:              types.BoolNull(),
		TLSOptCommonNameRequired:        types.BoolNull(),
		TLSOptDNSNameRequired:           types.BoolNull(),
		TLSOptEnableDiags:               types.BoolNull(),
		TLSOptExportCertData:            types.BoolNull(),
		TLSOptIPAddressRequired:         types.BoolNull(),
		TLSOptNoEmptyFragments:          types.BoolNull(),
		TLSOptNoSessionReuseRequired:    types.BoolNull(),
		TLSOptStdEnvVars:                types.BoolNull(),

		MasqAddress: types.StringNull(),
		Banner:      types.StringNull(),
		Options:     types.StringUnknown(),
		DirMask:     types.StringNull(),
		FileMask:    types.StringNull(),
		TLSPolicy:   types.StringNull(),

		SSLTLSCertificate: types.Int64Null(),
		AnonPath:          types.StringUnknown(),
	}

	p := m.updatePayload()

	allKeys := []string{
		"port", "clients", "ipconnections", "loginattempt", "timeout",
		"timeout_notransfer", "localuserbw", "localuserdlbw", "anonuserbw",
		"anonuserdlbw", "passiveportsmin", "passiveportsmax",
		"defaultroot", "onlyanonymous", "onlylocal", "ident", "fxp", "resume",
		"reversedns", "tls",
		"tls_opt_allow_client_renegotiations", "tls_opt_allow_dot_login",
		"tls_opt_allow_per_user", "tls_opt_common_name_required",
		"tls_opt_dns_name_required", "tls_opt_enable_diags",
		"tls_opt_export_cert_data", "tls_opt_ip_address_required",
		"tls_opt_no_empty_fragments", "tls_opt_no_session_reuse_required",
		"tls_opt_stdenvvars",
		"masqaddress", "banner", "options", "dirmask", "filemask", "tls_policy",
		"ssltls_certificate", "anonpath",
	}
	for _, k := range allKeys {
		if _, ok := p[k]; ok {
			t.Errorf("expected %q to be omitted (null/unknown)", k)
		}
	}
	if len(p) != 0 {
		t.Errorf("expected empty payload, got %v", p)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded plain field when its model value is known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	m := baseModel()
	m.SSLTLSCertificate = types.Int64Value(9)
	m.AnonPath = types.StringValue("/mnt/pool/ftp")

	p := m.updatePayload()

	want := map[string]any{
		"port":               int64(21),
		"clients":            int64(5),
		"ipconnections":      int64(2),
		"loginattempt":       int64(1),
		"timeout":            int64(600),
		"timeout_notransfer": int64(300),
		"localuserbw":        int64(0),
		"localuserdlbw":      int64(0),
		"anonuserbw":         int64(0),
		"anonuserdlbw":       int64(0),
		"passiveportsmin":    int64(0),
		"passiveportsmax":    int64(0),

		"defaultroot":                         true,
		"onlyanonymous":                       false,
		"onlylocal":                           false,
		"ident":                               false,
		"fxp":                                 false,
		"resume":                              true,
		"reversedns":                          false,
		"tls":                                 false,
		"tls_opt_allow_client_renegotiations": false,
		"tls_opt_allow_dot_login":             false,
		"tls_opt_allow_per_user":              false,
		"tls_opt_common_name_required":        false,
		"tls_opt_dns_name_required":           false,
		"tls_opt_enable_diags":                false,
		"tls_opt_export_cert_data":            false,
		"tls_opt_ip_address_required":         false,
		"tls_opt_no_empty_fragments":          false,
		"tls_opt_no_session_reuse_required":   false,
		"tls_opt_stdenvvars":                  false,

		"masqaddress": "",
		"banner":      "",
		"options":     "",
		"dirmask":     "022",
		"filemask":    "077",
		"tls_policy":  "on",

		"ssltls_certificate": int64(9),
		"anonpath":           "/mnt/pool/ftp",
	}

	for k, v := range want {
		got, ok := p[k]
		if !ok {
			t.Errorf("payload missing key %q", k)
			continue
		}
		if got != v {
			t.Errorf("payload[%q] = %v (%T), want %v (%T)", k, got, got, v, v)
		}
	}
	if len(p) != len(want) {
		t.Errorf("payload has %d keys, want %d: %v", len(p), len(want), p)
	}
}

// TestUpdatePayload_SSLTLSCertificateThreeWay verifies the three-way
// handling for the nullable ssltls_certificate field: null/unknown is
// omitted, an explicit 0 is sent as nil (clearing the certificate), and any
// other value is sent as-is.
func TestUpdatePayload_SSLTLSCertificateThreeWay(t *testing.T) {
	cases := []struct {
		name      string
		value     types.Int64
		wantKey   bool
		wantValue any
	}{
		{"null", types.Int64Null(), false, nil},
		{"unknown", types.Int64Unknown(), false, nil},
		{"zero", types.Int64Value(0), true, nil},
		{"positive", types.Int64Value(5), true, int64(5)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModel()
			m.SSLTLSCertificate = tc.value

			p := m.updatePayload()
			v, ok := p["ssltls_certificate"]
			if ok != tc.wantKey {
				t.Fatalf("ssltls_certificate present = %v, want %v", ok, tc.wantKey)
			}
			if tc.wantKey && v != tc.wantValue {
				t.Errorf("ssltls_certificate = %v, want %v", v, tc.wantValue)
			}
		})
	}
}

// TestUpdatePayload_AnonPathThreeWay verifies the three-way handling for the
// nullable anonpath field: null/unknown is omitted, an explicit "" is sent
// as nil (clearing the path), and any other value is sent as-is.
func TestUpdatePayload_AnonPathThreeWay(t *testing.T) {
	cases := []struct {
		name      string
		value     types.String
		wantKey   bool
		wantValue any
	}{
		{"null", types.StringNull(), false, nil},
		{"unknown", types.StringUnknown(), false, nil},
		{"empty", types.StringValue(""), true, nil},
		{"set", types.StringValue("/mnt/pool/ftp"), true, "/mnt/pool/ftp"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModel()
			m.AnonPath = tc.value

			p := m.updatePayload()
			v, ok := p["anonpath"]
			if ok != tc.wantKey {
				t.Fatalf("anonpath present = %v, want %v", ok, tc.wantKey)
			}
			if tc.wantKey && v != tc.wantValue {
				t.Errorf("anonpath = %v, want %v", v, tc.wantValue)
			}
		})
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// FTPConfigResource.Delete calls only this pure function.
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
	if detail != "FTP configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestFTPConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag on
// FTPConfigDataSourceModel has a corresponding attribute in the datasource
// schema, and vice versa. terraform-plugin-framework requires an exact
// field/attribute match: any mismatch causes every datasource Read to fail
// with "Struct defines fields not found in object: ...".
func TestFTPConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &FTPConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(FTPConfigDataSourceModel{})
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
		t.Fatalf("FTPConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
