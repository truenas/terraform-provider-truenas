// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func staticChannel() IPMILanModel {
	return IPMILanModel{
		ID:                types.Int64Value(1),
		Channel:           types.Int64Value(1),
		DHCP:              types.BoolValue(false),
		IPAddress:         types.StringValue("192.0.2.103"),
		Netmask:           types.StringValue("255.255.255.0"),
		Gateway:           types.StringValue("192.0.2.1"),
		Vlan:              types.Int64Null(),
		VlanPriority:      types.Int64Value(0),
		MACAddress:        types.StringValue("aa:bb:cc:dd:ee:ff"),
		Password:          types.StringNull(),
		ApplyRemote:       types.BoolNull(),
		PasswordWOVersion: types.Int64Null(),
	}
}

func modifyPlan(t *testing.T, state, plan IPMILanModel) IPMILanModel {
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
	resp := resource.ModifyPlanResponse{Plan: pl}
	(&IPMILanResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: st, Plan: pl, Config: tfsdk.Config{Schema: sch, Raw: pl.Raw}}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ModifyPlan: %v", resp.Diagnostics)
	}
	var out IPMILanModel
	if d := resp.Plan.Get(ctx, &out); d.HasError() {
		t.Fatalf("get plan: %v", d)
	}
	return out
}

// TestModifyPlan_StaticToDHCPUnknownsAddress guards B-129: switching a
// static channel to DHCP must not carry the old static address into the plan.
func TestModifyPlan_StaticToDHCPUnknownsAddress(t *testing.T) {
	state := staticChannel()
	plan := staticChannel() // UseStateForUnknown copied the static values
	plan.DHCP = types.BoolValue(true)
	out := modifyPlan(t, state, plan)
	if !out.IPAddress.IsUnknown() || !out.Netmask.IsUnknown() || !out.Gateway.IsUnknown() {
		t.Errorf("address fields not unknown after static->DHCP: %v %v %v", out.IPAddress, out.Netmask, out.Gateway)
	}
}

// TestModifyPlan_StaticNoChangeKeepsAddress: a static channel and a no-op
// plan keep their values.
func TestModifyPlan_StaticNoChangeKeepsAddress(t *testing.T) {
	out := modifyPlan(t, staticChannel(), staticChannel())
	if out.IPAddress.ValueString() != "192.0.2.103" {
		t.Errorf("ipaddress = %v", out.IPAddress)
	}
	plan := staticChannel()
	plan.Vlan = types.Int64Value(10)
	out = modifyPlan(t, staticChannel(), plan)
	if out.IPAddress.ValueString() != "192.0.2.103" {
		t.Errorf("static update changed ipaddress plan to %v", out.IPAddress)
	}
}

// TestValidateConfig_DHCPWithStaticAddress rejects ipaddress with dhcp=true.
func TestValidateConfig_DHCPWithStaticAddress(t *testing.T) {
	ctx := context.Background()
	sch := resourceSchema()
	m := staticChannel()
	m.DHCP = types.BoolValue(true)
	m.Netmask = types.StringNull()
	m.Gateway = types.StringNull()
	m.ID = types.Int64Unknown()
	m.MACAddress = types.StringUnknown()
	pl := tfsdk.Plan{Schema: sch}
	if d := pl.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.ValidateConfigResponse{}
	(&IPMILanResource{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: sch, Raw: pl.Raw}}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Error("ipaddress with dhcp = true was accepted")
	}
}
