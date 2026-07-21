package scrub_task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ScrubTaskDataSource{}

// ScrubTaskDataSource implements the truenas_scrub_task data source.
type ScrubTaskDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of ScrubTaskDataSource.
func NewDataSource() datasource.DataSource { return &ScrubTaskDataSource{} }

func (d *ScrubTaskDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scrub_task"
}

func (d *ScrubTaskDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS scrub schedule by pool id.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.Int64Attribute{Computed: true},
			"pool":        dschema.Int64Attribute{Required: true, Description: "ID of the pool to look up."},
			"pool_name":   dschema.StringAttribute{Computed: true},
			"threshold":   dschema.Int64Attribute{Computed: true},
			"description": dschema.StringAttribute{Computed: true},
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
			"enabled": dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *ScrubTaskDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ScrubTaskDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ScrubTaskDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "pool.scrub.query",
		[]any{[]any{"pool", "=", state.Pool.ValueInt64()}})
	if err != nil {
		resp.Diagnostics.AddError("Query scrub tasks failed", err.Error())
		return
	}

	var results []scrubTaskAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Scrub task not found",
			fmt.Sprintf("no scrub task found for pool %d", state.Pool.ValueInt64()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
