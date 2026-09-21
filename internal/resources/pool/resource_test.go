// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package pool contains unit tests for the truenas_pool resource.
// Acceptance tests (TF_ACC=1) are not included here because pool
// create/delete requires spare physical disks on the target TrueNAS.
package pool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPoolSchema verifies that resourceSchema returns a schema containing
// all expected top-level attributes.
func TestPoolSchema(t *testing.T) {
	s := resourceSchema()
	required := []string{
		"id", "name", "topology", "autotrim",
		"guid", "status", "healthy", "path",
		"size", "free", "allocated",
	}
	for _, attr := range required {
		if _, ok := s.Attributes[attr]; !ok {
			t.Errorf("resourceSchema missing attribute %q", attr)
		}
	}
}

// TestPoolAPIPayload constructs a PoolModel with a MIRROR data vdev and
// verifies that apiPayload produces the correct wire-format structure.
func TestPoolAPIPayload(t *testing.T) {
	ctx := context.Background()

	diskList, diags := types.ListValueFrom(ctx, types.StringType, []string{"sda", "sdb"})
	if diags.HasError() {
		t.Fatalf("failed to build disk list: %v", diags)
	}
	emptyCache, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("failed to build cache list: %v", diags)
	}
	emptySpare, diags := types.ListValueFrom(ctx, types.StringType, []string{})
	if diags.HasError() {
		t.Fatalf("failed to build spare list: %v", diags)
	}

	dataVdevList, diags := types.ListValueFrom(ctx, vdevObjectType, []VdevModel{{
		Type:  types.StringValue("MIRROR"),
		Disks: diskList,
	}})
	if diags.HasError() {
		t.Fatalf("failed to build data vdev list: %v", diags)
	}
	emptyLog, diags := types.ListValueFrom(ctx, vdevObjectType, []VdevModel{})
	if diags.HasError() {
		t.Fatalf("failed to build log list: %v", diags)
	}

	m := &PoolModel{
		Name: types.StringValue("testpool"),
		Topology: TopologyModel{
			Data:  dataVdevList,
			Log:   emptyLog,
			Cache: emptyCache,
			Spare: emptySpare,
		},
		AutoTrim: types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx, testResolver())
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	// Verify name
	if payload["name"] != "testpool" {
		t.Errorf("expected name=%q, got %v", "testpool", payload["name"])
	}

	// autotrim is NOT a pool.create input (rejected as "Extra inputs are not
	// permitted"); the resource applies it via a follow-up pool.update.
	if _, ok := payload["autotrim"]; ok {
		t.Errorf("pool.create payload must not contain autotrim, got %v", payload["autotrim"])
	}

	// Verify topology
	topo, ok := payload["topology"].(map[string]any)
	if !ok {
		t.Fatal("topology is not map[string]any")
	}

	data, ok := topo["data"].([]map[string]any)
	if !ok {
		t.Fatalf("topology.data is not []map[string]any, got %T", topo["data"])
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 data vdev, got %d", len(data))
	}
	if data[0]["type"] != "MIRROR" {
		t.Errorf("expected vdev type=MIRROR, got %v", data[0]["type"])
	}
	disks, ok := data[0]["disks"].([]string)
	if !ok {
		t.Fatalf("data[0].disks is not []string, got %T", data[0]["disks"])
	}
	if len(disks) != 2 || disks[0] != "sda" || disks[1] != "sdb" {
		t.Errorf("unexpected disks: %v", disks)
	}

	// pool.create's topology key is "spares" (plural), not "spare".
	if _, ok := topo["spare"]; ok {
		t.Error("topology must use \"spares\" (plural), not \"spare\"")
	}
	spare, ok := topo["spares"].([]string)
	if !ok {
		t.Fatalf("topology.spares is not []string, got %T", topo["spares"])
	}
	if len(spare) != 0 {
		t.Errorf("expected empty spares, got %v", spare)
	}

	// Cache should be an empty vdev slice
	cache, ok := topo["cache"].([]map[string]any)
	if !ok {
		t.Fatalf("topology.cache is not []map[string]any, got %T", topo["cache"])
	}
	if len(cache) != 0 {
		t.Errorf("expected empty cache vdevs, got %v", cache)
	}

	// Payload must serialize to valid JSON
	if _, err := json.Marshal(payload); err != nil {
		t.Errorf("payload does not marshal to JSON: %v", err)
	}
}

