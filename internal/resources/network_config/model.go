// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// networkConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one global network configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const networkConfigResourceID = "network_config"

// serviceAnnouncementAttrTypes describes the attribute types of the nested
// "service_announcement" object.
var serviceAnnouncementAttrTypes = map[string]attr.Type{
	"mdns":    types.BoolType,
	"netbios": types.BoolType,
	"wsd":     types.BoolType,
}

// ServiceAnnouncementModel maps to the nested "service_announcement"
// attribute.
type ServiceAnnouncementModel struct {
	Mdns    types.Bool `tfsdk:"mdns"`
	Netbios types.Bool `tfsdk:"netbios"`
	Wsd     types.Bool `tfsdk:"wsd"`
}

// NetworkConfigModel is the Terraform state model for
// truenas_network_config.
type NetworkConfigModel struct {
	ID                  types.String `tfsdk:"id"` // fixed: "network_config"
	Hostname            types.String `tfsdk:"hostname"`
	Domain              types.String `tfsdk:"domain"`
	Domains             types.List   `tfsdk:"domains"` // List[String]
	Hosts               types.List   `tfsdk:"hosts"`   // List[String]
	HTTPProxy           types.String `tfsdk:"httpproxy"`
	IPv4Gateway         types.String `tfsdk:"ipv4gateway"` // "" -> nil on wire
	IPv6Gateway         types.String `tfsdk:"ipv6gateway"` // "" -> nil on wire
	Nameserver1         types.String `tfsdk:"nameserver1"` // "" -> nil on wire
	Nameserver2         types.String `tfsdk:"nameserver2"` // "" -> nil on wire
	Nameserver3         types.String `tfsdk:"nameserver3"` // "" -> nil on wire
	ServiceAnnouncement types.Object `tfsdk:"service_announcement"`
}

// NetworkConfigDataSourceModel is the read-only model for the
// truenas_network_config datasource.
type NetworkConfigDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Hostname            types.String `tfsdk:"hostname"`
	Domain              types.String `tfsdk:"domain"`
	Domains             types.List   `tfsdk:"domains"`
	Hosts               types.List   `tfsdk:"hosts"`
	HTTPProxy           types.String `tfsdk:"httpproxy"`
	IPv4Gateway         types.String `tfsdk:"ipv4gateway"`
	IPv6Gateway         types.String `tfsdk:"ipv6gateway"`
	Nameserver1         types.String `tfsdk:"nameserver1"`
	Nameserver2         types.String `tfsdk:"nameserver2"`
	Nameserver3         types.String `tfsdk:"nameserver3"`
	ServiceAnnouncement types.Object `tfsdk:"service_announcement"`
}

// serviceAnnouncementAPI is the JSON wire format of the nested
// "service_announcement" object.
type serviceAnnouncementAPI struct {
	Mdns    bool `json:"mdns"`
	Netbios bool `json:"netbios"`
	Wsd     bool `json:"wsd"`
}

// networkConfigAPI mirrors the JSON object returned by
// network.configuration.config and accepted (as a subset) by
// network.configuration.update. ipv4gateway, ipv6gateway, and nameserver1-3
// are nullable on the wire, so they are modeled as pointers, even though a
// live TrueNAS box has been observed to return "" (not null) for an unset
// nameserver3: mapping is defensive of either representation (see
// responseToModel). activity, hostname_b, hostname_virtual (HA-only), and
// state are intentionally NOT modeled: they are not writable configuration.
type networkConfigAPI struct {
	ID                  int64                   `json:"id"`
	Hostname            string                  `json:"hostname"`
	Domain              string                  `json:"domain"`
	Domains             []string                `json:"domains"`
	Hosts               []string                `json:"hosts"`
	HTTPProxy           string                  `json:"httpproxy"`
	IPv4Gateway         *string                 `json:"ipv4gateway"`
	IPv6Gateway         *string                 `json:"ipv6gateway"`
	Nameserver1         *string                 `json:"nameserver1"`
	Nameserver2         *string                 `json:"nameserver2"`
	Nameserver3         *string                 `json:"nameserver3"`
	ServiceAnnouncement *serviceAnnouncementAPI `json:"service_announcement"`
}

