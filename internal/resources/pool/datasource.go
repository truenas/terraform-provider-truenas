// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &PoolDataSource{}

// PoolDataSource reads a TrueNAS pool by name.
type PoolDataSource struct {
	client *client.Client
}

// NewDataSource returns a new PoolDataSource.
func NewDataSource() datasource.DataSource { return &PoolDataSource{} }

func (d *PoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (d *PoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceSchema()
}

func (d *PoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state PoolDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "pool.query", [][]any{{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Read pool failed", err.Error())
		return
	}

	var pools []poolAPI
	if err := json.Unmarshal(raw, &pools); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	if len(pools) == 0 {
		resp.Diagnostics.AddError("Pool not found",
			fmt.Sprintf("no pool named %q found on TrueNAS", state.Name.ValueString()))
		return
	}

	// Resolve topology disk names to stable serials (issue #9); a disk.query
	// failure is non-fatal (empty resolver falls back to raw names).
	res, _ := newDiskResolver(ctx, d.client)
	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &pools[0], &state, res)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
