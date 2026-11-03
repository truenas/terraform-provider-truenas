// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// fakeAPI answers *.query methods from canned rows and records every Call.
type fakeAPI struct {
	rows     map[string]any   // method -> rows (marshalled as the response)
	missing  map[string]bool  // method -> "Method does not exist"
	failing  map[string]error // method -> error
	calls    []string
	datasets map[string]string // name -> guid, for pool.dataset.query
}

func (f *fakeAPI) Call(_ context.Context, method string, params ...any) (json.RawMessage, error) {
	f.calls = append(f.calls, fmt.Sprintf("%s %v", method, params))
	if err, ok := f.failing[method]; ok {
		return nil, err
	}
	return json.RawMessage(`true`), nil
}

func (f *fakeAPI) CallRead(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	if f.missing[method] {
		return nil, &client.APIError{Code: -32601, Message: "Method does not exist"}
	}
	if err, ok := f.failing[method]; ok {
		return nil, err
	}
	if method == "pool.dataset.query" {
		filter, _ := params[0].([][]any)
		var out []guidRow
		for name, guid := range f.datasets {
			var hit bool
			switch filter[0][0] {
			case "id":
				hit = filter[0][2] == name
			case "guid.value":
				hit = filter[0][2] == guid
			case "type":
				hit = true
			}
			if hit {
				var r guidRow
				r.Name = name
				r.Guid.Value = guid
				out = append(out, r)
			}
		}
		return json.Marshal(out)
	}
	rows, ok := f.rows[method]
	if !ok {
		rows = []any{}
	}
	return json.Marshal(rows)
}

