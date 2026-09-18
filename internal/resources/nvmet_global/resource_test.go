// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNVMeTGlobalSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton value
// never supplied by the user.
func TestNVMeTGlobalSchema_IDIsComputed(t *testing.T) {
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

// TestNVMeTGlobalSchema_AllFieldsOptionalComputed verifies that every non-id
// field is Optional+Computed with UseStateForUnknown plan modifiers.
func TestNVMeTGlobalSchema_AllFieldsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	basenqnAttr, ok := s.Attributes["basenqn"]
	if !ok {
		t.Fatal("schema missing 'basenqn' attribute")
	}
	basenqnStr, ok := basenqnAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'basenqn' attribute is %T, want schema.StringAttribute", basenqnAttr)
	}
	if !basenqnStr.IsOptional() || !basenqnStr.IsComputed() {
		t.Error("'basenqn' should be Optional+Computed")
	}
	if len(basenqnStr.PlanModifiers) == 0 {
		t.Error("'basenqn' should have plan modifiers (UseStateForUnknown)")
	}

	for _, name := range []string{"ana", "kernel", "rdma", "xport_referral"} {
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

// TestResponseToModel_MapsAllFields verifies that responseToModel copies
// every API field onto the Terraform model, and sets the fixed singleton ID.
func TestResponseToModel_MapsAllFields(t *testing.T) {
	api := &nvmetGlobalAPI{
		ID:            1,
		Basenqn:       "nqn.2011-06.com.truenas:uuid:269a",
		ANA:           false,
		Kernel:        true,
		RDMA:          false,
		XportReferral: true,
	}

	m := &NVMeTGlobalModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != nvmetGlobalResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), nvmetGlobalResourceID)
	}
	if m.Basenqn.ValueString() != api.Basenqn {
		t.Errorf("Basenqn = %q, want %q", m.Basenqn.ValueString(), api.Basenqn)
	}
	if m.ANA.ValueBool() != api.ANA {
		t.Errorf("ANA = %v, want %v", m.ANA.ValueBool(), api.ANA)
	}
	if m.Kernel.ValueBool() != api.Kernel {
		t.Errorf("Kernel = %v, want %v", m.Kernel.ValueBool(), api.Kernel)
	}
	if m.RDMA.ValueBool() != api.RDMA {
		t.Errorf("RDMA = %v, want %v", m.RDMA.ValueBool(), api.RDMA)
	}
	if m.XportReferral.ValueBool() != api.XportReferral {
		t.Errorf("XportReferral = %v, want %v", m.XportReferral.ValueBool(), api.XportReferral)
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits any
// field whose model value is null or unknown.
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &NVMeTGlobalModel{
		Basenqn:       types.StringNull(),
		ANA:           types.BoolUnknown(),
		Kernel:        types.BoolValue(true),
		RDMA:          types.BoolNull(),
		XportReferral: types.BoolUnknown(),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if _, ok := p["basenqn"]; ok {
		t.Error("expected 'basenqn' to be omitted (null)")
	}
	if _, ok := p["ana"]; ok {
		t.Error("expected 'ana' to be omitted (unknown)")
	}
	if v, ok := p["kernel"]; !ok || v != true {
		t.Errorf("expected 'kernel' = true, got %v (present=%v)", v, ok)
	}
	if _, ok := p["rdma"]; ok {
		t.Error("expected 'rdma' to be omitted (null)")
	}
	if _, ok := p["xport_referral"]; ok {
		t.Error("expected 'xport_referral' to be omitted (unknown)")
	}
	if len(p) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(p), p)
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	m := &NVMeTGlobalModel{
		Basenqn:       types.StringValue("nqn.2011-06.com.truenas:uuid:custom"),
		ANA:           types.BoolValue(true),
		Kernel:        types.BoolValue(true),
		RDMA:          types.BoolValue(true),
		XportReferral: types.BoolValue(false),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	want := map[string]any{
		"basenqn":        "nqn.2011-06.com.truenas:uuid:custom",
		"ana":            true,
		"kernel":         true,
		"rdma":           true,
		"xport_referral": false,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if len(p) != len(want) {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(want))
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// NVMeTGlobalResource.Delete calls only this pure function.
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
	if detail != "NVMe-oF global configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestNVMeTGlobalDataSourceModel_MatchesSchema verifies that every tfsdk tag
// on NVMeTGlobalDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestNVMeTGlobalDataSourceModel_MatchesSchema(t *testing.T) {
	d := &NVMeTGlobalDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(NVMeTGlobalDataSourceModel{})
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
		t.Fatalf("NVMeTGlobalDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
