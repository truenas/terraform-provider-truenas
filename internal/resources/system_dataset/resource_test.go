// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSystemDatasetSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestSystemDatasetSchema_IDIsComputed(t *testing.T) {
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

// TestSystemDatasetSchema_PoolOptionalComputed verifies that pool is
// Optional+Computed with a UseStateForUnknown plan modifier.
func TestSystemDatasetSchema_PoolOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["pool"]
	if !ok {
		t.Fatal("schema missing 'pool' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'pool' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'pool' should be Optional+Computed")
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("'pool' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestSystemDatasetSchema_PoolExcludeOptionalOnly verifies that
// pool_exclude is Optional ONLY (never Computed): it is a write-only
// migration argument that TrueNAS never returns on systemdataset.config.
func TestSystemDatasetSchema_PoolExcludeOptionalOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["pool_exclude"]
	if !ok {
		t.Fatal("schema missing 'pool_exclude' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'pool_exclude' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() {
		t.Error("'pool_exclude' should be Optional")
	}
	if strAttr.IsComputed() {
		t.Error("'pool_exclude' should NOT be Computed (write-only)")
	}
	if strAttr.IsRequired() {
		t.Error("'pool_exclude' should NOT be Required")
	}
}

// TestSystemDatasetSchema_ComputedOnlyQuartet verifies that basename, path,
// and pool_set are Computed-only with NO plan modifiers (they change
// whenever pool changes, so UseStateForUnknown would be wrong), while uuid
// is Computed-only WITH UseStateForUnknown.
func TestSystemDatasetSchema_ComputedOnlyQuartet(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"basename", "path"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsComputed() || strAttr.IsOptional() {
			t.Errorf("%q should be Computed-only", name)
		}
		if len(strAttr.PlanModifiers) != 0 {
			t.Errorf("%q should have NO plan modifiers, got %d", name, len(strAttr.PlanModifiers))
		}
	}

	uuidAttr, ok := s.Attributes["uuid"]
	if !ok {
		t.Fatal("schema missing 'uuid' attribute")
	}
	uuidStr, ok := uuidAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'uuid' attribute is %T, want schema.StringAttribute", uuidAttr)
	}
	if !uuidStr.IsComputed() || uuidStr.IsOptional() {
		t.Error("'uuid' should be Computed-only")
	}
	if len(uuidStr.PlanModifiers) == 0 {
		t.Error("'uuid' should have plan modifiers (UseStateForUnknown)")
	}

	poolSetAttr, ok := s.Attributes["pool_set"]
	if !ok {
		t.Fatal("schema missing 'pool_set' attribute")
	}
	poolSetBool, ok := poolSetAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'pool_set' attribute is %T, want schema.BoolAttribute", poolSetAttr)
	}
	if !poolSetBool.IsComputed() || poolSetBool.IsOptional() {
		t.Error("'pool_set' should be Computed-only")
	}
	if len(poolSetBool.PlanModifiers) != 0 {
		t.Errorf("'pool_set' should have NO plan modifiers, got %d", len(poolSetBool.PlanModifiers))
	}
}

// TestResponseToModel_MapsFields verifies that the API response maps
// through to the Terraform model unchanged, and that PoolExclude (write-
// only) is left untouched.
func TestResponseToModel_MapsFields(t *testing.T) {
	api := &systemDatasetAPI{
		ID:       1,
		Basename: "tank/.system",
		Path:     "/var/db/system",
		Pool:     "tank",
		PoolSet:  true,
		UUID:     "ae32c386e13840b2bf9c0083275e7941",
	}

	m := &SystemDatasetModel{PoolExclude: types.StringValue("boot-pool")}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != systemDatasetResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), systemDatasetResourceID)
	}
	if m.Pool.ValueString() != api.Pool {
		t.Errorf("Pool = %q, want %q", m.Pool.ValueString(), api.Pool)
	}
	if m.Basename.ValueString() != api.Basename {
		t.Errorf("Basename = %q, want %q", m.Basename.ValueString(), api.Basename)
	}
	if m.Path.ValueString() != api.Path {
		t.Errorf("Path = %q, want %q", m.Path.ValueString(), api.Path)
	}
	if m.UUID.ValueString() != api.UUID {
		t.Errorf("UUID = %q, want %q", m.UUID.ValueString(), api.UUID)
	}
	if m.PoolSet.ValueBool() != api.PoolSet {
		t.Errorf("PoolSet = %v, want %v", m.PoolSet.ValueBool(), api.PoolSet)
	}
	// PoolExclude must be untouched by responseToModel (write-only, never
	// decoded from a response).
	if m.PoolExclude.ValueString() != "boot-pool" {
		t.Errorf("PoolExclude = %q, want unchanged %q", m.PoolExclude.ValueString(), "boot-pool")
	}
}

