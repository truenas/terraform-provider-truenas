// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &CronjobDataSource{}

// CronjobDataSource implements the truenas_cronjob data source.
type CronjobDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of CronjobDataSource.
func NewDataSource() datasource.DataSource { return &CronjobDataSource{} }

func (d *CronjobDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cronjob"
}

func (d *CronjobDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS cron job by description. Cron jobs have no unique name field; description " +
			"serves as the lookup key.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the cron job."},
			"description": dschema.StringAttribute{Required: true, Description: "Description to look up."},
			"command":     dschema.StringAttribute{Computed: true, Description: "Shell command or script to execute."},
			"user":        dschema.StringAttribute{Computed: true, Description: "System user account to run the command as."},
			"schedule": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Cron schedule for when the job runs.",
				Attributes: map[string]dschema.Attribute{
					"minute": dschema.StringAttribute{Computed: true, Description: "Cron minute."},
					"hour":   dschema.StringAttribute{Computed: true, Description: "Cron hour."},
					"dom":    dschema.StringAttribute{Computed: true, Description: "Day of month."},
					"month":  dschema.StringAttribute{Computed: true, Description: "Month."},
					"dow":    dschema.StringAttribute{Computed: true, Description: "Day of week (cron format, 7=Sunday)."},
				},
			},
			"enabled": dschema.BoolAttribute{Computed: true, Description: "Whether the cron job is active and will be executed."},
			"stdout": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether to IGNORE standard output (if false, it is included in the notification email).",
			},
			"stderr": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether to IGNORE standard error (if false, it is included in the notification email).",
			},
		},
	}
}

func (d *CronjobDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CronjobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CronjobDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "cronjob.query",
		[]any{[]any{"description", "=", state.Description.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query cron jobs failed", err.Error())
		return
	}

	var results []cronjobAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Cron job not found",
			fmt.Sprintf("no cron job found with description %q", state.Description.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
