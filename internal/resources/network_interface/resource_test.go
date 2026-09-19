// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringList(t *testing.T, ctx context.Context, vals []string) types.List {
	t.Helper()
	l, diags := types.ListValueFrom(ctx, types.StringType, vals)
	if diags.HasError() {
		t.Fatalf("building string list: %v", diags)
	}
	return l
}

func emptyAliasList(t *testing.T, ctx context.Context) types.List {
	t.Helper()
	l, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: aliasAttrTypes}, []AliasModel{})
	if diags.HasError() {
		t.Fatalf("building alias list: %v", diags)
	}
	return l
}

func baseModel(t *testing.T, ctx context.Context, ifType string) NetworkInterfaceModel {
	return NetworkInterfaceModel{
		Name:                types.StringValue("test0"),
		Type:                types.StringValue(ifType),
		Description:         types.StringValue(""),
		MTU:                 types.Int64Value(0),
		IPv4DHCP:            types.BoolValue(false),
		IPv6Auto:            types.BoolValue(false),
		Aliases:             emptyAliasList(t, ctx),
		BridgeMembers:       stringList(t, ctx, []string{}),
		STP:                 types.BoolValue(false),
		LagProtocol:         types.StringValue(""),
		LagPorts:            stringList(t, ctx, []string{}),
		VlanParentInterface: types.StringValue(""),
		VlanTag:             types.Int64Value(0),
	}
}

// TestCreatePayload_Bridge verifies that a BRIDGE create payload contains
// bridge_members (and stp) but no lag/vlan keys, plus name+type.
func TestCreatePayload_Bridge(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "BRIDGE")
	m.BridgeMembers = stringList(t, ctx, []string{"eth1", "eth2"})
	m.STP = types.BoolValue(true)

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}

	if _, ok := payload["bridge_members"]; !ok {
		t.Error("BRIDGE payload missing bridge_members")
	}
	if _, ok := payload["stp"]; !ok {
		t.Error("BRIDGE payload missing stp")
	}
	for _, k := range []string{"lag_protocol", "lag_ports", "vlan_parent_interface", "vlan_tag"} {
		if _, ok := payload[k]; ok {
			t.Errorf("BRIDGE payload should not contain %q", k)
		}
	}
	if payload["name"] != "test0" {
		t.Errorf("create payload name = %v, want test0", payload["name"])
	}
	if payload["type"] != "BRIDGE" {
		t.Errorf("create payload type = %v, want BRIDGE", payload["type"])
	}
}

// TestCreatePayload_Vlan verifies that a VLAN create payload contains
// vlan_parent_interface/vlan_tag but no bridge/lag keys.
func TestCreatePayload_Vlan(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "VLAN")
	m.VlanParentInterface = types.StringValue("eth0")
	m.VlanTag = types.Int64Value(100)

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}

	if _, ok := payload["vlan_parent_interface"]; !ok {
		t.Error("VLAN payload missing vlan_parent_interface")
	}
	if _, ok := payload["vlan_tag"]; !ok {
		t.Error("VLAN payload missing vlan_tag")
	}
	for _, k := range []string{"bridge_members", "stp", "lag_protocol", "lag_ports"} {
		if _, ok := payload[k]; ok {
			t.Errorf("VLAN payload should not contain %q", k)
		}
	}
}

// TestCreatePayload_LinkAggregation verifies that a LINK_AGGREGATION create
// payload contains lag_protocol/lag_ports but no bridge/vlan keys.
func TestCreatePayload_LinkAggregation(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "LINK_AGGREGATION")
	m.LagProtocol = types.StringValue("LACP")
	m.LagPorts = stringList(t, ctx, []string{"eth1", "eth2"})

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}

	if _, ok := payload["lag_protocol"]; !ok {
		t.Error("LINK_AGGREGATION payload missing lag_protocol")
	}
	if _, ok := payload["lag_ports"]; !ok {
		t.Error("LINK_AGGREGATION payload missing lag_ports")
	}
	for _, k := range []string{"bridge_members", "stp", "vlan_parent_interface", "vlan_tag"} {
		if _, ok := payload[k]; ok {
			t.Errorf("LINK_AGGREGATION payload should not contain %q", k)
		}
	}
	if payload["lag_protocol"] != "LACP" {
		t.Errorf("lag_protocol = %v, want LACP", payload["lag_protocol"])
	}
}

// TestCreatePayload_LagFieldsUnsetOmitted verifies that lag_protocol is
// omitted from the payload when null/unknown, rather than sent as "" (the
// Go zero value for types.String), which would incorrectly overwrite the
// API's chosen default.
func TestCreatePayload_LagFieldsUnsetOmitted(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "LINK_AGGREGATION")
	m.LagProtocol = types.StringNull()
	m.LagPorts = stringList(t, ctx, []string{"eth1"})

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}
	if _, ok := payload["lag_protocol"]; ok {
		t.Errorf("payload should not contain lag_protocol when unset, got %v", payload["lag_protocol"])
	}

	m2 := baseModel(t, ctx, "LINK_AGGREGATION")
	m2.LagProtocol = types.StringUnknown()
	m2.LagPorts = stringList(t, ctx, []string{"eth1"})

	payload2, diags2 := m2.createPayload(ctx)
	if diags2.HasError() {
		t.Fatalf("createPayload diags: %v", diags2)
	}
	if _, ok := payload2["lag_protocol"]; ok {
		t.Errorf("payload should not contain lag_protocol when unknown, got %v", payload2["lag_protocol"])
	}
}

