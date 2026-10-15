// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestDatasetPlanAllowsPoolChanges(t *testing.T) {
	for _, previous := range []types.String{types.StringNull(), types.StringValue("old/ix-apps")} {
		t.Run(previous.String(), func(t *testing.T) {
			// An imported singleton already exists even when Docker is unconfigured.
			request := planmodifier.StringRequest{
				State:       tfsdk.State{Raw: tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}, map[string]tftypes.Value{})},
				StateValue:  previous,
				ConfigValue: types.StringNull(),
				PlanValue:   types.StringUnknown(),
			}
			response := planmodifier.StringResponse{PlanValue: request.PlanValue}
			attribute := resourceSchema().Attributes["dataset"].(schema.StringAttribute)
			for _, modifier := range attribute.PlanModifiers {
				modifier.PlanModifyString(context.Background(), request, &response)
				request.PlanValue = response.PlanValue
			}
			if response.Diagnostics.HasError() {
				t.Fatalf("plan modification failed: %v", response.Diagnostics)
			}
			if !response.PlanValue.IsUnknown() {
				t.Fatalf("dataset must remain unknown when the pool changes, got %s", response.PlanValue)
			}
		})
	}
}
