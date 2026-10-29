// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// objectOf builds a top-level schema value whose attributes are all null
// (fill == nil) or all unknown (fill == tftypes.UnknownValue), except the
// attributes named in set.
func objectOf(t *testing.T, typ tftypes.Type, fill any, set map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	obj, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", typ)
	}
	vals := make(map[string]tftypes.Value, len(obj.AttributeTypes))
	for name, at := range obj.AttributeTypes {
		if v, ok := set[name]; ok {
			vals[name] = v
			continue
		}
		vals[name] = tftypes.NewValue(at, fill)
	}
	return tftypes.NewValue(obj, vals)
}

// TestCloudBackupDataSource_ConfigDecodes is the offline guard for B-017: the
// data source config holds only the lookup key, every Computed attribute is
// null, and the model must decode it. A plain-struct nested field cannot hold
// null and failed every read with a Value Conversion Error.
func TestCloudBackupDataSource_ConfigDecodes(t *testing.T) {
	ctx := context.Background()
	d := &CloudBackupDataSource{}
	var sresp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &sresp)

	cfg := tfsdk.Config{
		Schema: sresp.Schema,
		Raw: objectOf(t, sresp.Schema.Type().TerraformType(ctx), nil, map[string]tftypes.Value{
			"description": tftypes.NewValue(tftypes.String, "tf-acc"),
		}),
	}
	var m CloudBackupDataSourceModel
	if diags := cfg.Get(ctx, &m); diags.HasError() {
		t.Fatalf("data source config did not decode: %v", diags)
	}
}

// TestCloudBackupResource_UnknownPlanDecodes is the offline guard for B-016:
// on create every omitted Optional+Computed attribute (schedule included) is
// unknown in the plan, and the model must decode it.
func TestCloudBackupResource_UnknownPlanDecodes(t *testing.T) {
	ctx := context.Background()
	r := &CloudBackupResource{}
	var sresp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sresp)

	plan := tfsdk.Plan{
		Schema: sresp.Schema,
		Raw:    objectOf(t, sresp.Schema.Type().TerraformType(ctx), tftypes.UnknownValue, nil),
	}
	var m CloudBackupModel
	if diags := plan.Get(ctx, &m); diags.HasError() {
		t.Fatalf("all-unknown plan did not decode: %v", diags)
	}
	if !m.Schedule.IsUnknown() {
		t.Errorf("schedule = %v, want unknown", m.Schedule)
	}
}