func TestIsGuid(t *testing.T) {
	for id, want := range map[string]bool{
		"4986242210286407626": true,
		"tank/data":           false,
		"tank":                false,
		"12345":               false, // too short to be a guid, could be a pool name
		"":                    false,
		"0x12345678901234":    false,
	} {
		if got := IsGuid(id); got != want {
			t.Errorf("IsGuid(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestLookupGuidAndNameByGuid(t *testing.T) {
	f := &fakeAPI{datasets: map[string]string{"tank/a": "111", "tank/a/b": "222"}}
	ctx := context.Background()
	if g, err := LookupGuid(ctx, f, "tank/a"); err != nil || g != "111" {
		t.Fatalf("LookupGuid = %q, %v", g, err)
	}
	if _, err := LookupGuid(ctx, f, "tank/none"); !client.IsNotFound(err) {
		t.Fatalf("LookupGuid missing: want not-found, got %v", err)
	}
	if n, err := NameByGuid(ctx, f, "222"); err != nil || n != "tank/a/b" {
		t.Fatalf("NameByGuid = %q, %v", n, err)
	}
	if _, err := NameByGuid(ctx, f, "999"); !client.IsNotFound(err) {
		t.Fatalf("NameByGuid missing: want not-found, got %v", err)
	}
	// ResolveByGuid: by guid when state has one, else by name; gone -> !found.
	if n, g, found, err := ResolveByGuid(ctx, f, types.StringValue("222"), "tank/stale"); err != nil || !found || n != "tank/a/b" || g != "222" {
		t.Fatalf("ResolveByGuid by guid = %q %q %v %v", n, g, found, err)
	}
	if n, g, found, err := ResolveByGuid(ctx, f, types.StringNull(), "tank/a"); err != nil || !found || n != "tank/a" || g != "111" {
		t.Fatalf("ResolveByGuid by name = %q %q %v %v", n, g, found, err)
	}
	if _, _, found, err := ResolveByGuid(ctx, f, types.StringValue("999"), "tank/a"); err != nil || found {
		t.Fatalf("ResolveByGuid gone guid: found=%v err=%v", found, err)
	}
	if _, _, found, err := ResolveByGuid(ctx, f, types.StringNull(), "tank/none"); err != nil || found {
		t.Fatalf("ResolveByGuid gone name: found=%v err=%v", found, err)
	}
}

func blockerRows() map[string]any {
	return map[string]any{
		"sharing.smb.query": []any{
			map[string]any{"id": 1, "name": "docs", "path": "/mnt/tank/a"},
			map[string]any{"id": 2, "name": "sub", "path": "/mnt/tank/a/sub"},
			map[string]any{"id": 3, "name": "other", "path": "/mnt/tank/ab"}, // prefix without separator: not under
		},
		"sharing.nfs.query": []any{map[string]any{"id": 7, "path": "/mnt/tank/other"}},
		"iscsi.extent.query": []any{
			map[string]any{"id": 4, "name": "lun0", "type": "DISK", "disk": "zvol/tank/a/vol", "dataset": "tank/a/vol", "path": "zvol/tank/a/vol"},
			map[string]any{"id": 5, "name": "file", "type": "FILE", "disk": "", "path": "/mnt/tank/a/ext.img"},
			map[string]any{"id": 6, "name": "elsewhere", "type": "DISK", "disk": "zvol/tank/z"},
		},
		"nvmet.namespace.query":   []any{map[string]any{"id": 8, "device_path": "zvol/tank/a/vol", "dataset": "tank/a/vol"}},
		"pool.snapshottask.query": []any{map[string]any{"id": 9, "dataset": "tank/a"}, map[string]any{"id": 10, "dataset": "tank"}},
		"replication.query": []any{
			map[string]any{"id": 11, "name": "repl", "source_datasets": []any{"tank/x", "tank/a/b"}, "target_dataset": "backup/a"},
			map[string]any{"id": 12, "name": "to-here", "source_datasets": []any{"backup/q"}, "target_dataset": "tank/a"},
		},
		"rsynctask.query":    []any{map[string]any{"id": 13, "path": "/mnt/tank/a/"}},
		"cloudsync.query":    []any{map[string]any{"id": 14, "description": "cs", "path": "/mnt/tank/a/photos"}},
		"cloud_backup.query": []any{},
		"vm.device.query": []any{
			map[string]any{"id": 15, "vm": 3, "attributes": map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/a/vol"}},
			map[string]any{"id": 16, "vm": 3, "attributes": map[string]any{"dtype": "NIC"}},
		},
	}
}

func TestRenameBlockers_dataset(t *testing.T) {
	f := &fakeAPI{rows: blockerRows()}
	got, err := RenameBlockers(context.Background(), f, "tank/a", "/mnt/tank/a")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, b := range got {
		lines = append(lines, b.String())
	}
	want := []string{
		`NVMe-oF namespace id 8 (device_path zvol/tank/a/vol)`,
		`SMB share "docs" (path /mnt/tank/a)`,
		`SMB share "sub" (path /mnt/tank/a/sub)`,
		`VM device id 15 (vm 3, path /dev/zvol/tank/a/vol)`,
		`cloud sync task id 14 (path /mnt/tank/a/photos)`,
		`iSCSI extent "file" (path /mnt/tank/a/ext.img)`,
		`iSCSI extent "lun0" (disk zvol/tank/a/vol)`,
		`periodic snapshot task id 9 (dataset tank/a)`,
		`replication task "repl" (source_datasets tank/a/b)`,
		`replication task "to-here" (target_dataset tank/a)`,
		`rsync task id 13 (path /mnt/tank/a/)`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("blockers:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	// The ancestor snapshot task (tank) and the sibling (tank/ab) are not
	// blockers.
	for _, l := range lines {
		if strings.Contains(l, "tank/ab") || strings.Contains(l, "(dataset tank)") {
			t.Errorf("unexpected blocker %s", l)
		}
	}
}

func TestRenameBlockers_zvol(t *testing.T) {
	f := &fakeAPI{rows: blockerRows()}
	got, err := RenameBlockers(context.Background(), f, "tank/a/vol", "")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, b := range got {
		lines = append(lines, b.String())
	}
	want := []string{
		`NVMe-oF namespace id 8 (device_path zvol/tank/a/vol)`,
		`VM device id 15 (vm 3, path /dev/zvol/tank/a/vol)`,
		`iSCSI extent "lun0" (disk zvol/tank/a/vol)`,
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("blockers:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestRenameBlockers_missingMethodSkippedOtherErrorsFail(t *testing.T) {
	f := &fakeAPI{rows: blockerRows(), missing: map[string]bool{"vm.device.query": true, "nvmet.namespace.query": true}}
	got, err := RenameBlockers(context.Background(), f, "tank/a/vol", "")
	if err != nil || len(got) != 1 || got[0].Kind != "iSCSI extent" {
		t.Fatalf("missing methods: got %v, %v", got, err)
	}
	f = &fakeAPI{rows: blockerRows(), failing: map[string]error{"sharing.nfs.query": errors.New("boom")}}
	if _, err := RenameBlockers(context.Background(), f, "tank/a", "/mnt/tank/a"); err == nil || !strings.Contains(err.Error(), "sharing.nfs.query") {
		t.Fatalf("query failure must fail the check, got %v", err)
	}
}

func TestPerformRename(t *testing.T) {
	ctx := context.Background()
	// Refused while in use, nothing renamed.
	f := &fakeAPI{rows: blockerRows(), datasets: map[string]string{"tank/a": "111"}}
	_, diags := PerformRename(ctx, f, "dataset", "111", "tank/a", "tank/b", "/mnt/tank/a", false)
	if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), `SMB share "docs"`) || len(f.calls) != 0 {
		t.Fatalf("in-use rename: diags=%v calls=%v", diags, f.calls)
	}
	// allow_rename_in_use: renamed with force, recursive false.
	f = &fakeAPI{rows: blockerRows(), datasets: map[string]string{"tank/a": "111"}}
	name, diags := PerformRename(ctx, f, "dataset", "111", "tank/a", "tank/b", "/mnt/tank/a", true)
	if diags.HasError() || name != "tank/b" || len(f.calls) != 1 ||
		!strings.Contains(f.calls[0], "pool.dataset.rename [tank/a map[force:true new_name:tank/b recursive:false]]") {
		t.Fatalf("allowed rename: name=%q diags=%v calls=%v", name, diags, f.calls)
	}
	// Not in use: renamed.
	f = &fakeAPI{datasets: map[string]string{"tank/a": "111"}}
	if name, diags := PerformRename(ctx, f, "dataset", "111", "tank/a", "tank/b", "/mnt/tank/a", false); diags.HasError() || name != "tank/b" || len(f.calls) != 1 {
		t.Fatalf("free rename: name=%q diags=%v calls=%v", name, diags, f.calls)
	}
	// Already at the planned name (moved with its parent): no rename call.
	f = &fakeAPI{datasets: map[string]string{"tank/b/child": "222"}}
	if name, diags := PerformRename(ctx, f, "dataset", "222", "tank/a/child", "tank/b/child", "/mnt/tank/a/child", false); diags.HasError() || name != "tank/b/child" || len(f.calls) != 0 {
		t.Fatalf("moved-with-parent: name=%q diags=%v calls=%v", name, diags, f.calls)
	}
	// Renamed out of band to a third name: the rename starts from the current
	// one, and the in-use check uses the current mountpoint.
	f = &fakeAPI{datasets: map[string]string{"tank/c": "111"}, rows: map[string]any{
		"sharing.smb.query": []any{map[string]any{"id": 1, "name": "s", "path": "/mnt/tank/c"}}}}
	if _, diags := PerformRename(ctx, f, "dataset", "111", "tank/a", "tank/b", "/mnt/tank/a", false); !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), "/mnt/tank/c") {
		t.Fatalf("third-name in-use: %v", diags)
	}
	f = &fakeAPI{datasets: map[string]string{"tank/c": "111"}}
	if _, diags := PerformRename(ctx, f, "dataset", "111", "tank/a", "tank/b", "/mnt/tank/a", false); diags.HasError() || !strings.Contains(f.calls[0], "pool.dataset.rename [tank/c ") {
		t.Fatalf("third-name rename: diags=%v calls=%v", diags, f.calls)
	}
	// The rename API failing is reported.
	f = &fakeAPI{datasets: map[string]string{"tank/a": "111"}, failing: map[string]error{"pool.dataset.rename": errors.New("EBUSY")}}
	if _, diags := PerformRename(ctx, f, "zvol", "111", "tank/a", "tank/b", "", false); !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), "EBUSY") {
		t.Fatalf("api failure: %v", diags)
	}
}

func TestResolveImportID(t *testing.T) {
	ctx := context.Background()
	f := &fakeAPI{datasets: map[string]string{"tank/a": "4986242210286407626"}}
	n, g, diags := ResolveImportID(ctx, f, resource.ImportStateRequest{ID: "tank/a"})
	if diags.HasError() || n != "tank/a" || g != "4986242210286407626" {
		t.Fatalf("by name: %q %q %v", n, g, diags)
	}
	n, g, diags = ResolveImportID(ctx, f, resource.ImportStateRequest{ID: "4986242210286407626"})
	if diags.HasError() || n != "tank/a" || g != "4986242210286407626" {
		t.Fatalf("by guid: %q %q %v", n, g, diags)
	}
	if _, _, diags = ResolveImportID(ctx, f, resource.ImportStateRequest{ID: "tank/none"}); !diags.HasError() {
		t.Fatal("missing dataset must fail the import")
	}
	// identity { guid = ... }
	ident := &tfsdk.ResourceIdentity{Schema: IdentitySchema(), Raw: tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{"guid": tftypes.String}},
		map[string]tftypes.Value{"guid": tftypes.NewValue(tftypes.String, "4986242210286407626")})}
	n, g, diags = ResolveImportID(ctx, f, resource.ImportStateRequest{Identity: ident})
	if diags.HasError() || n != "tank/a" || g != "4986242210286407626" {
		t.Fatalf("by identity: %q %q %v", n, g, diags)
	}
}