// TestSystemDatasetAPI_NoPoolExcludeField verifies by construction that
// systemDatasetAPI has no pool_exclude field at all: this is a compile-time
// guarantee that pool_exclude can never be decoded from a
// systemdataset.config response.
func TestSystemDatasetAPI_NoPoolExcludeField(t *testing.T) {
	typ := reflect.TypeOf(systemDatasetAPI{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Tag.Get("json") == "pool_exclude" {
			t.Fatal("systemDatasetAPI must not have a pool_exclude field (write-only, never returned by the API)")
		}
	}
}

// TestUpdatePayload_PoolGuarded verifies that pool is omitted from the
// update payload when null/unknown, and included when known.
func TestUpdatePayload_PoolGuarded(t *testing.T) {
	m := &SystemDatasetModel{Pool: types.StringNull(), PoolExclude: types.StringNull()}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["pool"]; ok {
		t.Error("expected 'pool' to be omitted when null")
	}

	m = &SystemDatasetModel{Pool: types.StringUnknown(), PoolExclude: types.StringNull()}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["pool"]; ok {
		t.Error("expected 'pool' to be omitted when unknown")
	}

	m = &SystemDatasetModel{Pool: types.StringValue("tank"), PoolExclude: types.StringNull()}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["pool"]; !ok || v != "tank" {
		t.Errorf("payload[pool] = %v (present=%v), want tank", v, ok)
	}
}

// TestUpdatePayload_PoolExcludeOnlyWhenSet verifies that pool_exclude is
// omitted from the update payload unless explicitly set.
func TestUpdatePayload_PoolExcludeOnlyWhenSet(t *testing.T) {
	m := &SystemDatasetModel{Pool: types.StringNull(), PoolExclude: types.StringNull()}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["pool_exclude"]; ok {
		t.Error("expected 'pool_exclude' to be omitted when null")
	}

	m = &SystemDatasetModel{Pool: types.StringNull(), PoolExclude: types.StringUnknown()}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["pool_exclude"]; ok {
		t.Error("expected 'pool_exclude' to be omitted when unknown")
	}

	m = &SystemDatasetModel{Pool: types.StringNull(), PoolExclude: types.StringValue("boot-pool")}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["pool_exclude"]; !ok || v != "boot-pool" {
		t.Errorf("payload[pool_exclude] = %v (present=%v), want boot-pool", v, ok)
	}
}

// TestUpdatePayload_ComputedOnlyQuartetNeverSent verifies that basename,
// path, uuid, and pool_set never appear in the update payload. They have no
// corresponding updatePayload field at all, so this documents the contract
// by construction: attempting to reference them would be a compile error.
func TestUpdatePayload_ComputedOnlyQuartetNeverSent(t *testing.T) {
	m := &SystemDatasetModel{
		Pool:        types.StringValue("tank"),
		PoolExclude: types.StringValue("boot-pool"),
		Basename:    types.StringValue("tank/.system"),
		Path:        types.StringValue("/var/db/system"),
		UUID:        types.StringValue("ae32c386e13840b2bf9c0083275e7941"),
		PoolSet:     types.BoolValue(true),
	}

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	for _, key := range []string{"basename", "path", "uuid", "pool_set"} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to never be present in the update payload", key)
		}
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// SystemDatasetResource.Delete calls only this pure function.
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
	if detail != "System dataset configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestUpdate_UsesCallJob verifies, by parsing this package's own source,
// that Update calls through applyUpdate (which wraps client.CallJob) rather
// than a plain synchronous client.Call for systemdataset.update. Moving the
// system dataset between pools is a long-running operation on TrueNAS, so
// the update path MUST go through job polling.
func TestUpdate_UsesCallJob(t *testing.T) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "resource.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse resource.go: %v", err)
	}

	var foundCallJob, foundBareCallForUpdate bool
	ast.Inspect(astFile, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "CallJob":
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Value == `"systemdataset.update"` {
					foundCallJob = true
				}
			}
		case "Call":
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Value == `"systemdataset.update"` {
					foundBareCallForUpdate = true
				}
			}
		}
		return true
	})

	if !foundCallJob {
		t.Error("expected resource.go to call client.CallJob with \"systemdataset.update\" (long-running job)")
	}
	if foundBareCallForUpdate {
		t.Error("systemdataset.update must go through CallJob, not a plain synchronous Call")
	}
}

// TestSystemDatasetDataSourceModel_MatchesSchema verifies that every tfsdk
// tag on SystemDatasetDataSourceModel has a corresponding attribute in the
// datasource schema, and vice versa. terraform-plugin-framework requires an
// exact field/attribute match: any mismatch causes every datasource Read to
// fail with "Struct defines fields not found in object: ...".
func TestSystemDatasetDataSourceModel_MatchesSchema(t *testing.T) {
	d := &SystemDatasetDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(SystemDatasetDataSourceModel{})
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
		t.Fatalf("SystemDatasetDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
