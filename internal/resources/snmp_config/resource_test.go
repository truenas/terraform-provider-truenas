// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSNMPConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestSNMPConfigSchema_IDIsComputed(t *testing.T) {
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

// TestSNMPConfigSchema_SecretsAreSensitiveWriteOnly verifies that
// "v3_password" and "v3_privpassphrase" are Optional + Sensitive, and
// specifically NOT Computed (write-only: never read back from TrueNAS, so
// they must not participate in drift detection).
func TestSNMPConfigSchema_SecretsAreSensitiveWriteOnly(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"v3_password", "v3_privpassphrase"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if !strAttr.IsSensitive() {
			t.Errorf("%q should be Sensitive", name)
		}
		if strAttr.IsComputed() {
			t.Errorf("%q should NOT be Computed (write-only, never returned by the API)", name)
		}
		if strAttr.IsRequired() {
			t.Errorf("%q should not be Required", name)
		}
		if !strAttr.IsWriteOnly() {
			t.Errorf("%q should be WriteOnly", name)
		}
	}
}

// TestSNMPConfigSchema_CommunityIsSensitive verifies that "community" is
// Optional+Computed+Sensitive: it is API-echoed (snmp.config returns it), so
// unlike v3_password/v3_privpassphrase it must stay Computed and persisted in
// state (not WriteOnly), but it is still a secret and must not print in plan
// output.
func TestSNMPConfigSchema_CommunityIsSensitive(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["community"]
	if !ok {
		t.Fatal("schema missing 'community' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'community' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'community' should be Optional+Computed")
	}
	if !strAttr.IsSensitive() {
		t.Error("'community' should be Sensitive")
	}
	if strAttr.IsWriteOnly() {
		t.Error("'community' should NOT be WriteOnly (it is API-echoed and read back)")
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("'community' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestSNMPConfigSchema_OtherFieldsAreOptionalComputed verifies that every
// non-secret, non-id field is Optional+Computed with UseStateForUnknown plan
// modifiers, matching the "non-secret fields" contract in the task brief.
func TestSNMPConfigSchema_OtherFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"contact", "location", "options", "v3_username", "v3_authtype", "v3_privproto"} {
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

	loglevelAttr, ok := s.Attributes["loglevel"]
	if !ok {
		t.Fatal("schema missing 'loglevel' attribute")
	}
	loglevelInt, ok := loglevelAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'loglevel' attribute is %T, want schema.Int64Attribute", loglevelAttr)
	}
	if !loglevelInt.IsOptional() || !loglevelInt.IsComputed() {
		t.Error("'loglevel' should be Optional+Computed")
	}
	if len(loglevelInt.PlanModifiers) == 0 {
		t.Error("'loglevel' should have plan modifiers (UseStateForUnknown)")
	}

	for _, name := range []string{"traps", "zilstat", "v3"} {
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

// TestResponseToModel_SecretsNeverSet verifies that responseToModel never
// writes to m.V3Password or m.V3PrivPassphrase, regardless of their prior
// value: both are write-only and the API never returns usable values for
// them.
func TestResponseToModel_SecretsNeverSet(t *testing.T) {
	api := &snmpConfigAPI{
		ID:         1,
		Community:  "public",
		Contact:    "",
		Location:   "",
		LogLevel:   3,
		V3AuthType: "SHA",
		V3Password: "", // API always returns empty/masked
	}

	m := &SNMPConfigModel{
		V3Password:       types.StringValue("super-secret"),
		V3PrivPassphrase: types.StringValue("also-secret"),
	}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.V3Password.ValueString() != "super-secret" {
		t.Errorf("V3Password = %q, want unchanged %q", m.V3Password.ValueString(), "super-secret")
	}
	if m.V3PrivPassphrase.ValueString() != "also-secret" {
		t.Errorf("V3PrivPassphrase = %q, want unchanged %q", m.V3PrivPassphrase.ValueString(), "also-secret")
	}

	m2 := &SNMPConfigModel{
		V3Password:       types.StringNull(),
		V3PrivPassphrase: types.StringNull(),
	}
	diags = responseToModel(api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m2.V3Password.IsNull() {
		t.Errorf("V3Password = %v, want still null", m2.V3Password)
	}
	if !m2.V3PrivPassphrase.IsNull() {
		t.Errorf("V3PrivPassphrase = %v, want still null", m2.V3PrivPassphrase)
	}
}

// TestResponseToModel_V3PrivProtoNilBecomesEmptyString verifies that a nil
// v3_privproto from the API maps to an empty string in the model.
func TestResponseToModel_V3PrivProtoNilBecomesEmptyString(t *testing.T) {
	api := &snmpConfigAPI{ID: 1, V3PrivProto: nil}

	m := &SNMPConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.V3PrivProto.IsNull() {
		t.Error("V3PrivProto should not be null when API returns nil, want empty string")
	}
	if m.V3PrivProto.ValueString() != "" {
		t.Errorf("V3PrivProto = %q, want empty string", m.V3PrivProto.ValueString())
	}
}

// TestResponseToModel_V3PrivProtoSet verifies that a non-nil v3_privproto
// from the API is carried through as-is.
func TestResponseToModel_V3PrivProtoSet(t *testing.T) {
	proto := "AES"
	api := &snmpConfigAPI{ID: 1, V3PrivProto: &proto}

	m := &SNMPConfigModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.V3PrivProto.ValueString() != "AES" {
		t.Errorf("V3PrivProto = %q, want %q", m.V3PrivProto.ValueString(), "AES")
	}
	if m.ID.ValueString() != snmpConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), snmpConfigResourceID)
	}
}

// TestResponseToDataSourceModel_V3PrivProtoNilBecomesEmptyString mirrors the
// same nil-to-empty-string handling for the datasource model.
func TestResponseToDataSourceModel_V3PrivProtoNilBecomesEmptyString(t *testing.T) {
	api := &snmpConfigAPI{ID: 1, V3PrivProto: nil}

	m := &SNMPConfigDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.V3PrivProto.ValueString() != "" {
		t.Errorf("V3PrivProto = %q, want empty string", m.V3PrivProto.ValueString())
	}
	if m.ID.ValueString() != snmpConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), snmpConfigResourceID)
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// any field whose model value is null or unknown, for every guarded field
// except v3_privproto/v3_password/v3_privpassphrase (which have their own
// tests below).
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &SNMPConfigModel{
		Community:        types.StringNull(),
		Contact:          types.StringValue("ops@example.com"),
		Location:         types.StringUnknown(),
		LogLevel:         types.Int64Value(3),
		Options:          types.StringNull(),
		Traps:            types.BoolUnknown(),
		Zilstat:          types.BoolNull(),
		V3:               types.BoolValue(true),
		V3Username:       types.StringNull(),
		V3AuthType:       types.StringUnknown(),
		V3Password:       types.StringNull(),
		V3PrivProto:      types.StringNull(),
		V3PrivPassphrase: types.StringNull(),
	}

	p := m.updatePayload()

	if _, ok := p["community"]; ok {
		t.Error("expected 'community' to be omitted (null)")
	}
	if v, ok := p["contact"]; !ok || v != "ops@example.com" {
		t.Errorf("expected 'contact' = 'ops@example.com', got %v (present=%v)", v, ok)
	}
	if _, ok := p["location"]; ok {
		t.Error("expected 'location' to be omitted (unknown)")
	}
	if v, ok := p["loglevel"]; !ok || v != int64(3) {
		t.Errorf("expected 'loglevel' = 3, got %v (present=%v)", v, ok)
	}
	if _, ok := p["options"]; ok {
		t.Error("expected 'options' to be omitted (null)")
	}
	if _, ok := p["traps"]; ok {
		t.Error("expected 'traps' to be omitted (unknown)")
	}
	if _, ok := p["zilstat"]; ok {
		t.Error("expected 'zilstat' to be omitted (null)")
	}
	if v, ok := p["v3"]; !ok || v != true {
		t.Errorf("expected 'v3' = true, got %v (present=%v)", v, ok)
	}
	if _, ok := p["v3_username"]; ok {
		t.Error("expected 'v3_username' to be omitted (null)")
	}
	if _, ok := p["v3_authtype"]; ok {
		t.Error("expected 'v3_authtype' to be omitted (unknown)")
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded non-secret, non-nullable field when its model value is
// known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	m := &SNMPConfigModel{
		Community:        types.StringValue("public"),
		Contact:          types.StringValue("ops@example.com"),
		Location:         types.StringValue("DC1"),
		LogLevel:         types.Int64Value(3),
		Options:          types.StringValue(""),
		Traps:            types.BoolValue(true),
		Zilstat:          types.BoolValue(false),
		V3:               types.BoolValue(true),
		V3Username:       types.StringValue("snmpv3user"),
		V3AuthType:       types.StringValue("SHA"),
		V3Password:       types.StringNull(),
		V3PrivProto:      types.StringNull(),
		V3PrivPassphrase: types.StringNull(),
	}

	p := m.updatePayload()

	want := map[string]any{
		"community":   "public",
		"contact":     "ops@example.com",
		"location":    "DC1",
		"loglevel":    int64(3),
		"options":     "",
		"traps":       true,
		"zilstat":     false,
		"v3":          true,
		"v3_username": "snmpv3user",
		"v3_authtype": "SHA",
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if len(p) != len(want) {
		t.Errorf("payload has %d keys (%v), want %d (v3_privproto/v3_password/v3_privpassphrase should be omitted)", len(p), p, len(want))
	}
}

