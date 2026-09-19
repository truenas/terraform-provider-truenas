// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SnapshotDataSource{}

type SnapshotDataSource struct {
	client *client.Client
}

func NewDataSource() datasource.DataSource { return &SnapshotDataSource{} }

func (d *SnapshotDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snapshot"
}

func (d *SnapshotDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches information about a TrueNAS ZFS snapshot.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Required:    true,
				Description: "Snapshot identifier in the form \"dataset@name\" (e.g. tank/mydata@snap1).",
			},
			"dataset":   dschema.StringAttribute{Computed: true},
			"name":      dschema.StringAttribute{Computed: true},
			"pool":      dschema.StringAttribute{Computed: true},
			"createtxg": dschema.StringAttribute{Computed: true},
		},
	}
}

func (d *SnapshotDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *SnapshotDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config SnapshotDatasourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "pool.snapshot.get_instance", config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read snapshot failed", err.Error())
		return
	}

	var api snapshotAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	state := SnapshotDatasourceModel{
		ID:        types.StringValue(api.ID),
		Dataset:   types.StringValue(api.Dataset),
		Name:      types.StringValue(api.SnapshotName),
		Pool:      types.StringValue(api.Pool),
		CreateTxg: types.StringValue(api.CreateTxg),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
