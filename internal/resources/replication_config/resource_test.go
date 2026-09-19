// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestReplicationConfigSchema_IDIsComputed verifies that "id" is a
// Computed-only StringAttribute with UseStateForUnknown, since it's a fixed
// singleton value never supplied by the user.
func TestReplicationConfigSchema_IDIsComputed(t *testing.T) {
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

// TestReplicationConfigSchema_MaxParallelOptionalComputed verifies that
// max_parallel_replication_tasks is Optional+Computed with a
// UseStateForUnknown plan modifier.
func TestReplicationConfigSchema_MaxParallelOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["max_parallel_replication_tasks"]
	if !ok {
		t.Fatal("schema missing 'max_parallel_replication_tasks' attribute")
	}
	intAttr, ok := attr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'max_parallel_replication_tasks' attribute is %T, want schema.Int64Attribute", attr)
	}
	if !intAttr.IsOptional() || !intAttr.IsComputed() {
		t.Error("'max_parallel_replication_tasks' should be Optional+Computed")
	}
	if len(intAttr.PlanModifiers) == 0 {
		t.Error("'max_parallel_replication_tasks' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestResponseToModel_NilMapsToZeroSentinel verifies that a nil
// max_parallel_replication_tasks on the wire maps to 0 (the "unlimited"
// sentinel), and that a non-nil value maps through unchanged.
func TestResponseToModel_NilMapsToZeroSentinel(t *testing.T) {
	api := &replicationConfigAPI{ID: 1, MaxParallelReplicationTasks: nil}

	m := &ReplicationConfigModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.MaxParallelReplicationTasks.ValueInt64() != 0 {
		t.Errorf("MaxParallelReplicationTasks = %v, want 0", m.MaxParallelReplicationTasks)
	}
	if m.ID.ValueString() != replicationConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), replicationConfigResourceID)
	}

	v := int64(5)
	api.MaxParallelReplicationTasks = &v
	m2 := &ReplicationConfigModel{}
	diags = responseToModel(context.Background(), api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m2.MaxParallelReplicationTasks.IsNull() || m2.MaxParallelReplicationTasks.ValueInt64() != 5 {
		t.Errorf("MaxParallelReplicationTasks = %v, want 5", m2.MaxParallelReplicationTasks)
	}
}

// TestResponseToDataSourceModel_NilMapsToZeroSentinel mirrors
// TestResponseToModel_NilMapsToZeroSentinel for the datasource model.
func TestResponseToDataSourceModel_NilMapsToZeroSentinel(t *testing.T) {
	api := &replicationConfigAPI{ID: 1, MaxParallelReplicationTasks: nil}

	m := &ReplicationConfigDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.MaxParallelReplicationTasks.ValueInt64() != 0 {
		t.Errorf("MaxParallelReplicationTasks = %v, want 0", m.MaxParallelReplicationTasks)
	}

	v := int64(3)
	api.MaxParallelReplicationTasks = &v
	m2 := &ReplicationConfigDataSourceModel{}
	diags = responseToDataSourceModel(context.Background(), api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m2.MaxParallelReplicationTasks.IsNull() || m2.MaxParallelReplicationTasks.ValueInt64() != 3 {
		t.Errorf("MaxParallelReplicationTasks = %v, want 3", m2.MaxParallelReplicationTasks)
	}
}

// TestUpdatePayload_ThreeWay verifies the three-way behavior for
// max_parallel_replication_tasks: null/unknown omits the key entirely, an
// explicit 0 sends JSON nil (meaning unlimited), and any other value N
// sends N.
func TestUpdatePayload_ThreeWay(t *testing.T) {
	// null -> omitted
	m := &ReplicationConfigModel{MaxParallelReplicationTasks: types.Int64Null()}
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["max_parallel_replication_tasks"]; ok {
		t.Error("expected 'max_parallel_replication_tasks' to be omitted when null")
	}

	// unknown -> omitted
	m = &ReplicationConfigModel{MaxParallelReplicationTasks: types.Int64Unknown()}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if _, ok := p["max_parallel_replication_tasks"]; ok {
		t.Error("expected 'max_parallel_replication_tasks' to be omitted when unknown")
	}

	// explicit 0 -> nil (unlimited)
	m = &ReplicationConfigModel{MaxParallelReplicationTasks: types.Int64Value(0)}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	v, ok := p["max_parallel_replication_tasks"]
	if !ok {
		t.Fatal("expected 'max_parallel_replication_tasks' to be present when explicitly 0")
	}
	if v != nil {
		t.Errorf("max_parallel_replication_tasks = %v, want nil", v)
	}

	// N -> N
	m = &ReplicationConfigModel{MaxParallelReplicationTasks: types.Int64Value(5)}
	p, diags = m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if v, ok := p["max_parallel_replication_tasks"]; !ok || v != int64(5) {
		t.Errorf("max_parallel_replication_tasks = %v (present=%v), want 5", v, ok)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// ReplicationConfigResource.Delete calls only this pure function.
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
	if detail != "Replication configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}

// TestReplicationConfigDataSourceModel_MatchesSchema verifies that every
// tfsdk tag on ReplicationConfigDataSourceModel has a corresponding
// attribute in the datasource schema, and vice versa.
// terraform-plugin-framework requires an exact field/attribute match: any
// mismatch causes every datasource Read to fail with "Struct defines
// fields not found in object: ...".
func TestReplicationConfigDataSourceModel_MatchesSchema(t *testing.T) {
	d := &ReplicationConfigDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(ReplicationConfigDataSourceModel{})
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
		t.Fatalf("ReplicationConfigDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}
