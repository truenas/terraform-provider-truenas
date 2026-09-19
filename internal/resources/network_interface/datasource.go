// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NetworkInterfaceDataSource{}

// NetworkInterfaceDataSource implements the truenas_network_interface data source.
type NetworkInterfaceDataSource struct{ client *client.Client }

// NewDataSource returns a new NetworkInterfaceDataSource.
func NewDataSource() datasource.DataSource { return &NetworkInterfaceDataSource{} }

func (d *NetworkInterfaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_interface"
}

func (d *NetworkInterfaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS network interface by name.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.StringAttribute{Computed: true, Description: "Interface name (same as name)."},
			"name":        dschema.StringAttribute{Required: true, Description: "Interface name to look up."},
			"type":        dschema.StringAttribute{Computed: true, Description: "Interface type: PHYSICAL, BRIDGE, LINK_AGGREGATION, VLAN."},
			"description": dschema.StringAttribute{Computed: true},
			"mtu":         dschema.Int64Attribute{Computed: true, Description: "MTU size (0 = unset)."},
			"ipv4_dhcp":   dschema.BoolAttribute{Computed: true},
			"ipv6_auto":   dschema.BoolAttribute{Computed: true},
			"aliases": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"address": dschema.StringAttribute{Computed: true},
						"netmask": dschema.Int64Attribute{Computed: true},
						"type":    dschema.StringAttribute{Computed: true},
					},
				},
			},
			"bridge_members":        dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"stp":                   dschema.BoolAttribute{Computed: true},
			"lag_protocol":          dschema.StringAttribute{Computed: true},
			"lag_ports":             dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"vlan_parent_interface": dschema.StringAttribute{Computed: true},
			"vlan_tag":              dschema.Int64Attribute{Computed: true},
		},
	}
}

func (d *NetworkInterfaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NetworkInterfaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NetworkInterfaceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "interface.query", [][]any{{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query network interfaces failed", err.Error())
		return
	}

	var results []interfaceAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Network interface not found",
			fmt.Sprintf("No network interface named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