// stringOrEmpty maps a nullable wire string to a non-null Terraform string:
// nil -> "", otherwise the pointed-to value (which may itself already be
// "" on a live system, e.g. an unset nameserver3).
func stringOrEmpty(v *string) types.String {
	if v == nil {
		return types.StringValue("")
	}
	return types.StringValue(*v)
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *networkConfigAPI, m *NetworkConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(networkConfigResourceID)
	m.Hostname = types.StringValue(api.Hostname)
	m.Domain = types.StringValue(api.Domain)
	m.HTTPProxy = types.StringValue(api.HTTPProxy)

	domains := api.Domains
	if domains == nil {
		domains = []string{}
	}
	domainsList, d := types.ListValueFrom(ctx, types.StringType, domains)
	diags.Append(d...)
	m.Domains = domainsList

	hosts := api.Hosts
	if hosts == nil {
		hosts = []string{}
	}
	hostsList, d2 := types.ListValueFrom(ctx, types.StringType, hosts)
	diags.Append(d2...)
	m.Hosts = hostsList

	m.IPv4Gateway = stringOrEmpty(api.IPv4Gateway)
	m.IPv6Gateway = stringOrEmpty(api.IPv6Gateway)
	m.Nameserver1 = stringOrEmpty(api.Nameserver1)
	m.Nameserver2 = stringOrEmpty(api.Nameserver2)
	m.Nameserver3 = stringOrEmpty(api.Nameserver3)

	if api.ServiceAnnouncement != nil {
		saObj, d3 := types.ObjectValueFrom(ctx, serviceAnnouncementAttrTypes, ServiceAnnouncementModel{
			Mdns:    types.BoolValue(api.ServiceAnnouncement.Mdns),
			Netbios: types.BoolValue(api.ServiceAnnouncement.Netbios),
			Wsd:     types.BoolValue(api.ServiceAnnouncement.Wsd),
		})
		diags.Append(d3...)
		m.ServiceAnnouncement = saObj
	} else {
		m.ServiceAnnouncement = types.ObjectNull(serviceAnnouncementAttrTypes)
	}

	return diags
}

// responseToDataSourceModel maps an API response onto a
// NetworkConfigDataSourceModel using the same rules as responseToModel.
func responseToDataSourceModel(ctx context.Context, api *networkConfigAPI, m *NetworkConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(networkConfigResourceID)
	m.Hostname = types.StringValue(api.Hostname)
	m.Domain = types.StringValue(api.Domain)
	m.HTTPProxy = types.StringValue(api.HTTPProxy)

	domains := api.Domains
	if domains == nil {
		domains = []string{}
	}
	domainsList, d := types.ListValueFrom(ctx, types.StringType, domains)
	diags.Append(d...)
	m.Domains = domainsList

	hosts := api.Hosts
	if hosts == nil {
		hosts = []string{}
	}
	hostsList, d2 := types.ListValueFrom(ctx, types.StringType, hosts)
	diags.Append(d2...)
	m.Hosts = hostsList

	m.IPv4Gateway = stringOrEmpty(api.IPv4Gateway)
	m.IPv6Gateway = stringOrEmpty(api.IPv6Gateway)
	m.Nameserver1 = stringOrEmpty(api.Nameserver1)
	m.Nameserver2 = stringOrEmpty(api.Nameserver2)
	m.Nameserver3 = stringOrEmpty(api.Nameserver3)

	if api.ServiceAnnouncement != nil {
		saObj, d3 := types.ObjectValueFrom(ctx, serviceAnnouncementAttrTypes, ServiceAnnouncementModel{
			Mdns:    types.BoolValue(api.ServiceAnnouncement.Mdns),
			Netbios: types.BoolValue(api.ServiceAnnouncement.Netbios),
			Wsd:     types.BoolValue(api.ServiceAnnouncement.Wsd),
		})
		diags.Append(d3...)
		m.ServiceAnnouncement = saObj
	} else {
		m.ServiceAnnouncement = types.ObjectNull(serviceAnnouncementAttrTypes)
	}

	return diags
}

