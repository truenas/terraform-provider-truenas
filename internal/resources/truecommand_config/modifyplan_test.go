// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func tcModel(enabled bool, apiKey types.String, woVersion types.Int64) TrueCommandConfigModel {
	return TrueCommandConfigModel{
		ID:              types.StringValue(trueCommandConfigResourceID),
		Enabled:         types.BoolValue(enabled),
		APIKey:          apiKey,
		APIKeyWO:        types.StringNull(),
		APIKeyWOVersion: woVersion,
		Status:          types.StringValue("DISABLED"),
		StatusReason:    types.StringValue("Truecommand service is disabled."),
		RemoteURL:       types.StringNull(),
		RemoteIPAddress: types.StringNull(),
	}
}

// runModifyPlan runs ModifyPlan with the given prior state (nil = create),
// plan and config, and returns the modified plan.
func runModifyPlan(t *testing.T, state *TrueCommandConfigModel, plan, config TrueCommandConfigModel) TrueCommandConfigModel {
	t.Helper()
	ctx := context.Background()
	sch := resourceSchema()
	st := tfsdk.State{Schema: sch}
	if state != nil {
		if d := st.Set(ctx, state); d.HasError() {
			t.Fatalf("state: %v", d)
		}
	} else {
		st.RemoveResource(ctx)
	}
	pl := tfsdk.Plan{Schema: sch}
	if d := pl.Set(ctx, &plan); d.HasError() {
		t.Fatalf("plan: %v", d)
	}
	cfgPlan := tfsdk.Plan{Schema: sch}
	if d := cfgPlan.Set(ctx, &config); d.HasError() {
		t.Fatalf("config: %v", d)
	}
	cfg := tfsdk.Config{Schema: sch, Raw: cfgPlan.Raw}
	resp := resource.ModifyPlanResponse{Plan: pl}
	(&TrueCommandConfigResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{State: st, Plan: pl, Config: cfg}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("ModifyPlan: %v", resp.Diagnostics)
	}
	var out TrueCommandConfigModel
	if d := resp.Plan.Get(ctx, &out); d.HasError() {
		t.Fatalf("get plan: %v", d)
	}
	return out
}

// TestModifyPlan_MigrateToWriteOnlyNullsAPIKey guards B-062: when the config
// switches from api_key to api_key_wo (api_key_wo_version set, api_key
// removed), UseStateForUnknown carries the old key into the plan; ModifyPlan
// must plan it null, which is what the apply stores.
func TestModifyPlan_MigrateToWriteOnlyNullsAPIKey(t *testing.T) {
	state := tcModel(false, types.StringValue("abcd1234abcd1234"), types.Int64Null())
	plan := tcModel(false, types.StringValue("abcd1234abcd1234"), types.Int64Value(1))
	config := tcModel(false, types.StringNull(), types.Int64Value(1))
	out := runModifyPlan(t, &state, plan, config)
	if !out.APIKey.IsNull() {
		t.Errorf("api_key planned %v, want null on the write-only path", out.APIKey)
	}
}

// TestModifyPlan_EnabledChangeUnknownsStatus guards B-063: enabling (or
// disabling) makes the server recompute status/status_reason/remote_*.
func TestModifyPlan_EnabledChangeUnknownsStatus(t *testing.T) {
	key := types.StringValue("abcd1234abcd1234")
	state := tcModel(false, key, types.Int64Null())
	plan := tcModel(true, key, types.Int64Null())
	out := runModifyPlan(t, &state, plan, plan)
	for name, v := range map[string]types.String{
		"status": out.Status, "status_reason": out.StatusReason,
		"remote_url": out.RemoteURL, "remote_ip_address": out.RemoteIPAddress,
	} {
		if !v.IsUnknown() {
			t.Errorf("%s = %v, want unknown after an enabled change", name, v)
		}
	}
}

// TestModifyPlan_NoChangeKeepsStatus: a no-op plan keeps the status values,
// so the plan stays empty.
func TestModifyPlan_NoChangeKeepsStatus(t *testing.T) {
	key := types.StringValue("abcd1234abcd1234")
	state := tcModel(false, key, types.Int64Null())
	out := runModifyPlan(t, &state, state, state)
	if !out.Status.Equal(state.Status) || !out.StatusReason.Equal(state.StatusReason) {
		t.Errorf("status changed on a no-op plan: %v / %v", out.Status, out.StatusReason)
	}
}

// TestResponseToModel_RedactedAPIKeyKeepsPrior guards B-029: a
// READONLY_ADMIN session gets api_key = "********"; the resource keeps the
// prior value and the data source reports null, never the mask.
func TestResponseToModel_RedactedAPIKeyKeepsPrior(t *testing.T) {
	mask := redactedAPIKey
	api := &truecommandConfigAPI{APIKey: &mask, Status: "DISABLED"}

	m := TrueCommandConfigModel{APIKey: types.StringValue("abcd1234abcd1234"), APIKeyWOVersion: types.Int64Null()}
	responseToModel(api, &m)
	if m.APIKey.ValueString() != "abcd1234abcd1234" {
		t.Errorf("resource api_key = %v, want the prior value kept", m.APIKey)
	}

	imported := TrueCommandConfigModel{APIKey: types.StringNull(), APIKeyWOVersion: types.Int64Null()}
	responseToModel(api, &imported)
	if !imported.APIKey.IsNull() {
		t.Errorf("imported api_key = %v, want null (never the mask)", imported.APIKey)
	}

	var ds TrueCommandConfigDataSourceModel
	responseToDataSourceModel(api, &ds)
	if !ds.APIKey.IsNull() {
		t.Errorf("data source api_key = %v, want null", ds.APIKey)
	}
}