// TestUpdatePayload_V3PrivProtoThreeWay verifies the three-way nullable
// handling for v3_privproto: omitted when null/unknown, sent as nil when
// explicitly set to "", and sent as the value otherwise.
func TestUpdatePayload_V3PrivProtoThreeWay(t *testing.T) {
	base := func() *SNMPConfigModel {
		return &SNMPConfigModel{
			Community:        types.StringValue("public"),
			Contact:          types.StringValue(""),
			Location:         types.StringValue(""),
			LogLevel:         types.Int64Value(3),
			Options:          types.StringValue(""),
			Traps:            types.BoolValue(false),
			Zilstat:          types.BoolValue(false),
			V3:               types.BoolValue(true),
			V3Username:       types.StringValue("user"),
			V3AuthType:       types.StringValue("SHA"),
			V3Password:       types.StringNull(),
			V3PrivPassphrase: types.StringNull(),
		}
	}

	m := base()
	m.V3PrivProto = types.StringNull()
	p := m.updatePayload()
	if _, ok := p["v3_privproto"]; ok {
		t.Error("expected 'v3_privproto' to be omitted when null")
	}

	m2 := base()
	m2.V3PrivProto = types.StringUnknown()
	p2 := m2.updatePayload()
	if _, ok := p2["v3_privproto"]; ok {
		t.Error("expected 'v3_privproto' to be omitted when unknown")
	}

	m3 := base()
	m3.V3PrivProto = types.StringValue("")
	p3 := m3.updatePayload()
	if v, ok := p3["v3_privproto"]; !ok {
		t.Error("expected 'v3_privproto' to be present (nil) when explicitly set to empty string")
	} else if v != nil {
		t.Errorf("expected 'v3_privproto' = nil when explicitly set to empty string, got %v", v)
	}

	m4 := base()
	m4.V3PrivProto = types.StringValue("AES")
	p4 := m4.updatePayload()
	if v, ok := p4["v3_privproto"]; !ok || v != "AES" {
		t.Errorf("expected 'v3_privproto' = 'AES', got %v (present=%v)", v, ok)
	}
}

