// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &IPMILanDataSource{}

// IPMILanDataSource implements the truenas_ipmi_lan datasource.
type IPMILanDataSource struct{ client *client.Client }

// NewDataSource returns a new IPMILanDataSource.
func NewDataSource() datasource.DataSource { return &IPMILanDataSource{} }

func (d *IPMILanDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipmi_lan"
}

func (d *IPMILanDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current LAN configuration of a TrueNAS Enterprise BMC/IPMI channel " +
			"(ipmi.lan.query). Makes no changes. Has no \"password\" attribute: probed live, ipmi.lan.query's " +
			"response objects never include a password under any name — see the truenas_ipmi_lan resource's " +
			"schema description for the full probed shape and safety notes.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.Int64Attribute{
				Computed:    true,
				Description: "Fixed identifier for this datasource read: always equal to \"channel\".",
			},
			"channel": dschema.Int64Attribute{
				Required:    true,
				Description: "IPMI LAN channel number to read (see ipmi.lan.channels for the set that exists on this system).",
			},
			"dhcp": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether this channel's BMC IP address is currently obtained via DHCP (true) or configured statically (false).",
			},
			"ipaddress": dschema.StringAttribute{
				Computed:    true,
				Description: "Current IPv4 address assigned to this channel's BMC network interface.",
			},
			"netmask": dschema.StringAttribute{
				Computed:    true,
				Description: "Current netmask for this channel.",
			},
			"gateway": dschema.StringAttribute{
				Computed:    true,
				Description: "Current default gateway for this channel.",
			},
			"vlan": dschema.Int64Attribute{
				Computed:    true,
				Description: "Current VLAN tag number for this channel, or unset/null if VLAN tagging is disabled.",
			},
			"vlan_priority": dschema.Int64Attribute{
				Computed:    true,
				Description: "Current 802.1p VLAN priority level reported for this channel.",
			},
			"mac_address": dschema.StringAttribute{
				Computed:    true,
				Description: "MAC address of this channel's BMC network interface.",
			},
		},
	}
}

func (d *IPMILanDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *IPMILanDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IPMILanDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	channel := config.Channel.ValueInt64()
	raw, err := d.client.CallRead(ctx, "ipmi.lan.query", ipmiLanQueryArgs(channel))
	if err != nil {
		resp.Diagnostics.AddError("Read IPMI LAN channel failed", err.Error())
		return
	}
	var results []ipmiLanAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse ipmi.lan.query response", err.Error())
		return
	}
	if len(results) == 0 {
		resp.Diagnostics.AddError("IPMI LAN channel not found", fmt.Sprintf("no IPMI LAN channel %d on this system", channel))
		return
	}

	var state IPMILanDataSourceModel
	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
