// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// enrolledModel is a fully known state for an enrolled 26.0+ box.
func enrolledModel() TnConnectConfigModel {
	return TnConnectConfigModel{
		ID:                           types.StringValue(tnConnectConfigResourceID),
		Enabled:                      types.BoolValue(true),
		Status:                       types.StringValue("CONFIGURED"),
		StatusReason:                 types.StringValue("TrueNAS Connect is configured"),
		Certificate:                  types.Int64Value(7),
		AccountServiceBaseURL:        types.StringValue("https://account.example/"),
		LecaServiceBaseURL:           types.StringValue("https://dns.example/"),
		TNCBaseURL:                   types.StringValue("https://tnc.example/"),
		HeartbeatURL:                 types.StringValue("https://heartbeat.example/"),
		RegistrationDetails:          types.StringValue(`{"account_id":"x"}`),
		Tier:                         types.StringValue("FOUNDATION"),
		LastHeartbeatFailureDatetime: types.StringNull(),
		IPs:                          types.ListNull(types.StringType),
		Interfaces:                   types.ListNull(types.StringType),
		InterfacesIPs:                types.ListNull(types.StringType),
		UseAllInterfaces:             types.BoolNull(),
	}
}

func runModifyPlan(t *testing.T, state, plan TnConnectConfigModel) TnConnectConfigModel {
	t.Helper()
	ctx := context.Background()
	sch := resourceSchema()
	st := tfsdk.State{Schema: sch}
	if d := st.Set(ctx, &state); d.HasError() {
		t.Fatalf("state: %v", d)
	}
	pl := tfsdk.Plan{Schema: sch}
	if d := pl.Set(ctx, &plan); d.HasError() {
		t.Fatalf("plan: %v", d)
	}
	cfg := tfsdk.Config{Schema: sch, Raw: pl.Raw}
	resp := resource.ModifyPlanResponse{Plan: pl}
	(&TnConnectConfigResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: st, Plan: pl, Config: cfg}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ModifyPlan: %v", resp.Diagnostics)
	}
	var out TnConnectConfigModel
	if d := resp.Plan.Get(ctx, &out); d.HasError() {
		t.Fatalf("get plan: %v", d)
	}
	return out
}

// TestModifyPlan_EnabledChangeUnknownsDerived guards B-064: unenrolling
// (enabled true -> false) must plan the enrollment-derived attributes
// unknown, not carry the CONFIGURED status/certificate/tier forward.
func TestModifyPlan_EnabledChangeUnknownsDerived(t *testing.T) {
	state := enrolledModel()
	plan := enrolledModel()
	plan.Enabled = types.BoolValue(false)
	out := runModifyPlan(t, state, plan)
	for name, v := range map[string]interface{ IsUnknown() bool }{
		"status":                          out.Status,
		"status_reason":                   out.StatusReason,
		"certificate":                     out.Certificate,
		"registration_details":            out.RegistrationDetails,
		"tier":                            out.Tier,
		"last_heartbeat_failure_datetime": out.LastHeartbeatFailureDatetime,
		"ips":                             out.IPs,
		"use_all_interfaces":              out.UseAllInterfaces,
	} {
		if !v.IsUnknown() {
			t.Errorf("%s not planned unknown after an enabled change", name)
		}
	}
	if out.HeartbeatURL.IsUnknown() || out.ID.IsUnknown() {
		t.Errorf("heartbeat_url/id must keep their state values")
	}
}

// TestModifyPlan_NoEnabledChangeKeepsState: a plan that does not change
// enabled must leave every computed attribute at its state value, so a
// no-op plan stays empty.
func TestModifyPlan_NoEnabledChangeKeepsState(t *testing.T) {
	state := enrolledModel()
	out := runModifyPlan(t, state, enrolledModel())
	if !out.Status.Equal(state.Status) || !out.Certificate.Equal(state.Certificate) || !out.Tier.Equal(state.Tier) {
		t.Errorf("computed attributes changed on a no-op plan: %+v", out)
	}
}
