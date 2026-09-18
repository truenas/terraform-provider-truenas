// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"errors"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// errMissingStaticFields is returned by updatePayload when the config sets
// dhcp = false but omits one or more of ipaddress/netmask/gateway. Probed
// live against ipmi.lan.update's accepts schema (TrueNAS 25.10.4 Enterprise
// HA, wss://10.220.16.188): the static-IP variant of the discriminated
// "dhcp" union (additionalProperties: false) marks ipaddress/netmask/
// gateway as REQUIRED — sending a static update without all three is
// rejected by the API. This client-side check surfaces that as a clear
// Terraform diagnostic before ever calling ipmi.lan.update, rather than a
// raw API error.
var errMissingStaticFields = errors.New(
	"ipaddress, netmask, and gateway are all required when dhcp = false")

// ipmiLanAPI mirrors one element of the JSON array ipmi.lan.query returns.
// Probed live (TrueNAS 25.10.4 Enterprise HA, channel 1, static IP):
//
//	{"channel": 1, "id": 1, "ip_address_source": "static",
//	 "ip_address": "10.220.2.97", "mac_address": "3c:ec:ef:da:e4:1d",
//	 "subnet_mask": "255.255.240.0",
//	 "default_gateway_ip_address": "10.220.0.1",
//	 "default_gateway_mac_address": "00:00:00:00:00:00",
//	 "backup_gateway_ip_address": "0.0.0.0",
//	 "backup_gateway_mac_address": "00:00:00:00:00:00",
//	 "vlan_id": null, "vlan_id_enable": false, "vlan_priority": 0}
//
// DECISIVE password evidence: this object has NO "password" key at all —
// not present, not masked, not null. ipmi.lan.query never echoes a
// password under any name, so the resource's "password" field is modeled
// WriteOnly (never populated from an API response), the same convention
// this provider uses for iscsi_auth's "secret"/"peersecret".
//
// Field-name asymmetry between query and update is real, not an oversight:
// ipmi.lan.query reports "ip_address"/"subnet_mask"/
// "default_gateway_ip_address"/"ip_address_source", while ipmi.lan.update
// accepts "ipaddress"/"netmask"/"gateway"/"dhcp" (bool). responseToModel
// below bridges that gap explicitly.
//
// Cross-release probe (TrueNAS 26.0, wss://192.168.1.68): ipmi.lan.channels
// reported TWO channels ([1, 8]) rather than the HA box's single channel,
// confirming "channel" (not a fixed singleton) is the right identity for
// this resource; ipmi.lan.update's accepts schema is otherwise identical
// between the two releases (dhcp/password/vlan/apply_remote on the DHCP
// variant, plus ipaddress/netmask/gateway required on the static variant
// — diffed field-by-field, only generic core.get_methods metadata keys
// differ, e.g. "cli_private" vs "no_authz_required").
type ipmiLanAPI struct {
	Channel                 int64  `json:"channel"`
	ID                      int64  `json:"id"`
	IPAddressSource         string `json:"ip_address_source"` // e.g. "static", "DHCP" (probed: lowercase "static" on 25.10.4 HA)
	IPAddress               string `json:"ip_address"`
	MACAddress              string `json:"mac_address"`
	SubnetMask              string `json:"subnet_mask"`
	DefaultGatewayIPAddress string `json:"default_gateway_ip_address"`
	VlanID                  *int64 `json:"vlan_id"` // null = tagging disabled
	VlanIDEnable            bool   `json:"vlan_id_enable"`
	VlanPriority            int64  `json:"vlan_priority"`
}

// IPMILanModel is the Terraform state model for truenas_ipmi_lan.
//
// "channel" is Required + RequiresReplace (see schema.go): ipmi.lan.update
// takes the channel number as a positional argument identifying which
// pre-existing physical BMC LAN channel to configure — channels are never
// created or destroyed via this API (ipmi.lan.channels enumerates the
// fixed hardware set, probed live: [1] on the HA box), so "channel" is
// this resource's true identity, not an assignable property.
type IPMILanModel struct {
	ID           types.Int64  `tfsdk:"id"` // always equals Channel
	Channel      types.Int64  `tfsdk:"channel"`
	DHCP         types.Bool   `tfsdk:"dhcp"`
	IPAddress    types.String `tfsdk:"ipaddress"`
	Netmask      types.String `tfsdk:"netmask"`
	Gateway      types.String `tfsdk:"gateway"`
	Vlan         types.Int64  `tfsdk:"vlan"` // null = tagging disabled
	VlanPriority types.Int64  `tfsdk:"vlan_priority"`
	MACAddress   types.String `tfsdk:"mac_address"`
	Password     types.String `tfsdk:"password"`     // WriteOnly, Sensitive; never read back
	ApplyRemote  types.Bool   `tfsdk:"apply_remote"` // call-time directive only, not part of query state
}

