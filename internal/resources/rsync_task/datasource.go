// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

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
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the rsync task."},
			"desc": dschema.StringAttribute{Required: true, Description: "Task description to look up."},
			"path": dschema.StringAttribute{Computed: true, Description: "Local filesystem path to synchronize."},
			"user": dschema.StringAttribute{Computed: true, Description: "Username to run the rsync task as."},
			"mode": dschema.StringAttribute{
				Computed:    true,
				Description: "Operating mechanism for rsync: MODULE (rsync module/daemon protocol) or SSH.",
			},
			"remotehost": dschema.StringAttribute{
				Computed:    true,
				Description: "IP address or hostname of the remote system.",
			},
			"remoteport": dschema.Int64Attribute{
				Computed:    true,
				Description: "Port number for the SSH connection. Only applies when mode is SSH.",
			},
			"remotemodule": dschema.StringAttribute{
				Computed:    true,
				Description: "Name of the remote rsync module.",
			},
			"ssh_credentials": dschema.Int64Attribute{
				Computed:    true,
				Description: "Keychain credential ID (of type SSH_CREDENTIALS) used to connect to the remote host in SSH mode.",
			},
			"remotepath": dschema.StringAttribute{
				Computed:    true,
				Description: "Path on the remote system to synchronize with.",
			},
			"direction": dschema.StringAttribute{
				Computed:    true,
				Description: "Whether data is PUSHed to or PULLed from the remote system.",
			},
			"schedule": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Cron schedule for when the rsync task should run.",
				Attributes: map[string]dschema.Attribute{
					"minute": dschema.StringAttribute{Computed: true, Description: "Cron minute."},
					"hour":   dschema.StringAttribute{Computed: true, Description: "Cron hour."},
					"dom":    dschema.StringAttribute{Computed: true, Description: "Day of month."},
					"month":  dschema.StringAttribute{Computed: true, Description: "Month."},
					"dow":    dschema.StringAttribute{Computed: true, Description: "Day of week (cron format, 7=Sunday)."},
				},
			},
			"recursive": dschema.BoolAttribute{Computed: true, Description: "Recursively transfer subdirectories."},
			"times":     dschema.BoolAttribute{Computed: true, Description: "Preserve modification times of files."},
			"compress": dschema.BoolAttribute{
				Computed:    true,
				Description: "Reduce the size of the data to be transmitted.",
			},
			"archive": dschema.BoolAttribute{
				Computed: true,
				Description: "Make rsync run recursively, preserving symlinks, permissions, modification times, " +
					"group, and special files.",
			},
			"delete": dschema.BoolAttribute{
				Computed:    true,
				Description: "Delete files in the destination directory that do not exist in the source directory.",
			},
			"quiet": dschema.BoolAttribute{
				Computed:    true,
				Description: "Suppress informational messages from rsync.",
			},
			"preserveperm": dschema.BoolAttribute{
				Computed:    true,
				Description: "Preserve original file permissions.",
			},
			"preserveattr": dschema.BoolAttribute{
				Computed:    true,
				Description: "Preserve extended attributes of files.",
			},
			"delayupdates": dschema.BoolAttribute{
				Computed:    true,
				Description: "Delay updating destination files until all transfers are complete.",
			},
			"extra": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional rsync command-line options.",
			},
			"enabled": dschema.BoolAttribute{Computed: true, Description: "Whether this rsync task is enabled."},
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