// TestCreatePayload_VlanFieldsUnsetOmitted verifies that vlan_parent_interface
// and vlan_tag are omitted from the payload when null/unknown, rather than
// sent as ""/0.
func TestCreatePayload_VlanFieldsUnsetOmitted(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "VLAN")
	m.VlanParentInterface = types.StringNull()
	m.VlanTag = types.Int64Null()

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}
	if _, ok := payload["vlan_parent_interface"]; ok {
		t.Errorf("payload should not contain vlan_parent_interface when unset, got %v", payload["vlan_parent_interface"])
	}
	if _, ok := payload["vlan_tag"]; ok {
		t.Errorf("payload should not contain vlan_tag when unset, got %v", payload["vlan_tag"])
	}

	m2 := baseModel(t, ctx, "VLAN")
	m2.VlanParentInterface = types.StringUnknown()
	m2.VlanTag = types.Int64Unknown()

	payload2, diags2 := m2.createPayload(ctx)
	if diags2.HasError() {
		t.Fatalf("createPayload diags: %v", diags2)
	}
	if _, ok := payload2["vlan_parent_interface"]; ok {
		t.Errorf("payload should not contain vlan_parent_interface when unknown, got %v", payload2["vlan_parent_interface"])
	}
	if _, ok := payload2["vlan_tag"]; ok {
		t.Errorf("payload should not contain vlan_tag when unknown, got %v", payload2["vlan_tag"])
	}
}

// TestUpdatePayload_NoNameOrType verifies the update payload never carries
// name or type, unlike the create payload.
func TestUpdatePayload_NoNameOrType(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "BRIDGE")

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload diags: %v", diags)
	}

	if _, ok := payload["name"]; ok {
		t.Error("update payload should not contain name")
	}
	if _, ok := payload["type"]; ok {
		t.Error("update payload should not contain type")
	}
}

// TestCreatePayload_HasNameAndType verifies the create payload always
// carries name and type.
func TestCreatePayload_HasNameAndType(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "VLAN")

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}

	if _, ok := payload["name"]; !ok {
		t.Error("create payload missing name")
	}
	if _, ok := payload["type"]; !ok {
		t.Error("create payload missing type")
	}
}

// TestPayload_MTUZeroOmitted verifies that mtu is omitted from the payload
// when set to 0 (unset).
func TestPayload_MTUZeroOmitted(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "BRIDGE")
	m.MTU = types.Int64Value(0)

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}
	if _, ok := payload["mtu"]; ok {
		t.Error("payload should not contain mtu when set to 0")
	}
}

// TestPayload_MTUNonZeroIncluded verifies that a non-zero mtu is included.
func TestPayload_MTUNonZeroIncluded(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "BRIDGE")
	m.MTU = types.Int64Value(9000)

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}
	if payload["mtu"] != int64(9000) {
		t.Errorf("payload[mtu] = %v, want 9000", payload["mtu"])
	}
}

// TestCreatePayload_AliasEmptyTypeOmitted verifies that an alias whose type
// is unset (empty string) does not send "type": "" in the payload -- TrueNAS
// infers INET/INET6 from the address itself.
func TestCreatePayload_AliasEmptyTypeOmitted(t *testing.T) {
	ctx := context.Background()
	m := baseModel(t, ctx, "BRIDGE")

	aliasModels := []AliasModel{
		{
			Address: types.StringValue("10.0.0.1"),
			Netmask: types.Int64Value(24),
			Type:    types.StringValue(""),
		},
	}
	aliasList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: aliasAttrTypes}, aliasModels)
	if diags.HasError() {
		t.Fatalf("building alias list: %v", diags)
	}
	m.Aliases = aliasList

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload diags: %v", diags)
	}

	aliases, ok := payload["aliases"].([]map[string]any)
	if !ok || len(aliases) != 1 {
		t.Fatalf("payload aliases = %#v, want one alias map", payload["aliases"])
	}
	if _, ok := aliases[0]["type"]; ok {
		t.Errorf("alias payload should not contain %q key when type is empty, got %#v", "type", aliases[0])
	}
	if aliases[0]["address"] != "10.0.0.1" {
		t.Errorf("alias address = %v, want 10.0.0.1", aliases[0]["address"])
	}
}

// TestValidateCreateType_PhysicalRejected verifies that creating a PHYSICAL
// interface is rejected.
func TestValidateCreateType_PhysicalRejected(t *testing.T) {
	if err := validateCreateType("PHYSICAL"); err == nil {
		t.Error("expected error for PHYSICAL create, got nil")
	}
}