// IPMILanDataSourceModel is the read-only model for the truenas_ipmi_lan
// datasource. "channel" is the required lookup key (Required, not
// Computed); every other field mirrors the resource's read-only surface.
// It has no "password" or "apply_remote" fields: a password is never
// readable from the API (see ipmiLanAPI's doc comment), and apply_remote
// is a write-time directive with no corresponding read state.
type IPMILanDataSourceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Channel      types.Int64  `tfsdk:"channel"`
	DHCP         types.Bool   `tfsdk:"dhcp"`
	IPAddress    types.String `tfsdk:"ipaddress"`
	Netmask      types.String `tfsdk:"netmask"`
	Gateway      types.String `tfsdk:"gateway"`
	Vlan         types.Int64  `tfsdk:"vlan"`
	VlanPriority types.Int64  `tfsdk:"vlan_priority"`
	MACAddress   types.String `tfsdk:"mac_address"`
}

// parseChannel parses a Terraform import ID (a channel number, e.g. "1")
// into an int64.
func parseChannel(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

// ipmiLanQueryArgs builds the single "data" argument ipmi.lan.query takes.
// DECISIVE probe finding: unlike most *.query methods in this codebase
// (which take two positional args, filters then options), ipmi.lan.query's
// accepts schema has exactly one positional argument named "data" — an
// object with (hyphenated!) "query-filters"/"query-options"/"ipmi-options"
// keys. Calling it the usual two-positional-args way fails with "[EINVAL]
// data: Input should be a valid dictionary or instance of IPMILanQuery"
// (confirmed live against the disposable Enterprise HA test box).
func ipmiLanQueryArgs(channel int64) map[string]any {
	return map[string]any{
		"query-filters": [][]any{{"channel", "=", channel}},
	}
}

// isDHCP reports whether an ip_address_source value means DHCP. Probed live
// value is the lowercase string "static" for a statically-configured
// channel; the API doc string for ip_address_source only gives "e.g.,
// \"DHCP\", \"Static\"" as an example rather than an exhaustive enum, so
// this compares case-insensitively rather than assuming exact casing.
// Cross-release probe (TrueNAS 26.0, wss://192.168.1.68, a second, unused BMC
// channel 8 present on that box): also observed the value "unspecified"
// for a channel with no configured address at all (ip_address "0.0.0.0",
// mac_address all-zero) — treated as non-DHCP (false) here, same as any
// other value that isn't literally "dhcp".
func isDHCP(ipAddressSource string) bool {
	return strings.EqualFold(ipAddressSource, "dhcp")
}

// responseToModel maps an ipmiLanAPI response onto an IPMILanModel.
// Password and ApplyRemote are NOT touched here — see their field doc
// comments on IPMILanModel; callers must preserve whatever value the
// caller already populated (from req.Config, per the WriteOnly convention
// this provider uses elsewhere, e.g. iscsi_auth).
func responseToModel(api *ipmiLanAPI, m *IPMILanModel) {
	m.ID = types.Int64Value(api.Channel)
	m.Channel = types.Int64Value(api.Channel)
	m.DHCP = types.BoolValue(isDHCP(api.IPAddressSource))
	m.IPAddress = types.StringValue(api.IPAddress)
	m.Netmask = types.StringValue(api.SubnetMask)
	m.Gateway = types.StringValue(api.DefaultGatewayIPAddress)
	if api.VlanID != nil {
		m.Vlan = types.Int64Value(*api.VlanID)
	} else {
		m.Vlan = types.Int64Null()
	}
	m.VlanPriority = types.Int64Value(api.VlanPriority)
	m.MACAddress = types.StringValue(api.MACAddress)
}

// responseToDataSourceModel is the datasource counterpart of responseToModel.
func responseToDataSourceModel(api *ipmiLanAPI, m *IPMILanDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.Channel)
	m.Channel = types.Int64Value(api.Channel)
	m.DHCP = types.BoolValue(isDHCP(api.IPAddressSource))
	m.IPAddress = types.StringValue(api.IPAddress)
	m.Netmask = types.StringValue(api.SubnetMask)
	m.Gateway = types.StringValue(api.DefaultGatewayIPAddress)
	if api.VlanID != nil {
		m.Vlan = types.Int64Value(*api.VlanID)
	} else {
		m.Vlan = types.Int64Null()
	}
	m.VlanPriority = types.Int64Value(api.VlanPriority)
	m.MACAddress = types.StringValue(api.MACAddress)

	return diags
}

