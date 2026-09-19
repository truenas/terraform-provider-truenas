// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &PeriodicSnapshotDataSource{}

// PeriodicSnapshotDataSource implements the truenas_periodic_snapshot_task data source.
type PeriodicSnapshotDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of PeriodicSnapshotDataSource.
func NewDataSource() datasource.DataSource { return &PeriodicSnapshotDataSource{} }

func (d *PeriodicSnapshotDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_periodic_snapshot_task"
}

func (d *PeriodicSnapshotDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS periodic snapshot task by dataset name.",
		Attributes: map[string]dschema.Attribute{
			"id":             dschema.Int64Attribute{Computed: true},
			"dataset":        dschema.StringAttribute{Required: true, Description: "Dataset path to look up."},
			"recursive":      dschema.BoolAttribute{Computed: true},
			"exclude":        dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"lifetime_value": dschema.Int64Attribute{Computed: true},
			"lifetime_unit":  dschema.StringAttribute{Computed: true},
			"naming_schema":  dschema.StringAttribute{Computed: true},
			"schedule": dschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dschema.Attribute{
					"minute": dschema.StringAttribute{Computed: true},
					"hour":   dschema.StringAttribute{Computed: true},
					"dom":    dschema.StringAttribute{Computed: true},
					"month":  dschema.StringAttribute{Computed: true},
					"dow":    dschema.StringAttribute{Computed: true},
				},
			},
			"allow_empty": dschema.BoolAttribute{Computed: true},
			"enabled":     dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *PeriodicSnapshotDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *PeriodicSnapshotDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state PeriodicSnapshotModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "pool.snapshottask.query",
		[]any{[]any{"dataset", "=", state.Dataset.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query periodic snapshot tasks failed", err.Error())
		return
	}

	var results []taskAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Periodic snapshot task not found",
			fmt.Sprintf("no task found for dataset %q", state.Dataset.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
