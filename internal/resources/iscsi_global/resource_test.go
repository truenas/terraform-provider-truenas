// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_global

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestISCSIGlobalSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestISCSIGlobalSchema_IDIsComputed(t *testing.T) {
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

// TestISCSIGlobalSchema_AllFieldsOptionalComputed verifies that every
// non-id field is Optional+Computed with UseStateForUnknown plan modifiers.
func TestISCSIGlobalSchema_AllFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	basenameAttr, ok := s.Attributes["basename"]
	if !ok {
		t.Fatal("schema missing 'basename' attribute")
	}
	basenameStr, ok := basenameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'basename' attribute is %T, want schema.StringAttribute", basenameAttr)
	}
	if !basenameStr.IsOptional() || !basenameStr.IsComputed() {
		t.Error("'basename' should be Optional+Computed")
	}
	if len(basenameStr.PlanModifiers) == 0 {
		t.Error("'basename' should have plan modifiers (UseStateForUnknown)")
	}

	listenPortAttr, ok := s.Attributes["listen_port"]
	if !ok {
		t.Fatal("schema missing 'listen_port' attribute")
	}
	listenPortInt, ok := listenPortAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'listen_port' attribute is %T, want schema.Int64Attribute", listenPortAttr)
	}
	if !listenPortInt.IsOptional() || !listenPortInt.IsComputed() {
		t.Error("'listen_port' should be Optional+Computed")
	}
	if len(listenPortInt.PlanModifiers) == 0 {
		t.Error("'listen_port' should have plan modifiers (UseStateForUnknown)")
	}

	for _, name := range []string{"alua", "iser"} {
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

	isnsAttr, ok := s.Attributes["isns_servers"]
	if !ok {
		t.Fatal("schema missing 'isns_servers' attribute")
	}
	isnsList, ok := isnsAttr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("'isns_servers' attribute is %T, want schema.ListAttribute", isnsAttr)
	}
	if !isnsList.IsOptional() || !isnsList.IsComputed() {
		t.Error("'isns_servers' should be Optional+Computed")
	}
	if len(isnsList.PlanModifiers) == 0 {
		t.Error("'isns_servers' should have plan modifiers (UseStateForUnknown)")
	}

	thresholdAttr, ok := s.Attributes["pool_avail_threshold"]
	if !ok {
		t.Fatal("schema missing 'pool_avail_threshold' attribute")
	}
	thresholdInt, ok := thresholdAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'pool_avail_threshold' attribute is %T, want schema.Int64Attribute", thresholdAttr)
	}
	if !thresholdInt.IsOptional() || !thresholdInt.IsComputed() {
		t.Error("'pool_avail_threshold' should be Optional+Computed")
	}
	if len(thresholdInt.PlanModifiers) == 0 {
		t.Error("'pool_avail_threshold' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestResponseToModel_ISNSServersNilBecomesEmptyList verifies that a nil
// ISNSServers slice from the API maps to an empty (non-null) list in the
// model.
func TestResponseToModel_ISNSServersNilBecomesEmptyList(t *testing.T) {
	api := &iscsiGlobalAPI{
		ID:                 1,
		Basename:           "iqn.2005-10.org.freenas.ctl",
		ListenPort:         3260,
		ALUA:               false,
		ISER:               false,
		ISNSServers:        nil,
		PoolAvailThreshold: nil,
	}

	m := &ISCSIGlobalModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ISNSServers.IsNull() {
		t.Error("ISNSServers should not be null when API returns nil, want empty list")
	}

	var out []string
	diags = m.ISNSServers.ElementsAs(context.Background(), &out, false)
	if diags.HasError() {
		t.Fatalf("unexpected error extracting elements: %v", diags)
	}
	if len(out) != 0 {
		t.Errorf("ISNSServers = %v, want empty slice", out)
	}
}

// TestResponseToModel_PoolAvailThresholdNilBecomesZero verifies that a nil
// PoolAvailThreshold from the API maps to 0 in the model.
func TestResponseToModel_PoolAvailThresholdNilBecomesZero(t *testing.T) {
	api := &iscsiGlobalAPI{
		ID:                 1,
		Basename:           "iqn.2005-10.org.freenas.ctl",
		ListenPort:         3260,
		ISNSServers:        []string{},
		PoolAvailThreshold: nil,
	}

	m := &ISCSIGlobalModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.PoolAvailThreshold.ValueInt64() != 0 {
		t.Errorf("PoolAvailThreshold = %d, want 0", m.PoolAvailThreshold.ValueInt64())
	}
	if m.ID.ValueString() != iscsiGlobalResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), iscsiGlobalResourceID)
	}
}

