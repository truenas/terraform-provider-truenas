// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestDatasetPlanFollowsPool(t *testing.T) {
	ctx := context.Background()
	unknown, empty := types.StringUnknown(), types.StringNull()
	oldPool, newPool := types.StringValue("old"), types.StringValue("new")
	oldDataset := types.StringValue("old/ix-apps")
	cases := []struct {
		name                      string
		previousPool, plannedPool types.String
		previousDataset, expected types.String
	}{
		{"configure", empty, newPool, empty, unknown},
		{"switch pool", oldPool, newPool, oldDataset, unknown},
		{"unknown pool", oldPool, unknown, oldDataset, unknown},
		{"clear pool", oldPool, empty, oldDataset, unknown},
		{"unchanged pool", oldPool, oldPool, oldDataset, oldDataset},
		{"still unconfigured", empty, empty, empty, empty},
	}
	s := resourceSchema()
	testSchema := schema.Schema{Attributes: map[string]schema.Attribute{
		"pool": s.Attributes["pool"], "dataset": s.Attributes["dataset"],
	}}
	objectType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"pool": tftypes.String, "dataset": tftypes.String,
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := func(v types.String) tftypes.Value {
				t.Helper()
				result, err := v.ToTerraformValue(ctx)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			state := tfsdk.State{Schema: testSchema, Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
				"pool": value(tc.previousPool), "dataset": value(tc.previousDataset),
			})}
			plan := tfsdk.Plan{Schema: testSchema, Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
				"pool": value(tc.plannedPool), "dataset": value(tc.previousDataset),
			})}
			response := resource.ModifyPlanResponse{Plan: plan}
			if modifier, ok := NewResource().(resource.ResourceWithModifyPlan); ok {
				modifier.ModifyPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &response)
			}
			var actual types.String
			response.Diagnostics.Append(response.Plan.GetAttribute(ctx, path.Root("dataset"), &actual)...)
			if response.Diagnostics.HasError() {
				t.Fatalf("plan modification failed: %v", response.Diagnostics)
			}
			if !actual.Equal(tc.expected) {
				t.Fatalf("dataset plan = %s, want %s", actual, tc.expected)
			}
		})
	}
}
