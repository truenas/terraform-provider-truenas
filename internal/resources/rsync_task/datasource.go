package rsync_task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &RsyncTaskDataSource{}

// RsyncTaskDataSource implements the truenas_rsync_task data source.
type RsyncTaskDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of RsyncTaskDataSource.
func NewDataSource() datasource.DataSource { return &RsyncTaskDataSource{} }

func (d *RsyncTaskDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rsync_task"
}

func (d *RsyncTaskDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS rsync task by description. Rsync tasks have no name field; desc serves as the lookup key.",
		Attributes: map[string]dschema.Attribute{
			"id":              dschema.Int64Attribute{Computed: true},
			"desc":            dschema.StringAttribute{Required: true, Description: "Task description to look up."},
			"path":            dschema.StringAttribute{Computed: true},
			"user":            dschema.StringAttribute{Computed: true},
			"mode":            dschema.StringAttribute{Computed: true},
			"remotehost":      dschema.StringAttribute{Computed: true},
			"remoteport":      dschema.Int64Attribute{Computed: true},
			"remotemodule":    dschema.StringAttribute{Computed: true},
			"ssh_credentials": dschema.Int64Attribute{Computed: true},
			"remotepath":      dschema.StringAttribute{Computed: true},
			"direction":       dschema.StringAttribute{Computed: true},
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
			"recursive":    dschema.BoolAttribute{Computed: true},
			"times":        dschema.BoolAttribute{Computed: true},
			"compress":     dschema.BoolAttribute{Computed: true},
			"archive":      dschema.BoolAttribute{Computed: true},
			"delete":       dschema.BoolAttribute{Computed: true},
			"quiet":        dschema.BoolAttribute{Computed: true},
			"preserveperm": dschema.BoolAttribute{Computed: true},
			"preserveattr": dschema.BoolAttribute{Computed: true},
			"delayupdates": dschema.BoolAttribute{Computed: true},
			"extra":        dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"enabled":      dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *RsyncTaskDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RsyncTaskDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state RsyncTaskDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "rsynctask.query",
		[]any{[]any{"desc", "=", state.Desc.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query rsync tasks failed", err.Error())
		return
	}

	var results []rsyncTaskAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Rsync task not found",
			fmt.Sprintf("no rsync task found with desc %q", state.Desc.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
