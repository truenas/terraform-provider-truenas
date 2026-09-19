// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_global

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ISCSIGlobalDataSource{}

// ISCSIGlobalDataSource implements the truenas_iscsi_global data source.
type ISCSIGlobalDataSource struct{ client *client.Client }

// NewDataSource returns a new ISCSIGlobalDataSource.
func NewDataSource() datasource.DataSource { return &ISCSIGlobalDataSource{} }

func (d *ISCSIGlobalDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_global"
}

func (d *ISCSIGlobalDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS global iSCSI service configuration. Takes no arguments: " +
			"there is exactly one iSCSI global configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"iscsi_global\".",
			},
			"basename": dschema.StringAttribute{
				Computed:    true,
				Description: "Base name (iSCSI Qualified Name prefix) used when generating target names.",
			},
			"listen_port": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP port the iSCSI service listens on.",
			},
			"alua": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether Asymmetric Logical Unit Access (ALUA) is enabled.",
			},
			"iser": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether iSER (iSCSI Extensions for RDMA) is enabled.",
			},
			"isns_servers": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of iSNS server hostnames or IP addresses.",
			},
			"pool_avail_threshold": dschema.Int64Attribute{
				Computed:    true,
				Description: "Percentage of pool free space remaining that triggers a warning alert. 0 means disabled.",
			},
		},
	}
}

func (d *ISCSIGlobalDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ISCSIGlobalDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ISCSIGlobalDataSourceModel

	raw, err := d.client.CallRead(ctx, "iscsi.global.config")
	if err != nil {
		resp.Diagnostics.AddError("Read iSCSI global configuration failed", err.Error())
		return
	}

	var api iscsiGlobalAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse iscsi.global.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
