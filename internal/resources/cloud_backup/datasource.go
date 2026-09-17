// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &CloudBackupDataSource{}

// CloudBackupDataSource implements the truenas_cloud_backup data source.
type CloudBackupDataSource struct{ client *client.Client }

// NewDataSource returns a new CloudBackupDataSource.
func NewDataSource() datasource.DataSource { return &CloudBackupDataSource{} }

func (d *CloudBackupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_backup"
}

func (d *CloudBackupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS cloud backup task by description.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the cloud backup task."},
			"description": dschema.StringAttribute{Required: true, Description: "Task description to look up."},
			"path":        dschema.StringAttribute{Computed: true, Description: "The local path backed up."},
			"credentials": dschema.Int64Attribute{Computed: true, Description: "ID of the cloud sync credentials used."},
			"attributes":  dschema.StringAttribute{Computed: true, Description: "JSON document of provider-specific attributes."},
			"schedule": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Cron schedule dictating when the task should run.",
				Attributes: map[string]dschema.Attribute{
					"minute": dschema.StringAttribute{Computed: true},
					"hour":   dschema.StringAttribute{Computed: true},
					"dom":    dschema.StringAttribute{Computed: true},
					"month":  dschema.StringAttribute{Computed: true},
					"dow":    dschema.StringAttribute{Computed: true},
				},
			},
			"pre_script":  dschema.StringAttribute{Computed: true, Description: "Bash script run immediately before every backup."},
			"post_script": dschema.StringAttribute{Computed: true, Description: "Bash script run immediately after every successful backup."},
			"snapshot":    dschema.BoolAttribute{Computed: true, Description: "Whether a temporary dataset snapshot is taken before every backup."},
			"include":     dschema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Paths passed to `restic backup --include`."},
			"exclude":     dschema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Paths passed to `restic backup --exclude`."},
			"enabled":     dschema.BoolAttribute{Computed: true, Description: "Whether the task is enabled."},
			"password": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Password for the restic repository. See the resource's schema documentation for read-back caveats.",
			},
			"keep_last":        dschema.Int64Attribute{Computed: true, Description: "How many of the most recent backup snapshots to keep."},
			"transfer_setting": dschema.StringAttribute{Computed: true, Description: "Transfer performance profile."},
			"absolute_paths":   dschema.BoolAttribute{Computed: true, Description: "Whether absolute paths are preserved in each backup."},
			"cache_path":       dschema.StringAttribute{Computed: true, Description: "Local path used to cache restic metadata, if configured."},
			"rate_limit":       dschema.Int64Attribute{Computed: true, Description: "Maximum upload/download rate in KiB/s, if configured."},
		},
	}
}

func (d *CloudBackupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CloudBackupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CloudBackupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "cloud_backup.query",
		[]any{[]any{"description", "=", state.Description.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query cloud backup tasks failed", err.Error())
		return
	}

	var results []cloudBackupAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Cloud backup task not found",
			fmt.Sprintf("No cloud backup task with description %q was found.", state.Description.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
