// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package pool contains unit tests for the truenas_pool datasource.
package pool

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestPoolDataSourceModelTopologyIsNullable is a regression test for a live
// acceptance failure: "Received null value, however the target type cannot
// handle null values. Path: topology. Target Type: pool.TopologyModel".
// PoolDataSourceModel.Topology must be a types.Object (which can represent
// null) rather than a bare TopologyModel struct (which cannot), because the
// datasource schema's "topology" attribute is Computed-only - it is always
// null when read from req.Config.Get.
func TestPoolDataSourceModelTopologyIsNullable(t *testing.T) {
	var m PoolDataSourceModel
	m.Name = types.StringValue("tank")
	m.Topology = types.ObjectNull(topologyAttrTypes)

	if !m.Topology.IsNull() {
		t.Fatal("expected Topology to be representable as null")
	}
}

// TestPoolResponseToDataSourceModel verifies that responseToDataSourceModel
// maps a TrueNAS API response (with topology children) into a non-null
// types.Object matching topologyAttrTypes, using the same conversion logic
// as the resource's responseToModel.
func TestPoolResponseToDataSourceModel(t *testing.T) {
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

	var m PoolDataSourceModel
	diags := responseToDataSourceModel(ctx, api, &m, testResolver())
	if diags.HasError() {
		t.Fatalf("responseToDataSourceModel returned errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("expected ID=42, got %d", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "tank" {
		t.Errorf("expected Name=tank, got %q", m.Name.ValueString())
	}
	if m.Topology.IsNull() || m.Topology.IsUnknown() {
		t.Fatal("expected Topology to be a known, non-null object")
	}

	var topo TopologyModel
	if diags := m.Topology.As(ctx, &topo, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("failed to decode Topology object: %v", diags)
	}
	var dataVdevs []VdevModel
	if diags := topo.Data.ElementsAs(ctx, &dataVdevs, false); diags.HasError() {
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