// planReq builds the state/plan/config for the plan-modifier tests from name
// and mountpoint values (unknown mountpoint = types.StringUnknown()).
func planReq(t *testing.T, stateName, stateMP string, planName string) (tfsdk.State, tfsdk.Plan) {
	t.Helper()
	sch := resourceSchema()
	ctx := context.Background()
	mk := func(name, mp string, unknownMP bool) tftypes.Value {
		vals := map[string]tftypes.Value{}
		for k, a := range sch.Attributes {
			vals[k] = tftypes.NewValue(a.GetType().TerraformType(ctx), nil)
		}
		vals["name"] = tftypes.NewValue(tftypes.String, name)
		if unknownMP {
			vals["mountpoint"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		} else {
			vals["mountpoint"] = tftypes.NewValue(tftypes.String, mp)
		}
		return tftypes.NewValue(sch.Type().TerraformType(ctx), vals)
	}
	state := tfsdk.State{Schema: sch, Raw: mk(stateName, stateMP, false)}
	if stateName == "" {
		state.Raw = tftypes.NewValue(sch.Type().TerraformType(ctx), nil)
	}
	plan := tfsdk.Plan{Schema: sch, Raw: mk(planName, "", true)}
	return state, plan
}

func TestIDFollowsName(t *testing.T) {
	ctx := context.Background()
	_, plan := planReq(t, "tank/a", "/mnt/tank/a", "tank/b")
	var resp planmodifier.StringResponse
	IDFollowsName().PlanModifyString(ctx, planmodifier.StringRequest{Path: path.Root("id"), Plan: plan, PlanValue: types.StringUnknown()}, &resp)
	if resp.PlanValue.ValueString() != "tank/b" {
		t.Errorf("id planned %v, want tank/b", resp.PlanValue)
	}
}

func TestMountpointFollowsName(t *testing.T) {
	ctx := context.Background()
	run := func(stateName, stateMP, planName string) attr.Value {
		state, plan := planReq(t, stateName, stateMP, planName)
		resp := planmodifier.StringResponse{PlanValue: types.StringUnknown()}
		MountpointFollowsName().PlanModifyString(ctx, planmodifier.StringRequest{
			Path: path.Root("mountpoint"), State: state, Plan: plan,
			StateValue: types.StringValue(stateMP), PlanValue: types.StringUnknown(),
		}, &resp)
		return resp.PlanValue
	}
	if v := run("tank/a", "/mnt/tank/a", "tank/a"); v.(types.String).ValueString() != "/mnt/tank/a" {
		t.Errorf("no rename: planned %v, want state value kept", v)
	}
	if v := run("tank/a", "/mnt/tank/a", "tank/b"); v.(types.String).ValueString() != "/mnt/tank/b" {
		t.Errorf("rename: planned %v, want /mnt/tank/b", v)
	}
	if v := run("tank/a", "/custom/mp", "tank/b"); !v.IsUnknown() {
		t.Errorf("rename with a custom mountpoint: planned %v, want unknown", v)
	}
	// Create: no state, stays unknown.
	state, plan := planReq(t, "", "", "tank/b")
	resp := planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	MountpointFollowsName().PlanModifyString(ctx, planmodifier.StringRequest{Path: path.Root("mountpoint"), State: state, Plan: plan, StateValue: types.StringNull(), PlanValue: types.StringUnknown()}, &resp)
	if !resp.PlanValue.IsUnknown() {
		t.Errorf("create: planned %v, want unknown", resp.PlanValue)
	}
}

func TestCheckRenamePlan(t *testing.T) {
	ctx := context.Background()
	check := func(stateName, planName string) (bool, string) {
		state, plan := planReq(t, stateName, "/mnt/"+stateName, planName)
		_, _, changed, diags := CheckRenamePlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, "dataset")
		msg := ""
		if diags.HasError() {
			msg = diags.Errors()[0].Summary()
		}
		return changed, msg
	}
	if changed, msg := check("tank/a", "tank/a"); changed || msg != "" {
		t.Errorf("same name: %v %q", changed, msg)
	}
	if changed, msg := check("tank/a", "tank/b/a"); !changed || msg != "" {
		t.Errorf("rename within pool: %v %q", changed, msg)
	}
	if changed, msg := check("tank/a", "fast/a"); !changed || !strings.Contains(msg, "another pool") {
		t.Errorf("cross-pool: %v %q", changed, msg)
	}
	if changed, msg := check("tank/a", "tank/a/b"); !changed || !strings.Contains(msg, "into itself") {
		t.Errorf("into itself: %v %q", changed, msg)
	}
	// Create (no state) is never a rename.
	state, plan := planReq(t, "", "", "tank/b")
	if _, _, changed, diags := CheckRenamePlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, "dataset"); changed || diags.HasError() {
		t.Errorf("create: changed=%v diags=%v", changed, diags)
	}
}

