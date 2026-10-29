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

	// Type-specific tuning (coverage audit).
	EnableLearning types.Bool   `tfsdk:"enable_learning"`  // BRIDGE
	LacpduRate     types.String `tfsdk:"lacpdu_rate"`      // LINK_AGGREGATION (LACP): SLOW/FAST
	XmitHashPolicy types.String `tfsdk:"xmit_hash_policy"` // LINK_AGGREGATION: LAYER2/LAYER2+3/LAYER3+4
	VlanPCP        types.Int64  `tfsdk:"vlan_pcp"`         // VLAN: priority code point 0-7
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
	EnableLearning      *bool      `json:"enable_learning"`
	LacpduRate          *string    `json:"lacpdu_rate"`
	XmitHashPolicy      *string    `json:"xmit_hash_policy"`
	VlanPCP             *int64     `json:"vlan_pcp"`
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

	// stp exists only on a BRIDGE; other types report it absent (nil), which
	// maps to null like the other type-specific fields. Reading it as false
	// made generate-config import write stp = false for a PHYSICAL
	// interface, which ValidateConfig then rejects (B-158).
	if api.STP != nil {
		m.STP = types.BoolValue(*api.STP)
	} else {
		m.STP = types.BoolNull()
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

	// Type-specific tuning (coverage audit): null when not applicable to the
	// interface's type.
	if api.EnableLearning != nil {
		m.EnableLearning = types.BoolValue(*api.EnableLearning)
	} else {
		m.EnableLearning = types.BoolNull()
	}
	if api.LacpduRate != nil {
		m.LacpduRate = types.StringValue(*api.LacpduRate)
	} else {
		m.LacpduRate = types.StringNull()
	}
	if api.XmitHashPolicy != nil {
		m.XmitHashPolicy = types.StringValue(*api.XmitHashPolicy)
	} else {
		m.XmitHashPolicy = types.StringNull()
	}
	if api.VlanPCP != nil {
		m.VlanPCP = types.Int64Value(*api.VlanPCP)
	} else {
		m.VlanPCP = types.Int64Null()
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
		if !m.EnableLearning.IsNull() && !m.EnableLearning.IsUnknown() {
			p["enable_learning"] = m.EnableLearning.ValueBool()
		}
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
		if !m.LacpduRate.IsNull() && !m.LacpduRate.IsUnknown() {
			p["lacpdu_rate"] = m.LacpduRate.ValueString()
		}
		if !m.XmitHashPolicy.IsNull() && !m.XmitHashPolicy.IsUnknown() {
			p["xmit_hash_policy"] = m.XmitHashPolicy.ValueString()
		}
	case "VLAN":
		if !m.VlanParentInterface.IsNull() && !m.VlanParentInterface.IsUnknown() {
			p["vlan_parent_interface"] = m.VlanParentInterface.ValueString()
		}
		if !m.VlanTag.IsNull() && !m.VlanTag.IsUnknown() {
			p["vlan_tag"] = m.VlanTag.ValueInt64()
		}
		if !m.VlanPCP.IsNull() && !m.VlanPCP.IsUnknown() {
			p["vlan_pcp"] = m.VlanPCP.ValueInt64()
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

// physicalDisallowedFieldsSet returns the names of type-specific attributes
// (valid only for BRIDGE/LINK_AGGREGATION/VLAN) that the config sets. On a
// PHYSICAL interface these are invalid: interface.update silently ignores them,
// so a plan that set one would otherwise fail with an opaque "inconsistent
// result after apply". A field counts as set only when the config gives it a
// meaningful value (non-null/unknown; non-empty for strings/lists), so a value
// merely carried forward by UseStateForUnknown never trips it. A PHYSICAL
// interface supports only the base IP fields (ipv4_dhcp, ipv6_auto, aliases,
// mtu, description).
func physicalDisallowedFieldsSet(m *NetworkInterfaceModel) []string {
	strSet := func(v types.String) bool { return !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" }
	boolSet := func(v types.Bool) bool { return !v.IsNull() && !v.IsUnknown() }
	intSet := func(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() && v.ValueInt64() != 0 }
	listSet := func(v types.List) bool { return !v.IsNull() && !v.IsUnknown() && len(v.Elements()) > 0 }

	checks := []struct {
		name string
		set  bool
	}{
		{"bridge_members", listSet(m.BridgeMembers)},
		{"stp", boolSet(m.STP)},
		{"enable_learning", boolSet(m.EnableLearning)},
		{"lag_protocol", strSet(m.LagProtocol)},
		{"lag_ports", listSet(m.LagPorts)},
		{"lacpdu_rate", strSet(m.LacpduRate)},
		{"xmit_hash_policy", strSet(m.XmitHashPolicy)},
		{"vlan_parent_interface", strSet(m.VlanParentInterface)},
		{"vlan_tag", intSet(m.VlanTag)},
		{"vlan_pcp", intSet(m.VlanPCP)},
	}
	var out []string
	for _, c := range checks {
		if c.set {
			out = append(out, c.name)
		}
	}
	return out
}