// TestValidateCreateType_VirtualTypesAllowed verifies that BRIDGE,
// LINK_AGGREGATION, and VLAN are all accepted for create.
func TestValidateCreateType_VirtualTypesAllowed(t *testing.T) {
	for _, typ := range []string{"BRIDGE", "LINK_AGGREGATION", "VLAN"} {
		if err := validateCreateType(typ); err != nil {
			t.Errorf("validateCreateType(%q) = %v, want nil", typ, err)
		}
	}
}

// TestResponseToModel_NilPointers verifies that nil pointer fields in
// interfaceAPI map to zero values (mtu -> 0, stp -> false, lag_protocol -> "",
// vlan_parent_interface -> "", vlan_tag -> 0) as would happen for a PHYSICAL
// interface's query response.
func TestResponseToModel_NilPointers(t *testing.T) {
	ctx := context.Background()
	api := &interfaceAPI{
		Name:                "eth0",
		Type:                "PHYSICAL",
		Description:         "",
		MTU:                 nil,
		STP:                 nil,
		LagProtocol:         nil,
		VlanParentInterface: nil,
		VlanTag:             nil,
	}

	var m NetworkInterfaceModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel diags: %v", diags)
	}

	if m.MTU.ValueInt64() != 0 {
		t.Errorf("MTU = %v, want 0", m.MTU.ValueInt64())
	}
	if m.STP.ValueBool() != false {
		t.Errorf("STP = %v, want false", m.STP.ValueBool())
	}
	if m.LagProtocol.ValueString() != "" {
		t.Errorf("LagProtocol = %q, want \"\"", m.LagProtocol.ValueString())
	}
	if m.VlanParentInterface.ValueString() != "" {
		t.Errorf("VlanParentInterface = %q, want \"\"", m.VlanParentInterface.ValueString())
	}
	if m.VlanTag.ValueInt64() != 0 {
		t.Errorf("VlanTag = %v, want 0", m.VlanTag.ValueInt64())
	}
	if m.ID.ValueString() != "eth0" {
		t.Errorf("ID = %q, want eth0", m.ID.ValueString())
	}
}

// TestResponseToModel_NonNilPointers verifies that non-nil pointer fields are
// dereferenced correctly.
func TestResponseToModel_NonNilPointers(t *testing.T) {
	ctx := context.Background()
	mtu := int64(9000)
	stp := true
	lagProto := "LACP"
	vlanParent := "eth0"
	vlanTag := int64(100)

	api := &interfaceAPI{
		Name:                "bond0",
		Type:                "LINK_AGGREGATION",
		MTU:                 &mtu,
		STP:                 &stp,
		LagProtocol:         &lagProto,
		VlanParentInterface: &vlanParent,
		VlanTag:             &vlanTag,
		LagPorts:            []string{"eth1", "eth2"},
	}

	var m NetworkInterfaceModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel diags: %v", diags)
	}

	if m.MTU.ValueInt64() != 9000 {
		t.Errorf("MTU = %v, want 9000", m.MTU.ValueInt64())
	}
	if !m.STP.ValueBool() {
		t.Error("STP = false, want true")
	}
	if m.LagProtocol.ValueString() != "LACP" {
		t.Errorf("LagProtocol = %q, want LACP", m.LagProtocol.ValueString())
	}
	if m.VlanParentInterface.ValueString() != "eth0" {
		t.Errorf("VlanParentInterface = %q, want eth0", m.VlanParentInterface.ValueString())
	}
	if m.VlanTag.ValueInt64() != 100 {
		t.Errorf("VlanTag = %v, want 100", m.VlanTag.ValueInt64())
	}

	var ports []string
	m.LagPorts.ElementsAs(ctx, &ports, false)
	if len(ports) != 2 || ports[0] != "eth1" || ports[1] != "eth2" {
		t.Errorf("LagPorts = %v, want [eth1 eth2]", ports)
	}
}

// TestResponseToModel_Aliases verifies aliases round-trip through the model.
func TestResponseToModel_Aliases(t *testing.T) {
	ctx := context.Background()
	api := &interfaceAPI{
		Name: "br0",
		Type: "BRIDGE",
		Aliases: []aliasAPI{
			{Address: "10.0.0.1", Netmask: 24, Type: "INET"},
		},
	}

	var m NetworkInterfaceModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel diags: %v", diags)
	}

	var aliases []AliasModel
	m.Aliases.ElementsAs(ctx, &aliases, false)
	if len(aliases) != 1 {
		t.Fatalf("len(aliases) = %d, want 1", len(aliases))
	}
	if aliases[0].Address.ValueString() != "10.0.0.1" {
		t.Errorf("alias address = %q, want 10.0.0.1", aliases[0].Address.ValueString())
	}
	if aliases[0].Netmask.ValueInt64() != 24 {
		t.Errorf("alias netmask = %v, want 24", aliases[0].Netmask.ValueInt64())
	}
	if aliases[0].Type.ValueString() != "INET" {
		t.Errorf("alias type = %q, want INET", aliases[0].Type.ValueString())
	}
}
