// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ReplicationConfigDataSource{}

// ReplicationConfigDataSource implements the truenas_replication_config
// data source.
type ReplicationConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new ReplicationConfigDataSource.
func NewDataSource() datasource.DataSource { return &ReplicationConfigDataSource{} }

func (d *ReplicationConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_replication_config"
}

func (d *ReplicationConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS replication configuration (system-wide replication " +
			"task concurrency). Takes no arguments: there is exactly one replication configuration per " +
			"TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"replication_config\".",
			},
			"max_parallel_replication_tasks": dschema.Int64Attribute{
				Computed:    true,
				Description: "Maximum number of replication tasks that may run in parallel. Null means unlimited.",
			},
		},
	}
}

func (d *ReplicationConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReplicationConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ReplicationConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "replication.config.config")
	if err != nil {
		resp.Diagnostics.AddError("Read replication configuration failed", err.Error())
		return
	}

	var api replicationConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse replication.config.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
