// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSystemGeneralSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestSystemGeneralSchema_IDIsComputed(t *testing.T) {
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

// TestSystemGeneralSchema_WritableFieldsOptionalComputed verifies that every
// writable field is Optional+Computed with a UseStateForUnknown plan
// modifier.
func TestSystemGeneralSchema_WritableFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	boolFields := []string{"ds_auth", "ui_consolemsg", "ui_httpsredirect", "usage_collection"}
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

	stringFields := []string{"kbdmap", "timezone", "ui_x_frame_options"}
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

	int64Fields := []string{"ui_certificate", "ui_httpsport", "ui_port"}
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

	listFields := []string{"ui_address", "ui_allowlist", "ui_httpsprotocols", "ui_v6address"}
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
	}
}

// TestSystemGeneralSchema_ComputedOnlyTrio verifies that
// ui_certificate_name, usage_collection_is_set, and wizardshown are
// Computed-only (not Optional) with NO plan modifiers:
// ui_certificate_name is derived from ui_certificate and changes when the
// certificate is reassigned, so carrying prior state forward would trip
// Terraform's post-apply consistency check.
func TestSystemGeneralSchema_ComputedOnlyTrio(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["ui_certificate_name"]
	if !ok {
		t.Fatal("schema missing 'ui_certificate_name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'ui_certificate_name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsComputed() || nameStr.IsOptional() {
		t.Error("'ui_certificate_name' should be Computed-only")
	}
	if len(nameStr.PlanModifiers) != 0 {
		t.Error("'ui_certificate_name' should have NO plan modifiers (derived from ui_certificate)")
	}

	for _, name := range []string{"usage_collection_is_set", "wizardshown"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsComputed() || boolAttr.IsOptional() {
			t.Errorf("%q should be Computed-only", name)
		}
		if len(boolAttr.PlanModifiers) != 0 {
			t.Errorf("%q should have NO plan modifiers, got %d", name, len(boolAttr.PlanModifiers))
		}
	}
}

// TestResponseToModel_NullableFieldsMapToNull verifies that a nil
// ui_certificate/usage_collection on the wire maps to Int64Null/BoolNull
// (not a zero value), and that a non-nil value maps through unchanged.
func TestResponseToModel_NullableFieldsMapToNull(t *testing.T) {
	api := &systemGeneralAPI{
		ID:               1,
		DSAuth:           false,
		Kbdmap:           "us",
		Timezone:         "America/Los_Angeles",
		UIAddress:        []string{"0.0.0.0"},
		UIAllowlist:      []string{},
		UICertificate:    uiCertificate{},
		UIConsolemsg:     false,
		UIHTTPSPort:      443,
		UIHTTPSProtocols: []string{"TLSv1.2", "TLSv1.3"},
		UIHTTPSRedirect:  false,
		UIPort:           80,
		UIV6Address:      []string{"::"},
		UIXFrameOptions:  "SAMEORIGIN",
		UsageCollection:  nil,
	}

	m := &SystemGeneralModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.UICertificate.IsNull() {
		t.Errorf("UICertificate = %v, want null", m.UICertificate)
	}
	if !m.UsageCollection.IsNull() {
		t.Errorf("UsageCollection = %v, want null", m.UsageCollection)
	}

	cert := int64(1)
	usage := true
	api.UICertificate = uiCertificate{ID: &cert}
	api.UsageCollection = &usage

	m2 := &SystemGeneralModel{}
	diags = responseToModel(context.Background(), api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m2.UICertificate.IsNull() || m2.UICertificate.ValueInt64() != 1 {
		t.Errorf("UICertificate = %v, want 1", m2.UICertificate)
	}
	if m2.UsageCollection.IsNull() || !m2.UsageCollection.ValueBool() {
		t.Errorf("UsageCollection = %v, want true", m2.UsageCollection)
	}
}

// TestResponseToModel_ListsNilMapToEmpty verifies that nil list fields from
// the API map to empty (non-null) Terraform lists.
func TestResponseToModel_ListsNilMapToEmpty(t *testing.T) {
	api := &systemGeneralAPI{
		UIAddress:        nil,
		UIAllowlist:      nil,
		UIHTTPSProtocols: nil,
		UIV6Address:      nil,
	}

	m := &SystemGeneralModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for name, l := range map[string]types.List{
		"ui_address":        m.UIAddress,
		"ui_allowlist":      m.UIAllowlist,
		"ui_httpsprotocols": m.UIHTTPSProtocols,
		"ui_v6address":      m.UIV6Address,
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
// non-pointer fields are copied through unchanged.
func TestResponseToModel_MapsPlainFields(t *testing.T) {
	api := &systemGeneralAPI{
		ID:                   1,
		DSAuth:               false,
		Kbdmap:               "us",
		Timezone:             "America/Los_Angeles",
		UIAddress:            []string{"0.0.0.0"},
		UIAllowlist:          []string{},
		UICertificateName:    "truenas_default",
		UIConsolemsg:         false,
		UIHTTPSPort:          443,
		UIHTTPSProtocols:     []string{"TLSv1.2", "TLSv1.3"},
		UIHTTPSRedirect:      false,
		UIPort:               80,
		UIV6Address:          []string{"::"},
		UIXFrameOptions:      "SAMEORIGIN",
		UsageCollectionIsSet: false,
		Wizardshown:          false,
	}

	m := &SystemGeneralModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != systemGeneralResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), systemGeneralResourceID)
	}
	if m.Kbdmap.ValueString() != api.Kbdmap {
		t.Errorf("Kbdmap = %q, want %q", m.Kbdmap.ValueString(), api.Kbdmap)
	}
	if m.Timezone.ValueString() != api.Timezone {
		t.Errorf("Timezone = %q, want %q", m.Timezone.ValueString(), api.Timezone)
	}
	if m.UICertificateName.ValueString() != api.UICertificateName {
		t.Errorf("UICertificateName = %q, want %q", m.UICertificateName.ValueString(), api.UICertificateName)
	}
	if m.UIHTTPSPort.ValueInt64() != api.UIHTTPSPort {
		t.Errorf("UIHTTPSPort = %d, want %d", m.UIHTTPSPort.ValueInt64(), api.UIHTTPSPort)
	}
	if m.UIPort.ValueInt64() != api.UIPort {
		t.Errorf("UIPort = %d, want %d", m.UIPort.ValueInt64(), api.UIPort)
	}
	if m.UIXFrameOptions.ValueString() != api.UIXFrameOptions {
		t.Errorf("UIXFrameOptions = %q, want %q", m.UIXFrameOptions.ValueString(), api.UIXFrameOptions)
	}
	if m.UsageCollectionIsSet.ValueBool() != api.UsageCollectionIsSet {
		t.Errorf("UsageCollectionIsSet = %v, want %v", m.UsageCollectionIsSet.ValueBool(), api.UsageCollectionIsSet)
	}
	if m.Wizardshown.ValueBool() != api.Wizardshown {
		t.Errorf("Wizardshown = %v, want %v", m.Wizardshown.ValueBool(), api.Wizardshown)
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// any field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &SystemGeneralModel{
		DSAuth:           types.BoolNull(),
		Kbdmap:           types.StringUnknown(),
		Timezone:         types.StringValue("America/Los_Angeles"),
		UIAddress:        types.ListNull(types.StringType),
		UIAllowlist:      types.ListUnknown(types.StringType),
		UICertificate:    types.Int64Null(),
		UIConsolemsg:     types.BoolUnknown(),
		UIHTTPSPort:      types.Int64Null(),
		UIHTTPSProtocols: types.ListNull(types.StringType),
		UIHTTPSRedirect:  types.BoolNull(),
		UIPort:           types.Int64Null(),
		UIV6Address:      types.ListNull(types.StringType),
		UIXFrameOptions:  types.StringNull(),
		UsageCollection:  types.BoolNull(),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{
		"ds_auth", "kbdmap", "ui_address", "ui_allowlist", "ui_certificate",
		"ui_consolemsg", "ui_httpsport", "ui_httpsprotocols", "ui_httpsredirect",
		"ui_port", "ui_v6address", "ui_x_frame_options", "usage_collection",
	} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to be omitted (null/unknown)", key)
		}
	}
	if v, ok := p["timezone"]; !ok || v != "America/Los_Angeles" {
		t.Errorf("expected 'timezone' = America/Los_Angeles, got %v (present=%v)", v, ok)
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known, including a non-zero
// ui_certificate.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	ctx := context.Background()
	uiAddress, _ := types.ListValueFrom(ctx, types.StringType, []string{"0.0.0.0"})
	uiAllowlist, _ := types.ListValueFrom(ctx, types.StringType, []string{})
	uiHTTPSProtocols, _ := types.ListValueFrom(ctx, types.StringType, []string{"TLSv1.2", "TLSv1.3"})
	uiV6Address, _ := types.ListValueFrom(ctx, types.StringType, []string{"::"})

	m := &SystemGeneralModel{
		DSAuth:           types.BoolValue(true),
		Kbdmap:           types.StringValue("us"),
		Timezone:         types.StringValue("America/Los_Angeles"),
		UIAddress:        uiAddress,
		UIAllowlist:      uiAllowlist,
		UICertificate:    types.Int64Value(1),
		UIConsolemsg:     types.BoolValue(true),
		UIHTTPSPort:      types.Int64Value(443),
		UIHTTPSProtocols: uiHTTPSProtocols,
		UIHTTPSRedirect:  types.BoolValue(true),
		UIPort:           types.Int64Value(80),
		UIV6Address:      uiV6Address,
		UIXFrameOptions:  types.StringValue("SAMEORIGIN"),
		UsageCollection:  types.BoolValue(true),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if v, ok := p["ui_certificate"]; !ok || v != int64(1) {
		t.Errorf("payload[ui_certificate] = %v (present=%v), want 1", v, ok)
	}
	if v, ok := p["usage_collection"]; !ok || v != true {
		t.Errorf("payload[usage_collection] = %v (present=%v), want true", v, ok)
	}
	if v, ok := p["ui_address"]; !ok {
		t.Error("expected 'ui_address' present")
	} else if s, ok := v.([]string); !ok || len(s) != 1 || s[0] != "0.0.0.0" {
		t.Errorf("payload[ui_address] = %v, want [0.0.0.0]", v)
	}
	if v, ok := p["ui_allowlist"]; !ok {
		t.Error("expected 'ui_allowlist' present")
	} else if s, ok := v.([]string); !ok || len(s) != 0 {
		t.Errorf("payload[ui_allowlist] = %v, want []", v)
	}

	want := map[string]any{
		"ds_auth":            true,
		"kbdmap":             "us",
		"timezone":           "America/Los_Angeles",
		"ui_consolemsg":      true,
		"ui_httpsport":       int64(443),
		"ui_httpsredirect":   true,
		"ui_port":            int64(80),
		"ui_x_frame_options": "SAMEORIGIN",
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
}

// TestUpdatePayload_UICertificateThreeWay verifies the three-way behavior
// for ui_certificate: null/unknown omits the key entirely, an explicit 0
// sends JSON nil (clearing the certificate), and any other value sends that
// value.
func TestUpdatePayload_UICertificateThreeWay(t *testing.T) {
	base := func(cert types.Int64) *SystemGeneralModel {
		return &SystemGeneralModel{
			DSAuth:           types.BoolNull(),
			Kbdmap:           types.StringNull(),
			Timezone:         types.StringNull(),
			UIAddress:        types.ListNull(types.StringType),
			UIAllowlist:      types.ListNull(types.StringType),
			UICertificate:    cert,
			UIConsolemsg:     types.BoolNull(),
			UIHTTPSPort:      types.Int64Null(),
			UIHTTPSProtocols: types.ListNull(types.StringType),
			UIHTTPSRedirect:  types.BoolNull(),
			UIPort:           types.Int64Null(),
			UIV6Address:      types.ListNull(types.StringType),
			UIXFrameOptions:  types.StringNull(),
			UsageCollection:  types.BoolNull(),
		}
	}

	// null -> omitted
	p, diags := base(types.Int64Null()).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["ui_certificate"]; ok {
		t.Error("expected 'ui_certificate' to be omitted when null")
	}

	// unknown -> omitted
	p, diags = base(types.Int64Unknown()).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["ui_certificate"]; ok {
		t.Error("expected 'ui_certificate' to be omitted when unknown")
	}

	// explicit 0 -> nil
	p, diags = base(types.Int64Value(0)).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	v, ok := p["ui_certificate"]
	if !ok {
		t.Fatal("expected 'ui_certificate' to be present when explicitly 0")
	}
	if v != nil {
		t.Errorf("ui_certificate = %v, want nil", v)
	}

	// N -> N
	p, diags = base(types.Int64Value(5)).updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["ui_certificate"]; !ok || v != int64(5) {
		t.Errorf("ui_certificate = %v (present=%v), want 5", v, ok)
	}
}

// TestUpdatePayload_ComputedOnlyTrioNeverSent verifies that
// ui_certificate_name, usage_collection_is_set, and wizardshown never
// appear in the update payload, regardless of their model values (they have
// no corresponding updatePayload field at all, so this documents the
// contract by construction: attempting to reference them would be a
// compile error).
func TestUpdatePayload_ComputedOnlyTrioNeverSent(t *testing.T) {
	m := &SystemGeneralModel{
		DSAuth:               types.BoolValue(true),
		Kbdmap:               types.StringValue("us"),
		Timezone:             types.StringValue("America/Los_Angeles"),
		UIAddress:            types.ListNull(types.StringType),
		UIAllowlist:          types.ListNull(types.StringType),
		UICertificate:        types.Int64Value(1),
		UIConsolemsg:         types.BoolValue(true),
		UIHTTPSPort:          types.Int64Value(443),
		UIHTTPSProtocols:     types.ListNull(types.StringType),
		UIHTTPSRedirect:      types.BoolValue(true),
		UIPort:               types.Int64Value(80),
		UIV6Address:          types.ListNull(types.StringType),
		UIXFrameOptions:      types.StringValue("SAMEORIGIN"),
		UsageCollection:      types.BoolValue(true),
		UICertificateName:    types.StringValue("truenas_default"),
		UsageCollectionIsSet: types.BoolValue(true),
		Wizardshown:          types.BoolValue(true),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{"ui_certificate_name", "usage_collection_is_set", "wizardshown"} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to never be present in the update payload", key)
		}
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// SystemGeneralResource.Delete calls only this pure function.
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
	if detail != "System general configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestSystemGeneralDataSourceModel_MatchesSchema verifies that every tfsdk
// tag on SystemGeneralDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestSystemGeneralDataSourceModel_MatchesSchema(t *testing.T) {
	d := &SystemGeneralDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(SystemGeneralDataSourceModel{})
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
		t.Fatalf("SystemGeneralDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

// TestUICertificateDecode_bothShapes verifies uiCertificate decodes both
// wire shapes of system.general.config's ui_certificate: TrueNAS 26.0's bare
// integer ID and 25.10's full certificate object (id + name), plus null.
func TestUICertificateDecode_bothShapes(t *testing.T) {
	var api systemGeneralAPI

	// 26.0: bare int, name in top-level ui_certificate_name.
	if err := json.Unmarshal([]byte(`{"ui_certificate": 7, "ui_certificate_name": "cert26"}`), &api); err != nil {
		t.Fatalf("26.0 shape: %v", err)
	}
	if api.UICertificate.ID == nil || *api.UICertificate.ID != 7 {
		t.Errorf("26.0 shape ID = %v, want 7", api.UICertificate.ID)
	}
	if got := api.certificateName(); got.ValueString() != "cert26" {
		t.Errorf("26.0 shape name = %v, want cert26", got)
	}

	// 25.10: full object, no top-level name field.
	api = systemGeneralAPI{}
	if err := json.Unmarshal([]byte(`{"ui_certificate": {"id": 3, "name": "truenas_default", "key_length": 2048}}`), &api); err != nil {
		t.Fatalf("25.10 shape: %v", err)
	}
	if api.UICertificate.ID == nil || *api.UICertificate.ID != 3 {
		t.Errorf("25.10 shape ID = %v, want 3", api.UICertificate.ID)
	}
	if got := api.certificateName(); got.ValueString() != "truenas_default" {
		t.Errorf("25.10 shape name = %v, want truenas_default", got)
	}

	// null: both nil, maps to null ID and empty name.
	api = systemGeneralAPI{}
	if err := json.Unmarshal([]byte(`{"ui_certificate": null}`), &api); err != nil {
		t.Fatalf("null shape: %v", err)
	}
	if !api.UICertificate.idValue().IsNull() {
		t.Errorf("null shape ID = %v, want null", api.UICertificate.idValue())
	}
	if got := api.certificateName(); got.ValueString() != "" {
		t.Errorf("null shape name = %v, want empty", got)
	}
}