func TestIdentityUpgraderV0(t *testing.T) {
	ctx := context.Background()
	f := &fakeAPI{datasets: map[string]string{"tank/a": "111"}}
	var c *client.Client // the upgrader only needs the api; exercise the lookup directly
	_ = c
	ups := IdentityUpgraders(func() *client.Client { return nil })
	if _, ok := ups[0]; !ok || ups[0].PriorSchema == nil {
		t.Fatal("no v0 upgrader with a prior schema")
	}
	// With no client (never in practice: the framework configures the
	// resource first) the guid is left empty and Read resolves it by name.
	prior := *ups[0].PriorSchema
	req := resource.UpgradeIdentityRequest{Identity: &tfsdk.ResourceIdentity{Schema: prior, Raw: tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}},
		map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "tank/a")})}}
	resp := resource.UpgradeIdentityResponse{Identity: &tfsdk.ResourceIdentity{Schema: IdentitySchema()}}
	ups[0].IdentityUpgrader(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var g string
	resp.Identity.GetAttribute(ctx, path.Root("guid"), &g)
	if g != "" {
		t.Errorf("guid without a client = %q, want empty", g)
	}
	// upgradeIdentityGuid is the lookup the upgrader performs with a client.
	if g, diags := upgradeIdentityGuid(ctx, f, "tank/a"); diags.HasError() || g != "111" {
		t.Errorf("upgrade lookup = %q %v", g, diags)
	}
	if g, diags := upgradeIdentityGuid(ctx, f, "tank/gone"); diags.HasError() || g != "" {
		t.Errorf("upgrade lookup of a gone dataset = %q %v, want empty and no error", g, diags)
	}
	f.failing = map[string]error{"pool.dataset.query": errors.New("boom")}
	if _, diags := upgradeIdentityGuid(ctx, f, "tank/a"); !diags.HasError() {
		t.Error("API failure during identity upgrade must be an error")
	}
}