// updatePayload builds the map[string]any "data" payload for
// ipmi.lan.update(channel, data). Config-driven (twofactor_auth/
// webshare_config/tn_connect_config/failover_config precedent): every
// field beyond the always-required "dhcp" is included ONLY when the
// caller's model has a non-null, non-unknown value for it — CALLERS MUST
// invoke this on a model populated from req.Config, never req.Plan, so an
// unconfigured field is never silently resent via a plan modifier's
// UseStateForUnknown echo.
//
// "dhcp" is always sent: probed live, both branches of ipmi.lan.update's
// discriminated union require it (additionalProperties: false, "dhcp" is
// the discriminator itself), so there is no meaningful "unconfigured"
// state for it — schema.go makes it Required.
//
// "ipaddress"/"netmask"/"gateway" are sent ONLY when dhcp = false, and are
// validated present here (errMissingStaticFields) rather than left to a
// raw API rejection: probed live, the DHCP-variant schema has
// additionalProperties: false and does NOT accept these three keys at all
// — sending them alongside dhcp = true is rejected outright.
//
// "vlan" is accepted by BOTH branches (probed live) so it is sent whenever
// configured, regardless of dhcp.
//
// "vlan" is NEVER sent as an explicit null, even though the API accepts
// one and does clear a previously-set tag (re-confirmed live: TrueNAS
// 25.10.4 Enterprise HA, wss://10.220.16.188, channel 1 — setting vlan=100
// then sending an explicit "vlan": null round-tripped to "vlan_id": null,
// "vlan_id_enable": false on the next ipmi.lan.query, and the box was
// restored and independently re-verified byte-for-byte afterward). An
// earlier version of this method DID send an explicit null on a detected
// state-had-value/config-is-null transition (a "stateVlanSet" parameter),
// but that was reverted: req.Config shows null for "vlan" both when the
// user writes `vlan = null` AND when "vlan" is simply never mentioned in
// their configuration at all — Optional+Computed attributes make these two
// cases genuinely indistinguishable, not just here but at the
// terraform-plugin-framework/terraform-core level (confirmed against
// int64planmodifier.UseStateForUnknown's source: for a null config value
// on a top-level Optional+Computed attribute, Terraform's own core-
// computed proposed plan value comes in *unknown*, and this plan modifier
// then substitutes the PRIOR STATE value for it — identically whether the
// null came from an explicit `vlan = null` or from omission). So Terraform's
// own planned value for "vlan" stays the prior non-null value in both
// cases, never null — meaning a version of this method that infers "clear"
// from state-was-set + config-is-null and sends an explicit null to the
// API produces a final state that no longer matches what Terraform itself
// planned, which is exactly the "Provider produced inconsistent result
// after apply" crash this reversion fixes. There is no available signal
// (config, plan, or otherwise) this method could use to safely distinguish
// intent, so it never attempts to clear "vlan" — see vlanCannotBeCleared
// and its use in resource.go's Update, which instead surfaces a diagnostic
// telling the caller how to clear it outside of a normal Terraform apply.
//
// "password" is sent only when configured and non-empty (mirrors
// iscsi_auth's PeerSecret handling): never read back from the API (see
// ipmiLanAPI's doc comment), so an empty/unset value must never overwrite
// an existing BMC password as a side effect of some other field changing.
//
// "apply_remote" is sent only when configured: probed live in
// ipmi.lan.update's accepts schema, present on BOTH branches, default
// false, "If on an HA system, and this field is set to True, the settings
// will be sent to the remote controller." SAFETY: this provider's own
// committed acceptance tests never set apply_remote = true — pushing this
// channel's own address/netmask/gateway to the OTHER controller's BMC
// would very likely collide with that peer's own distinct BMC address
// rather than being a safe no-op, even when every other field is
// unchanged. Leave unconfigured (the default) to manage only the local
// controller's BMC LAN channel.
func (m *IPMILanModel) updatePayload() (map[string]any, error) {
	dhcp := m.DHCP.ValueBool()
	p := map[string]any{"dhcp": dhcp}

	if !dhcp {
		if m.IPAddress.IsNull() || m.IPAddress.IsUnknown() ||
			m.Netmask.IsNull() || m.Netmask.IsUnknown() ||
			m.Gateway.IsNull() || m.Gateway.IsUnknown() {
			return nil, errMissingStaticFields
		}
		p["ipaddress"] = m.IPAddress.ValueString()
		p["netmask"] = m.Netmask.ValueString()
		p["gateway"] = m.Gateway.ValueString()
	}

	if !m.Vlan.IsNull() && !m.Vlan.IsUnknown() {
		p["vlan"] = m.Vlan.ValueInt64()
	}

	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		if v := m.Password.ValueString(); v != "" {
			p["password"] = v
		}
	}

	if !m.ApplyRemote.IsNull() && !m.ApplyRemote.IsUnknown() {
		p["apply_remote"] = m.ApplyRemote.ValueBool()
	}

	return p, nil
}

// vlanCannotBeCleared reports whether resource.go's Update should refuse to
// proceed with an apply rather than silently leave "vlan" unchanged on the
// BMC while Terraform's own state moves it to null. True exactly when the
// prior state has a non-null "vlan" and the new config's "vlan" is null —
// see updatePayload's doc comment above for why this is the ONLY signal
// available (config alone can't tell an explicit `vlan = null` apart from
// "vlan" simply never being managed, and neither can Terraform's own
// planned value, which UseStateForUnknown keeps pinned to the prior
// non-null state in both cases). Note this necessarily also fires for a
// config that has genuinely never managed "vlan" at all, whenever some
// OTHER field on the same resource changes and Update is invoked for that
// unrelated reason — there is no way to avoid that false positive while
// still catching the real "user asked to clear vlan" case, which is why
// resource.go's Update surfaces this as an actionable diagnostic (set
// "vlan" explicitly to its current value to keep applying unrelated
// changes) rather than silently guessing either way.
func vlanCannotBeCleared(stateVlan, configVlan types.Int64) bool {
	return !stateVlan.IsNull() && configVlan.IsNull()
}