// TestPoolResponseToModel verifies that responseToModel correctly maps
// a TrueNAS API response (with topology children) into Terraform state.
func TestPoolResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &poolAPI{
		ID:        42,
		Name:      "tank",
		GUID:      "1234567890",
		Status:    "ONLINE",
		Healthy:   true,
		Path:      "/mnt/tank",
		Size:      1099511627776,
		Free:      1099511627776,
		Allocated: 0,
	}
	api.AutoTrim.Parsed = true
	api.Topology.Data = []poolVdev{
		{Type: "MIRROR", Children: []poolDisk{{Disk: "sda"}, {Disk: "sdb"}}},
	}
	api.Topology.Log = []poolVdev{}
	api.Topology.Cache = []poolVdev{}
	api.Topology.Spare = []poolVdev{}

	var m PoolModel
	diags := responseToModel(ctx, api, &m, testResolver())
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("expected ID=42, got %d", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "tank" {
		t.Errorf("expected Name=tank, got %q", m.Name.ValueString())
	}
	if !m.AutoTrim.ValueBool() {
		t.Error("expected AutoTrim=true")
	}
	var dataVdevs []VdevModel
	if diags := m.Topology.Data.ElementsAs(ctx, &dataVdevs, false); diags.HasError() {
		t.Fatalf("data ElementsAs failed: %v", diags)
	}
	if len(dataVdevs) != 1 {
		t.Fatalf("expected 1 data vdev, got %d", len(dataVdevs))
	}
	if dataVdevs[0].Type.ValueString() != "MIRROR" {
		t.Errorf("expected data[0].type=MIRROR, got %q", dataVdevs[0].Type.ValueString())
	}

	var diskNames []string
	if diags := dataVdevs[0].Disks.ElementsAs(ctx, &diskNames, false); diags.HasError() {
		t.Fatalf("ElementsAs failed: %v", diags)
	}
	if len(diskNames) != 2 || diskNames[0] != "sda" || diskNames[1] != "sdb" {
		t.Errorf("unexpected disk names: %v", diskNames)
	}
}

// TestPoolApiPayload_OmittedLogUnknown is the regression test for issue #7: a
// config that sets only topology.data leaves log/cache/spare as unknown values
// (they are Optional+Computed). Before the fix, TopologyModel.Log was a plain
// []VdevModel that could not hold an unknown, so req.Plan.Get crashed with
// "Received unknown value ... Target Type: []pool.VdevModel". With Data/Log as
// types.List, the model holds unknown and apiPayload emits empty lists for the
// omitted vdev types.
func TestPoolApiPayload_OmittedLogUnknown(t *testing.T) {
	ctx := context.Background()

	disks, diags := types.ListValueFrom(ctx, types.StringType, []string{"sdc", "sdd"})
	if diags.HasError() {
		t.Fatalf("disk list: %v", diags)
	}
	dataList, diags := types.ListValueFrom(ctx, vdevObjectType, []VdevModel{{
		Type:  types.StringValue("MIRROR"),
		Disks: disks,
	}})
	if diags.HasError() {
		t.Fatalf("data list: %v", diags)
	}

	m := &PoolModel{
		Name: types.StringValue("tank"),
		Topology: TopologyModel{
			Data:  dataList,
			Log:   types.ListUnknown(vdevObjectType),   // omitted -> unknown
			Cache: types.ListUnknown(types.StringType), // omitted -> unknown
			Spare: types.ListUnknown(types.StringType), // omitted -> unknown
		},
		AutoTrim: types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx, testResolver())
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors for unknown log/cache/spare: %v", diags)
	}
	topo := payload["topology"].(map[string]any)

	if data := topo["data"].([]map[string]any); len(data) != 1 {
		t.Errorf("data vdevs = %d, want 1", len(data))
	}
	for _, k := range []string{"log", "cache", "spares"} {
		v := topo[k]
		switch vv := v.(type) {
		case []map[string]any:
			if len(vv) != 0 {
				t.Errorf("topology.%s = %v, want empty", k, vv)
			}
		case []string:
			if len(vv) != 0 {
				t.Errorf("topology.%s = %v, want empty", k, vv)
			}
		default:
			t.Errorf("topology.%s has unexpected type %T", k, v)
		}
	}
}

// TestPoolApiPayload_NullTopologyLists verifies null (as opposed to unknown)
// omitted vdev lists are also tolerated and emit empty lists.
func TestPoolApiPayload_NullTopologyLists(t *testing.T) {
	ctx := context.Background()

	disks, _ := types.ListValueFrom(ctx, types.StringType, []string{"sdc", "sdd"})
	dataList, _ := types.ListValueFrom(ctx, vdevObjectType, []VdevModel{{Type: types.StringValue("MIRROR"), Disks: disks}})

	m := &PoolModel{
		Name: types.StringValue("tank"),
		Topology: TopologyModel{
			Data:  dataList,
			Log:   types.ListNull(vdevObjectType),
			Cache: types.ListNull(types.StringType),
			Spare: types.ListNull(types.StringType),
		},
		AutoTrim: types.BoolValue(false),
	}

	payload, diags := m.apiPayload(ctx, testResolver())
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors for null lists: %v", diags)
	}
	topo := payload["topology"].(map[string]any)
	if log := topo["log"].([]map[string]any); len(log) != 0 {
		t.Errorf("topology.log = %v, want empty", log)
	}
}