// updatePayload builds the network.configuration.update argument. Every
// writable field is guarded: each is only included when known
// (Optional+Computed). hostname, domain, domains, hosts, and httpproxy use a
// plain guard (omitted when null/unknown, sent as-is otherwise).
// ipv4gateway, ipv6gateway, and nameserver1-3 are omitted when null/unknown
// and otherwise sent as their string value ("" clears the field on TrueNAS —
// these must never be sent as JSON nil, which TrueNAS rejects).
// service_announcement is sent as a 3-key map only when the
// object is known (non-null, non-unknown); it is omitted entirely otherwise.
func (m *NetworkConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Hostname.IsNull() && !m.Hostname.IsUnknown() {
		p["hostname"] = m.Hostname.ValueString()
	}
	if !m.Domain.IsNull() && !m.Domain.IsUnknown() {
		p["domain"] = m.Domain.ValueString()
	}
	if !m.Domains.IsNull() && !m.Domains.IsUnknown() {
		var v []string
		diags.Append(m.Domains.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["domains"] = v
	}
	if !m.Hosts.IsNull() && !m.Hosts.IsUnknown() {
		var v []string
		diags.Append(m.Hosts.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["hosts"] = v
	}
	if !m.HTTPProxy.IsNull() && !m.HTTPProxy.IsUnknown() {
		p["httpproxy"] = m.HTTPProxy.ValueString()
	}
	// ipv4gateway/ipv6gateway/nameserver1-3: send the string value as-is — ""
	// clears the field. TrueNAS's schema for these is anyOf("", valid IP) and
	// REJECTS null, so an empty value must be sent as "" (not nil). Sending nil
	// for an empty gateway/nameserver fails create/update with
	// "[EINVAL] ... Input should be ''", which breaks any box that leaves
	// ipv6gateway or a nameserver slot empty (the common case).
	if !m.IPv4Gateway.IsNull() && !m.IPv4Gateway.IsUnknown() {
		p["ipv4gateway"] = m.IPv4Gateway.ValueString()
	}
	if !m.IPv6Gateway.IsNull() && !m.IPv6Gateway.IsUnknown() {
		p["ipv6gateway"] = m.IPv6Gateway.ValueString()
	}
	if !m.Nameserver1.IsNull() && !m.Nameserver1.IsUnknown() {
		p["nameserver1"] = m.Nameserver1.ValueString()
	}
	if !m.Nameserver2.IsNull() && !m.Nameserver2.IsUnknown() {
		p["nameserver2"] = m.Nameserver2.ValueString()
	}
	if !m.Nameserver3.IsNull() && !m.Nameserver3.IsUnknown() {
		p["nameserver3"] = m.Nameserver3.ValueString()
	}
	if !m.ServiceAnnouncement.IsNull() && !m.ServiceAnnouncement.IsUnknown() {
		var sa ServiceAnnouncementModel
		diags.Append(m.ServiceAnnouncement.As(ctx, &sa, basetypes.ObjectAsOptions{})...)
		p["service_announcement"] = map[string]bool{
			"mdns":    sa.Mdns.ValueBool(),
			"netbios": sa.Netbios.ValueBool(),
			"wsd":     sa.Wsd.ValueBool(),
		}
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the global network configuration controls
// hostname, DNS, and default gateways, so removing this resource from
// Terraform state must never rewrite the box's configuration and risk
// losing network connectivity. Splitting this into its own function keeps
// Delete's "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Network configuration left in place",
		"Network configuration left in place; removed from Terraform state only",
	)
	return diags
}