// TestUpdatePayload_SecretsOnlyWhenSet verifies that v3_password and
// v3_privpassphrase are included only when they have a known, non-null
// value, and are otherwise omitted entirely (never sent as an empty string
// or null).
func TestUpdatePayload_SecretsOnlyWhenSet(t *testing.T) {
	base := func() *SNMPConfigModel {
		return &SNMPConfigModel{
			Community:   types.StringValue("public"),
			Contact:     types.StringValue(""),
			Location:    types.StringValue(""),
			LogLevel:    types.Int64Value(3),
			Options:     types.StringValue(""),
			Traps:       types.BoolValue(false),
			Zilstat:     types.BoolValue(false),
			V3:          types.BoolValue(true),
			V3Username:  types.StringValue("user"),
			V3AuthType:  types.StringValue("SHA"),
			V3PrivProto: types.StringNull(),
		}
	}

	m := base()
	m.V3Password = types.StringValue("hunter2")
	m.V3PrivPassphrase = types.StringNull()
	p := m.updatePayload()
	if v, ok := p["v3_password"]; !ok || v != "hunter2" {
		t.Errorf("expected 'v3_password' = 'hunter2', got %v (present=%v)", v, ok)
	}
	if _, ok := p["v3_privpassphrase"]; ok {
		t.Error("expected 'v3_privpassphrase' to be omitted when null")
	}

	m2 := base()
	m2.V3Password = types.StringNull()
	m2.V3PrivPassphrase = types.StringValue("priv-secret")
	p2 := m2.updatePayload()
	if _, ok := p2["v3_password"]; ok {
		t.Error("expected 'v3_password' to be omitted when null")
	}
	if v, ok := p2["v3_privpassphrase"]; !ok || v != "priv-secret" {
		t.Errorf("expected 'v3_privpassphrase' = 'priv-secret', got %v (present=%v)", v, ok)
	}

	m3 := base()
	m3.V3Password = types.StringUnknown()
	m3.V3PrivPassphrase = types.StringUnknown()
	p3 := m3.updatePayload()
	if _, ok := p3["v3_password"]; ok {
		t.Error("expected 'v3_password' to be omitted when unknown")
	}
	if _, ok := p3["v3_privpassphrase"]; ok {
		t.Error("expected 'v3_privpassphrase' to be omitted when unknown")
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// SNMPConfigResource.Delete calls only this pure function.
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
	if detail != "SNMP configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestSNMPConfigDataSourceModel_NoSecretFields verifies that
// SNMPConfigDataSourceModel has no v3_password or v3_privpassphrase field:
// the API never returns usable values for either, so datasource attributes
// for them would always read as empty/unknown.
func TestSNMPConfigDataSourceModel_NoSecretFields(t *testing.T) {
	modelType := reflect.TypeOf(SNMPConfigDataSourceModel{})
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "v3_password" || tag == "v3_privpassphrase" {
			t.Errorf("SNMPConfigDataSourceModel must not have a %q field (write-only secret)", tag)
		}
	}
}

// TestSNMPConfigDataSourceModel_MatchesSchema verifies that every tfsdk tag
// on SNMPConfigDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestSNMPConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &SNMPConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(SNMPConfigDataSourceModel{})
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
		t.Fatalf("SNMPConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
