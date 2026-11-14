// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package pool contains unit tests for the truenas_pool resource model.
package pool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mirrorTopo builds a TopologyModel carrying a single data vdev of the given
// type and disks, for reconcile tests.
func mirrorTopo(t *testing.T, vdevType string, disks ...string) TopologyModel {
	t.Helper()
	ctx := context.Background()
	diskList, d := types.ListValueFrom(ctx, types.StringType, disks)
	if d.HasError() {
		t.Fatalf("build disk list: %v", d)
	}
	vdevs := []VdevModel{{Type: types.StringValue(vdevType), Disks: diskList}}
	dataList, d := types.ListValueFrom(ctx, vdevObjectType, vdevs)
	if d.HasError() {
		t.Fatalf("build vdev list: %v", d)
	}
	empty, d := types.ListValueFrom(ctx, vdevObjectType, []VdevModel{})
	if d.HasError() {
		t.Fatalf("build empty vdev list: %v", d)
	}
	emptyDisks, d := types.ListValueFrom(ctx, types.StringType, []string{})
	if d.HasError() {
		t.Fatalf("build empty disk list: %v", d)
	}
	return TopologyModel{Data: dataList, Log: empty, Cache: emptyDisks, Spare: emptyDisks}
}

// TestReconcilePlanTopology_mirrorMembersOutOfOrder reproduces issue #66: a
// mirror whose members are listed in config by their serials in a DIFFERENT
// order than pool.query reports (and than state stores, as kernel sdX names).
// A vdev's members are a set, so this is not a topology change — but the
// positional reconcile compared plan[i] against state[i], each a different
// physical disk, failed to collapse the diff, and ModifyPlan then refused the
// apply as an in-place topology change. The reconciled data vdev must equal
// the state value so topologyChanged reports no change.
func TestReconcilePlanTopology_mirrorMembersOutOfOrder(t *testing.T) {
	ctx := context.Background()
	res := testResolver(diskA, diskB) // sdb=WD-WCC7K5PACL0V, sdc=SMC0515D90925BQ85185

	// State: kernel names in pool.query order.
	state := mirrorTopo(t, "MIRROR", "sdb", "sdc")
	// Plan: same two disks by serial, swapped order.
	plan := mirrorTopo(t, "MIRROR", diskB.Serial, diskA.Serial)

	got, diags := reconcilePlanTopology(ctx, plan, &state, res)
	if diags.HasError() {
		t.Fatalf("reconcilePlanTopology: %v", diags)
	}
	if topologyChanged(got, state) {
		t.Errorf("out-of-order serials for the same mirror members read as a topology change; "+
			"reconciled data = %v, state data = %v", got.Data, state.Data)
	}
}

// TestReconcilePlanTopology_realMemberSwapStillReplaces guards the fix: a vdev
// whose membership genuinely changes (a different physical disk) must still
// read as a topology change so ModifyPlan refuses the in-place apply.
func TestReconcilePlanTopology_realMemberSwapStillReplaces(t *testing.T) {
	ctx := context.Background()
	res := testResolver(diskA, diskB, diskC)

	state := mirrorTopo(t, "MIRROR", "sdb", "sdc")              // diskA, diskB
	plan := mirrorTopo(t, "MIRROR", diskA.Serial, diskC.Serial) // diskB replaced by diskC

	got, diags := reconcilePlanTopology(ctx, plan, &state, res)
	if diags.HasError() {
		t.Fatalf("reconcilePlanTopology: %v", diags)
	}
	if !topologyChanged(got, state) {
		t.Error("a real member swap must read as a topology change (force replacement)")
	}
}

// TestReconcilePlanTopology_inOrderSameFormCollapses is a sanity check that the
// set-based reconcile still collapses the ordinary case: same disks, same
// order, config by serial vs state by kernel name.
func TestReconcilePlanTopology_inOrderSameFormCollapses(t *testing.T) {
	ctx := context.Background()
	res := testResolver(diskA, diskB)

	state := mirrorTopo(t, "MIRROR", "sdb", "sdc")
	plan := mirrorTopo(t, "MIRROR", diskA.Serial, diskB.Serial)

	got, diags := reconcilePlanTopology(ctx, plan, &state, res)
	if diags.HasError() {
		t.Fatalf("reconcilePlanTopology: %v", diags)
	}
	if topologyChanged(got, state) {
		t.Errorf("same disks in same order read as a change; reconciled = %v, state = %v", got.Data, state.Data)
	}
}

// TestAutotrimParsedUnmarshalJSON is a regression test for a live
// acceptance failure: pool.query / pool.get_instance return autotrim as a
// ZFS property object whose "parsed" field is the string "on"/"off", not a
// JSON bool. A naive `bool` struct field failed with "json: cannot
// unmarshal string into Go struct field .autotrim.parsed of type bool".
func TestAutotrimParsedUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    bool
		wantErr bool
	}{
		{name: "string on", json: `"on"`, want: true},
		{name: "string off", json: `"off"`, want: false},
		{name: "string ON uppercase", json: `"ON"`, want: true},
		{name: "string OFF uppercase", json: `"OFF"`, want: false},
		{name: "bool true", json: `true`, want: true},
		{name: "bool false", json: `false`, want: false},
		{name: "unrecognized string", json: `"maybe"`, wantErr: true},
		{name: "unsupported type", json: `1`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got autotrimParsed
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %s, got none", tt.json)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %s: %v", tt.json, err)
			}
			if bool(got) != tt.want {
				t.Errorf("input %s: expected %v, got %v", tt.json, tt.want, bool(got))
			}
		})
	}
}

// TestPoolAPIAutotrimPropertyObjectDecode verifies that a full poolAPI
// response, shaped like the live pool.query payload (autotrim.parsed as a
// string), decodes without error and maps to the correct bool.
func TestPoolAPIAutotrimPropertyObjectDecode(t *testing.T) {
	raw := []byte(`{
		"id": 1,
		"name": "tank",
		"guid": "123",
		"status": "ONLINE",
		"healthy": true,
		"path": "/mnt/tank",
		"size": 100,
		"free": 50,
		"allocated": 50,
		"autotrim": {"parsed": "on", "rawvalue": "on", "value": "on", "source": "DEFAULT"},
		"topology": {"data": [], "log": [], "cache": [], "spare": []}
	}`)

	var api poolAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		t.Fatalf("failed to decode pool API response: %v", err)
	}
	if !bool(api.AutoTrim.Parsed) {
		t.Errorf("expected AutoTrim.Parsed=true, got false")
	}
}