// TestPoolApiPayload_CacheAndSpares is the regression test for the pool.create
// payload shape (issue #7 follow-up): cache vdevs must be type "STRIPE" (not
// "DISK"), spares go under the "spares" key as a flat disk-name array, and
// autotrim is never sent to pool.create.
func TestPoolApiPayload_CacheAndSpares(t *testing.T) {
	ctx := context.Background()

	data, _ := types.ListValueFrom(ctx, types.StringType, []string{"sdc", "sdd"})
	dataList, _ := types.ListValueFrom(ctx, vdevObjectType, []VdevModel{{Type: types.StringValue("MIRROR"), Disks: data}})
	cache, _ := types.ListValueFrom(ctx, types.StringType, []string{"nvme0"})
	spares, _ := types.ListValueFrom(ctx, types.StringType, []string{"sde", "sdf"})

	m := &PoolModel{
		Name: types.StringValue("tank"),
		Topology: TopologyModel{
			Data:  dataList,
			Log:   types.ListNull(vdevObjectType),
			Cache: cache,
			Spare: spares,
		},
		AutoTrim: types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx, testResolver())
	if diags.HasError() {
		t.Fatalf("apiPayload: %v", diags)
	}
	if _, ok := payload["autotrim"]; ok {
		t.Error("pool.create payload must not contain autotrim")
	}
	topo := payload["topology"].(map[string]any)

	cacheVdevs, ok := topo["cache"].([]map[string]any)
	if !ok || len(cacheVdevs) != 1 {
		t.Fatalf("topology.cache = %v, want 1 vdev", topo["cache"])
	}
	if cacheVdevs[0]["type"] != "STRIPE" {
		t.Errorf("cache vdev type = %v, want STRIPE", cacheVdevs[0]["type"])
	}

	sp, ok := topo["spares"].([]string)
	if !ok || len(sp) != 2 || sp[0] != "sde" {
		t.Errorf("topology.spares = %v, want [sde sdf]", topo["spares"])
	}
	if _, ok := topo["spare"]; ok {
		t.Error("must not use singular \"spare\" key")
	}
}

// TestPoolResponseToModel_SingleDiskVdevs covers the read path for single-disk
// cache/log/spare vdevs, which TrueNAS reports with the device at the vdev's
// top level (empty children) and type "DISK". vdevDisks must fall back to
// vdev.disk, and a "DISK" log vdev must normalize to "STRIPE" so it round-trips
// with the create config.
func TestPoolResponseToModel_SingleDiskVdevs(t *testing.T) {
	ctx := context.Background()

	api := &poolAPI{ID: 7, Name: "tank", Status: "ONLINE", Healthy: true}
	api.AutoTrim.Parsed = false
	api.Topology.Data = []poolVdev{{Type: "MIRROR", Children: []poolDisk{{Disk: "sdb"}, {Disk: "sdc"}}}}
	// single-disk log/cache/spare: disk at top level, empty children, type DISK
	api.Topology.Log = []poolVdev{{Type: "DISK", Disk: "sdd"}}
	api.Topology.Cache = []poolVdev{{Type: "DISK", Disk: "sdf"}}
	api.Topology.Spare = []poolVdev{{Type: "DISK", Disk: "sdg"}}

	var m PoolModel
	if diags := responseToModel(ctx, api, &m, testResolver()); diags.HasError() {
		t.Fatalf("responseToModel: %v", diags)
	}

	// log: type normalized DISK->STRIPE, disk read from vdev.disk.
	var logVdevs []VdevModel
	if diags := m.Topology.Log.ElementsAs(ctx, &logVdevs, false); diags.HasError() {
		t.Fatalf("log ElementsAs: %v", diags)
	}
	if len(logVdevs) != 1 || logVdevs[0].Type.ValueString() != "STRIPE" {
		t.Fatalf("log vdev = %+v, want one STRIPE", logVdevs)
	}
	var logDisks []string
	_ = logVdevs[0].Disks.ElementsAs(ctx, &logDisks, false)
	if len(logDisks) != 1 || logDisks[0] != "sdd" {
		t.Errorf("log disks = %v, want [sdd]", logDisks)
	}

	// cache/spare: flat disk-name lists, read from vdev.disk.
	var cache, spare []string
	_ = m.Topology.Cache.ElementsAs(ctx, &cache, false)
	_ = m.Topology.Spare.ElementsAs(ctx, &spare, false)
	if len(cache) != 1 || cache[0] != "sdf" {
		t.Errorf("cache = %v, want [sdf]", cache)
	}
	if len(spare) != 1 || spare[0] != "sdg" {
		t.Errorf("spare = %v, want [sdg]", spare)
	}
}
