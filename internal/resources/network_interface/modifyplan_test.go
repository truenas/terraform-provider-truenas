// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func lacpLAG(t *testing.T, ctx context.Context) NetworkInterfaceModel {
	m := baseModel(t, ctx, "LINK_AGGREGATION")
	m.ID = types.StringValue("bond0")
	m.Name = types.StringValue("bond0")
	m.LagProtocol = types.StringValue("LACP")
	m.LagPorts = stringList(t, ctx, []string{"enp1s0f0"})
	m.LacpduRate = types.StringValue("SLOW")
	m.XmitHashPolicy = types.StringValue("LAYER2+3")
	m.EnableLearning = types.BoolNull()
	m.VlanPCP = types.Int64Null()
	m.STP = types.BoolNull()
	return m
}

func runModifyPlan(t *testing.T, state, plan, config NetworkInterfaceModel) NetworkInterfaceModel {
	t.Helper()
	ctx := context.Background()
	sch := resourceSchema()
	set := func(m NetworkInterfaceModel) tfsdk.Plan {
		p := tfsdk.Plan{Schema: sch}
		if d := p.Set(ctx, &m); d.HasError() {
			t.Fatalf("set: %v", d)
		}
		return p
	}
	st := set(state)
	pl := set(plan)
	cf := set(config)
	resp := resource.ModifyPlanResponse{Plan: pl}
	(&NetworkInterfaceResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{
		State:  tfsdk.State{Schema: sch, Raw: st.Raw},
		Plan:   pl,
		Config: tfsdk.Config{Schema: sch, Raw: cf.Raw},
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ModifyPlan: %v", resp.Diagnostics)
	}
	var out NetworkInterfaceModel
	if d := resp.Plan.Get(ctx, &out); d.HasError() {
		t.Fatalf("get: %v", d)
	}
	return out
}

// TestModifyPlan_LagProtocolChangeUnknownsLACPFields guards B-065: LACP ->
// FAILOVER with lacpdu_rate/xmit_hash_policy unset in config must not plan
// the LACP values.
func TestModifyPlan_LagProtocolChangeUnknownsLACPFields(t *testing.T) {
	ctx := context.Background()
	state := lacpLAG(t, ctx)
	plan := lacpLAG(t, ctx) // UseStateForUnknown carried SLOW / LAYER2+3
	plan.LagProtocol = types.StringValue("FAILOVER")
	config := plan
	config.LacpduRate = types.StringNull()
	config.XmitHashPolicy = types.StringNull()
	out := runModifyPlan(t, state, plan, config)
	if !out.LacpduRate.IsUnknown() || !out.XmitHashPolicy.IsUnknown() {
		t.Errorf("lacpdu_rate=%v xmit_hash_policy=%v, want both unknown", out.LacpduRate, out.XmitHashPolicy)
	}
}

// TestModifyPlan_ConfiguredLACPFieldsKept: values set in config stay, and a
// plan without a protocol change is untouched.
func TestModifyPlan_ConfiguredLACPFieldsKept(t *testing.T) {
	ctx := context.Background()
	state := lacpLAG(t, ctx)
	plan := lacpLAG(t, ctx)
	plan.LagProtocol = types.StringValue("LOADBALANCE")
	plan.LacpduRate = types.StringNull()
	config := plan // xmit_hash_policy configured, lacpdu_rate not
	config.LacpduRate = types.StringNull()
	out := runModifyPlan(t, state, plan, config)
	if out.XmitHashPolicy.ValueString() != "LAYER2+3" {
		t.Errorf("configured xmit_hash_policy changed to %v", out.XmitHashPolicy)
	}
	same := runModifyPlan(t, state, state, state)
	if same.LacpduRate.ValueString() != "SLOW" {
		t.Errorf("no-op plan changed lacpdu_rate to %v", same.LacpduRate)
	}
}

// TestResponseToModel_PhysicalSTPNull guards B-158: a PHYSICAL interface
// reports no stp; it must read as null so a generated import config passes
// ValidateConfig.
func TestResponseToModel_PhysicalSTPNull(t *testing.T) {
	ctx := context.Background()
	var m NetworkInterfaceModel
	responseToModel(ctx, &interfaceAPI{Name: "enp1s0f0", Type: "PHYSICAL"}, &m)
	if !m.STP.IsNull() {
		t.Errorf("stp = %v, want null for PHYSICAL", m.STP)
	}
	if bad := physicalDisallowedFieldsSet(&m); len(bad) > 0 {
		t.Errorf("read-back PHYSICAL model trips ValidateConfig: %v", bad)
	}
}
