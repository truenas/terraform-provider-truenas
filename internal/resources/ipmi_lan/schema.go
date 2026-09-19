// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the LAN configuration of a single TrueNAS Enterprise BMC/IPMI channel " +
			"(ipmi.lan.query/ipmi.lan.update). IPMI LAN channels are a fixed set of pre-existing hardware " +
			"channels enumerated by ipmi.lan.channels (probed live on the disposable Enterprise HA test box: " +
			"just [1]) — they are never created or destroyed via this API, only configured, so \"channel\" " +
			"identifies which existing channel this resource manages rather than an assignable property; " +
			"Terraform create/update both call ipmi.lan.update, and Terraform delete only removes the resource " +
			"from state (the BMC's LAN configuration is left in place, never reset)." +
			"\n\n" +
			"SAFETY: this configures the out-of-band BMC network interface itself, not the host OS network. " +
			"Changing \"dhcp\", \"ipaddress\", \"netmask\", \"gateway\", or \"vlan\" to a value that does not " +
			"match what the physical network (switch port, VLAN trunking) actually expects can make the BMC " +
			"unreachable over its own network path until corrected — note that this is independent of the " +
			"Terraform provider's own connection to TrueNAS (which goes over the host's management network, " +
			"not the BMC's), so ipmi.lan.update itself always remains callable to correct a bad BMC network " +
			"change even if the BMC becomes temporarily unreachable by IP. This provider's own committed " +
			"acceptance tests only ever exercise \"vlan\" (see acceptance_test.go), always paired with a " +
			"restore registered via t.Cleanup before the change is made.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:      true,
				Description:   "Fixed identifier for this resource: always equal to \"channel\".",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"channel": schema.Int64Attribute{
				Required: true,
				Description: "IPMI LAN channel number to manage (see the truenas_ipmi_lan datasource or " +
					"ipmi.lan.channels for the set of channels that exist on this system). Identifies a " +
					"pre-existing hardware channel; changing it targets a different channel entirely rather " +
					"than renaming this one, so it forces replacement.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"dhcp": schema.BoolAttribute{
				Required: true,
				Description: "Whether this channel's BMC IP address is obtained via DHCP (true) or configured " +
					"statically (false). Required on every apply: probed live, ipmi.lan.update's accepts " +
					"schema always requires this key to select which variant (DHCP vs static) of the update " +
					"payload is being sent — there is no meaningful \"leave unconfigured\" state for it. " +
					"SAFETY: changing this can change how the BMC obtains its network address; see the " +
					"resource description above.",
			},
			"ipaddress": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Static IPv4 address for this channel, e.g. \"192.168.1.150\". Required (and " +
					"only sent) when \"dhcp\" = false — probed live, ipmi.lan.update's DHCP-variant schema " +
					"rejects this key outright (additionalProperties: false). Reads back the BMC's current " +
					"address either way.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"netmask": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Static netmask for this channel, e.g. \"255.255.255.0\". Required (and only " +
					"sent) when \"dhcp\" = false; see \"ipaddress\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"gateway": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Static default gateway for this channel, e.g. \"192.168.1.1\". Required (and " +
					"only sent) when \"dhcp\" = false; see \"ipaddress\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vlan": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "VLAN tag number (0-4096 per the API's own accepts schema) for this channel. " +
					"Accepted regardless of \"dhcp\". This is the only field this provider's own committed " +
					"acceptance tests exercise via set-and-restore (see the resource description's SAFETY " +
					"note): a real, probed live round trip against the disposable Enterprise HA test box, " +
					"always paired with a t.Cleanup restore registered before the change." +
					"\n\n" +
					"IMPORTANT: setting `vlan = null` (or removing it from config — Terraform cannot tell " +
					"these apart) does NOT disable VLAN tagging. The underlying API does accept an explicit " +
					"null and does clear a previously-set tag (probed live against ipmi.lan.update), but this " +
					"provider deliberately never sends one: because Optional+Computed attributes make an " +
					"explicit null indistinguishable from \"never configured\" at every layer Terraform " +
					"exposes to a provider (config AND plan — this attribute's own UseStateForUnknown plan " +
					"modifier keeps the planned value pinned to whatever is already in state either way), " +
					"there is no reliable signal this provider could use to tell a genuine clear request " +
					"apart from an ordinary unrelated update on a resource that has simply never managed " +
					"\"vlan\". If this channel currently has a non-null vlan and configuration omits or nulls " +
					"this attribute, Update returns an error diagnostic rather than guessing; set `vlan = " +
					"<its current value>` explicitly to keep applying unrelated changes. To actually clear a " +
					"previously-set VLAN tag, call ipmi.lan.update directly (TrueNAS UI, API, or midclient) " +
					"and then run `terraform apply -refresh-only` to reconcile state.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"vlan_priority": schema.Int64Attribute{
				Computed: true,
				Description: "Current 802.1p VLAN priority level reported for this channel. Read-only: " +
					"probed live, ipmi.lan.update's accepts schema has no \"vlan_priority\" key, only \"vlan\" " +
					"(the tag number) — priority is not independently settable through this API.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"mac_address": schema.StringAttribute{
				Computed:      true,
				Description:   "MAC address of this channel's BMC network interface. Read-only.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "New BMC/IPMI password for this channel (8-16 characters, ASCII upper/lower/" +
					"digits/special per the API's own accepts schema). Write-only: never stored in Terraform " +
					"state and never read back — probed live, ipmi.lan.query's response objects have no " +
					"\"password\" key at all, under any name, masked or otherwise. Omit to leave the BMC's " +
					"current password untouched. Requires Terraform >= 1.11." +
					"\n\n" +
					"SAFETY: this provider's own committed acceptance tests never set this field — an " +
					"incorrect password change could lock out legitimate BMC administrative access.",
			},
			"apply_remote": schema.BoolAttribute{
				Optional: true,
				Description: "If true on an Enterprise HA system, also sends this same update to the peer " +
					"controller's BMC LAN channel. A write-time directive only — it has no corresponding " +
					"read-back state (ipmi.lan.query has no such key) and is never sent unless explicitly " +
					"configured." +
					"\n\n" +
					"SAFETY: this provider's own committed acceptance tests never set this field to true — " +
					"see the resource description's SAFETY note and updatePayload's doc comment in model.go " +
					"for why resending this channel's own address/netmask/gateway to the peer's BMC is not a " +
					"safe operation to exercise live, even when nothing else about the payload changes.",
			},
		},
	}
}