// TestResponseToModel_PoolAvailThresholdSet verifies that a non-nil
// PoolAvailThreshold from the API is carried through as-is.
func TestResponseToModel_PoolAvailThresholdSet(t *testing.T) {
	threshold := int64(20)
	api := &iscsiGlobalAPI{
		ID:                 1,
		Basename:           "iqn.2005-10.org.freenas.ctl",
		ListenPort:         3260,
		ISNSServers:        []string{"isns.example.com"},
		PoolAvailThreshold: &threshold,
	}

	m := &ISCSIGlobalModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.PoolAvailThreshold.ValueInt64() != threshold {
		t.Errorf("PoolAvailThreshold = %d, want %d", m.PoolAvailThreshold.ValueInt64(), threshold)
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any
// field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &ISCSIGlobalModel{
		Basename:           types.StringNull(),
		ListenPort:         types.Int64Value(3260),
		ALUA:               types.BoolUnknown(),
		ISER:               types.BoolNull(),
		ISNSServers:        types.ListNull(types.StringType),
		PoolAvailThreshold: types.Int64Null(),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if _, ok := p["basename"]; ok {
		t.Error("expected 'basename' to be omitted (null)")
	}
	if v, ok := p["listen_port"]; !ok || v != int64(3260) {
		t.Errorf("expected 'listen_port' = 3260, got %v (present=%v)", v, ok)
	}
	if _, ok := p["alua"]; ok {
		t.Error("expected 'alua' to be omitted (unknown)")
	}
	if _, ok := p["iser"]; ok {
		t.Error("expected 'iser' to be omitted (null)")
	}
	if _, ok := p["isns_servers"]; ok {
		t.Error("expected 'isns_servers' to be omitted (null)")
	}
	if _, ok := p["pool_avail_threshold"]; ok {
		t.Error("expected 'pool_avail_threshold' to be omitted (null)")
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	servers, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"isns1.example.com", "isns2.example.com"})
	if diags.HasError() {
		t.Fatalf("unexpected error building list: %v", diags)
	}

	m := &ISCSIGlobalModel{
		Basename:           types.StringValue("iqn.2005-10.org.freenas.ctl:custom"),
		ListenPort:         types.Int64Value(3260),
		ALUA:               types.BoolValue(true),
		ISER:               types.BoolValue(true),
		ISNSServers:        servers,
		PoolAvailThreshold: types.Int64Value(20),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"basename":             "iqn.2005-10.org.freenas.ctl:custom",
		"listen_port":          int64(3260),
		"alua":                 true,
		"iser":                 true,
		"pool_avail_threshold": int64(20),
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	isnsList, ok := p["isns_servers"].([]string)
	if !ok {
		t.Fatalf("payload[isns_servers] is %T, want []string", p["isns_servers"])
	}
	if len(isnsList) != 2 || isnsList[0] != "isns1.example.com" || isnsList[1] != "isns2.example.com" {
		t.Errorf("payload[isns_servers] = %v, want [isns1.example.com isns2.example.com]", isnsList)
	}
	if len(p) != len(want)+1 {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want)+1)
	}
}

// TestUpdatePayload_PoolAvailThresholdZeroSendsNil verifies that an
// explicitly-set 0 threshold is sent as a nil value in the payload (clearing
// the threshold on TrueNAS), not omitted and not sent as the literal 0.
func TestUpdatePayload_PoolAvailThresholdZeroSendsNil(t *testing.T) {
	m := &ISCSIGlobalModel{
		Basename:           types.StringNull(),
		ListenPort:         types.Int64Null(),
		ALUA:               types.BoolNull(),
		ISER:               types.BoolNull(),
		ISNSServers:        types.ListNull(types.StringType),
		PoolAvailThreshold: types.Int64Value(0),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	v, ok := p["pool_avail_threshold"]
	if !ok {
		t.Fatal("expected 'pool_avail_threshold' to be present (nil) when explicitly set to 0")
	}
	if v != nil {
		t.Errorf("expected 'pool_avail_threshold' = nil, got %v", v)
	}
}

// TestUpdatePayload_PoolAvailThresholdNullOmitted verifies that a null/unknown
// threshold is omitted entirely from the payload.
func TestUpdatePayload_PoolAvailThresholdNullOmitted(t *testing.T) {
	base := ISCSIGlobalModel{
		Basename:    types.StringNull(),
		ListenPort:  types.Int64Null(),
		ALUA:        types.BoolNull(),
		ISER:        types.BoolNull(),
		ISNSServers: types.ListNull(types.StringType),
	}

	nullModel := base
	nullModel.PoolAvailThreshold = types.Int64Null()
	p, diags := nullModel.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["pool_avail_threshold"]; ok {
		t.Error("expected 'pool_avail_threshold' to be omitted when null")
	}

	unknownModel := base
	unknownModel.PoolAvailThreshold = types.Int64Unknown()
	p2, diags := unknownModel.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p2["pool_avail_threshold"]; ok {
		t.Error("expected 'pool_avail_threshold' to be omitted when unknown")
	}
}

// TestUpdatePayload_ISNSServersNilElementsBecomeEmptySlice verifies that a
// known but empty isns_servers list is sent as an empty slice, not nil/null.
func TestUpdatePayload_ISNSServersNilElementsBecomeEmptySlice(t *testing.T) {
	emptyList, diags := types.ListValueFrom(context.Background(), types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("unexpected error building empty list: %v", diags)
	}

	m := &ISCSIGlobalModel{
		Basename:           types.StringNull(),
		ListenPort:         types.Int64Null(),
		ALUA:               types.BoolNull(),
		ISER:               types.BoolNull(),
		ISNSServers:        emptyList,
		PoolAvailThreshold: types.Int64Null(),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	isnsList, ok := p["isns_servers"].([]string)
	if !ok {
		t.Fatalf("payload[isns_servers] is %T, want []string", p["isns_servers"])
	}
	if len(isnsList) != 0 {
		t.Errorf("payload[isns_servers] = %v, want empty slice", isnsList)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// ISCSIGlobalResource.Delete calls only this pure function.
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
	if detail != "iSCSI global configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestISCSIGlobalDataSourceModel_MatchesSchema verifies that every tfsdk tag
// on ISCSIGlobalDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestISCSIGlobalDataSourceModel_MatchesSchema(t *testing.T) {
	d := &ISCSIGlobalDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(ISCSIGlobalDataSourceModel{})
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
		t.Fatalf("ISCSIGlobalDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
