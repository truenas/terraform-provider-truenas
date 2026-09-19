// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NetworkConfigDataSource{}

// NetworkConfigDataSource implements the truenas_network_config data
// source.
type NetworkConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new NetworkConfigDataSource.
func NewDataSource() datasource.DataSource { return &NetworkConfigDataSource{} }

func (d *NetworkConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_config"
}

func (d *NetworkConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS global network configuration (hostname, domain, DNS " +
			"servers, default gateways, service announcement). Takes no arguments: there is exactly one global " +
			"network configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"network_config\".",
			},
			"hostname": dschema.StringAttribute{
				Computed:    true,
				Description: "System hostname.",
			},
			"domain": dschema.StringAttribute{
				Computed:    true,
				Description: "Primary DNS domain name.",
			},
			"domains": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional DNS search domains.",
			},
			"hosts": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional /etc/hosts entries.",
			},
			"httpproxy": dschema.StringAttribute{
				Computed:    true,
				Description: "HTTP proxy URL used by the system for outbound HTTP(S) requests.",
			},
			"ipv4gateway": dschema.StringAttribute{
				Computed:    true,
				Description: "Default IPv4 gateway.",
			},
			"ipv6gateway": dschema.StringAttribute{
				Computed:    true,
				Description: "Default IPv6 gateway.",
			},
			"nameserver1": dschema.StringAttribute{
				Computed:    true,
				Description: "Primary DNS nameserver.",
			},
			"nameserver2": dschema.StringAttribute{
				Computed:    true,
				Description: "Secondary DNS nameserver.",
			},
			"nameserver3": dschema.StringAttribute{
				Computed:    true,
				Description: "Tertiary DNS nameserver.",
			},
			"service_announcement": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Network service announcement/discovery protocols enabled on the system.",
				Attributes: map[string]dschema.Attribute{
					"mdns":    dschema.BoolAttribute{Computed: true, Description: "Whether mDNS (multicast DNS) service announcement is enabled."},
					"netbios": dschema.BoolAttribute{Computed: true, Description: "Whether NetBIOS name service announcement is enabled."},
					"wsd":     dschema.BoolAttribute{Computed: true, Description: "Whether WS-Discovery service announcement is enabled."},
				},
			},
		},
	}
}

func (d *NetworkConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NetworkConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NetworkConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "network.configuration.config")
	if err != nil {
		resp.Diagnostics.AddError("Read network configuration failed", err.Error())
		return
	}

	var api networkConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse network.configuration.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
