// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &LXCConfigDataSource{}

// LXCConfigDataSource implements the truenas_lxc_config data source.
type LXCConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new LXCConfigDataSource.
func NewDataSource() datasource.DataSource { return &LXCConfigDataSource{} }

func (d *LXCConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lxc_config"
}

func (d *LXCConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS LXC service configuration. Takes no arguments: there " +
			"is exactly one LXC configuration per TrueNAS system. Requires TrueNAS 26.0 or later (see " +
			"the truenas_lxc_config resource's schema description for the version-gate details).",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"lxc_config\".",
			},
			"preferred_pool": dschema.StringAttribute{
				Computed: true,
				Description: "ZFS storage pool backing LXC-based instances and image datasets, or null if " +
					"not configured.",
			},
			"bridge": dschema.StringAttribute{
				Computed: true,
				Description: "Network bridge interface used for LXC instance networking, or null if not " +
					"configured (managed/created automatically).",
			},
			"v4_network": dschema.StringAttribute{
				Computed:    true,
				Description: "IPv4 network CIDR for the LXC instance bridge network.",
			},
			"v6_network": dschema.StringAttribute{
				Computed:    true,
				Description: "IPv6 network CIDR for the LXC instance bridge network.",
			},
		},
	}
}

func (d *LXCConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *LXCConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state LXCConfigDataSourceModel

	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Version probe failed", err.Error())
		return
	}
	resp.Diagnostics.Append(versionGateDiagnostics(version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "lxc.config")
	if err != nil {
		resp.Diagnostics.AddError("Read LXC configuration failed", err.Error())
		return
	}

	var api lxcConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse lxc.config response", err.Error())
		return
	}

	responseToDataSourceModel(&api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
