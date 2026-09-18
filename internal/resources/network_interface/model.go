// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// errPhysicalCreate is returned by validateCreateType when asked to create a
// PHYSICAL interface, which the API does not support -- physical interfaces
// must be imported instead.
var errPhysicalCreate = errors.New("physical interfaces cannot be created; import them instead")

// AliasModel maps to a single entry in the aliases list.
type AliasModel struct {
	Address types.String `tfsdk:"address"`
	Netmask types.Int64  `tfsdk:"netmask"`
	Type    types.String `tfsdk:"type"` // INET, INET6
}

// aliasAttrTypes is the attribute type map for an AliasModel object.
var aliasAttrTypes = map[string]attr.Type{
	"address": types.StringType,
	"netmask": types.Int64Type,
	"type":    types.StringType,
}

// NetworkInterfaceModel is the Terraform state model for truenas_network_interface.
type NetworkInterfaceModel struct {
	ID                  types.String `tfsdk:"id"`   // interface name
	Name                types.String `tfsdk:"name"` // e.g. br0, bond0, vlan100
	Type                types.String `tfsdk:"type"` // PHYSICAL (import only), BRIDGE, LINK_AGGREGATION, VLAN
	Description         types.String `tfsdk:"description"`
	MTU                 types.Int64  `tfsdk:"mtu"` // 0 = unset -> omitted from payload
	IPv4DHCP            types.Bool   `tfsdk:"ipv4_dhcp"`
	IPv6Auto            types.Bool   `tfsdk:"ipv6_auto"`
	Aliases             types.List   `tfsdk:"aliases"`        // List[AliasModel]
	BridgeMembers       types.List   `tfsdk:"bridge_members"` // List[String]
	STP                 types.Bool   `tfsdk:"stp"`
	LagProtocol         types.String `tfsdk:"lag_protocol"` // "" = unset
	LagPorts            types.List   `tfsdk:"lag_ports"`    // List[String]
	VlanParentInterface types.String `tfsdk:"vlan_parent_interface"`
	VlanTag             types.Int64  `tfsdk:"vlan_tag"` // 0 = unset
}

// aliasAPI is the JSON wire format for an alias entry.
type aliasAPI struct {
	Address string `json:"address"`
	Netmask int64  `json:"netmask"`
	Type    string `json:"type"`
}

// interfaceAPI is the JSON wire format for a TrueNAS network interface object.
// NOTE: for BRIDGE/LAG/VLAN-specific keys the physical-interface query response
// may omit them entirely -- all pointer types map nil -> zero value below.
type interfaceAPI struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Type                string     `json:"type"`
	Description         string     `json:"description"`
	MTU                 *int64     `json:"mtu"`
	IPv4DHCP            bool       `json:"ipv4_dhcp"`
	IPv6Auto            bool       `json:"ipv6_auto"`
	Aliases             []aliasAPI `json:"aliases"`
	BridgeMembers       []string   `json:"bridge_members"`
	STP                 *bool      `json:"stp"`
	LagProtocol         *string    `json:"lag_protocol"`
	LagPorts            []string   `json:"lag_ports"`
	VlanParentInterface *string    `json:"vlan_parent_interface"`
	VlanTag             *int64     `json:"vlan_tag"`
	// "state" (runtime) deliberately not modeled.
}

// responseToModel maps an interfaceAPI response onto a Terraform model.
func responseToModel(ctx context.Context, api *interfaceAPI, m *NetworkInterfaceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Type = types.StringValue(api.Type)
	m.Description = types.StringValue(api.Description)

	if api.MTU != nil {
		m.MTU = types.Int64Value(*api.MTU)
	} else {
		m.MTU = types.Int64Value(0)
	}

	m.IPv4DHCP = types.BoolValue(api.IPv4DHCP)
	m.IPv6Auto = types.BoolValue(api.IPv6Auto)

	if api.STP != nil {
		m.STP = types.BoolValue(*api.STP)
	} else {
		m.STP = types.BoolValue(false)
	}

	if api.LagProtocol != nil {
		m.LagProtocol = types.StringValue(*api.LagProtocol)
	} else {
		m.LagProtocol = types.StringValue("")
	}

	if api.VlanParentInterface != nil {
		m.VlanParentInterface = types.StringValue(*api.VlanParentInterface)
	} else {
		m.VlanParentInterface = types.StringValue("")
	}

	if api.VlanTag != nil {
		m.VlanTag = types.Int64Value(*api.VlanTag)
	} else {
		m.VlanTag = types.Int64Value(0)
	}

	// Convert aliases.
	aliasModels := make([]AliasModel, 0, len(api.Aliases))
	for _, a := range api.Aliases {
		aliasModels = append(aliasModels, AliasModel{
			Address: types.StringValue(a.Address),
			Netmask: types.Int64Value(a.Netmask),
			Type:    types.StringValue(a.Type),
		})
	}
	aliasList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: aliasAttrTypes}, aliasModels)
	diags.Append(d...)
	m.Aliases = aliasList

	// Convert bridge_members.
	bridgeMembers := api.BridgeMembers
	if bridgeMembers == nil {
		bridgeMembers = []string{}
	}
	bmList, d := types.ListValueFrom(ctx, types.StringType, bridgeMembers)
	diags.Append(d...)
	m.BridgeMembers = bmList

	// Convert lag_ports.
	lagPorts := api.LagPorts
	if lagPorts == nil {
		lagPorts = []string{}
	}
	lpList, d := types.ListValueFrom(ctx, types.StringType, lagPorts)
	diags.Append(d...)
	m.LagPorts = lpList

	return diags
}

// basePayload builds the type-agnostic portion of the create/update payload.
func (m *NetworkInterfaceModel) basePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var aliasItems []AliasModel
	if !m.Aliases.IsNull() && !m.Aliases.IsUnknown() {
		diags.Append(m.Aliases.ElementsAs(ctx, &aliasItems, false)...)
	}
	aliasesPayload := make([]map[string]any, 0, len(aliasItems))
	for _, a := range aliasItems {
		alias := map[string]any{
			"address": a.Address.ValueString(),
			"netmask": a.Netmask.ValueInt64(),
		}
		// Omit type when unset -- TrueNAS infers INET/INET6 from the address;
		// sending an empty string is rejected/incorrect.
		if !a.Type.IsNull() && !a.Type.IsUnknown() && a.Type.ValueString() != "" {
			alias["type"] = a.Type.ValueString()
		}
		aliasesPayload = append(aliasesPayload, alias)
	}

	p := map[string]any{
		"description": m.Description.ValueString(),
		"ipv4_dhcp":   m.IPv4DHCP.ValueBool(),
		"ipv6_auto":   m.IPv6Auto.ValueBool(),
		"aliases":     aliasesPayload,
	}

	if !m.MTU.IsNull() && !m.MTU.IsUnknown() && m.MTU.ValueInt64() != 0 {
		p["mtu"] = m.MTU.ValueInt64()
	}

	switch m.Type.ValueString() {
	case "BRIDGE":
		var bridgeMembers []string
		if !m.BridgeMembers.IsNull() && !m.BridgeMembers.IsUnknown() {
			diags.Append(m.BridgeMembers.ElementsAs(ctx, &bridgeMembers, false)...)
		}
		if bridgeMembers == nil {
			bridgeMembers = []string{}
		}
		p["bridge_members"] = bridgeMembers
		if !m.STP.IsNull() && !m.STP.IsUnknown() {
			p["stp"] = m.STP.ValueBool()
		}
	case "LINK_AGGREGATION":
		var lagPorts []string
		if !m.LagPorts.IsNull() && !m.LagPorts.IsUnknown() {
			diags.Append(m.LagPorts.ElementsAs(ctx, &lagPorts, false)...)
		}
		if lagPorts == nil {
			lagPorts = []string{}
		}
		if !m.LagProtocol.IsNull() && !m.LagProtocol.IsUnknown() {
			p["lag_protocol"] = m.LagProtocol.ValueString()
		}
		p["lag_ports"] = lagPorts
	case "VLAN":
		if !m.VlanParentInterface.IsNull() && !m.VlanParentInterface.IsUnknown() {
			p["vlan_parent_interface"] = m.VlanParentInterface.ValueString()
		}
		if !m.VlanTag.IsNull() && !m.VlanTag.IsUnknown() {
			p["vlan_tag"] = m.VlanTag.ValueInt64()
		}
	}

	return p, diags
}

// createPayload builds the payload for interface.create: base fields plus name+type.
func (m *NetworkInterfaceModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	p["name"] = m.Name.ValueString()
	p["type"] = m.Type.ValueString()
	return p, diags
}

// updatePayload builds the payload for interface.update: base fields only,
// name and type are never sent on update.
func (m *NetworkInterfaceModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	return m.basePayload(ctx)
}

// validateCreateType returns an error if the given type cannot be created via
// the API (physical interfaces must be imported, never created).
func validateCreateType(t string) error {
	if t == "PHYSICAL" {
		return errPhysicalCreate
	}
	return nil
}
