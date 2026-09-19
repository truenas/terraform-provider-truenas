// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package init_shutdown_script

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &InitShutdownScriptDataSource{}

// InitShutdownScriptDataSource implements the truenas_init_shutdown_script
// data source.
type InitShutdownScriptDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of InitShutdownScriptDataSource.
func NewDataSource() datasource.DataSource { return &InitShutdownScriptDataSource{} }

func (d *InitShutdownScriptDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_init_shutdown_script"
}

func (d *InitShutdownScriptDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS init/shutdown script by comment. Init/shutdown scripts have no unique name " +
			"field; comment serves as the lookup key.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the init/shutdown script."},
			"comment": dschema.StringAttribute{Required: true, Description: "Comment to look up."},
			"type": dschema.StringAttribute{
				Computed:    true,
				Description: "Type of task: COMMAND or SCRIPT.",
			},
			"command": dschema.StringAttribute{Computed: true, Description: "Shell command to execute (COMMAND type)."},
			"script":  dschema.StringAttribute{Computed: true, Description: "Path to a script to execute (SCRIPT type)."},
			"when": dschema.StringAttribute{
				Computed:    true,
				Description: "When to execute: PREINIT, POSTINIT, or SHUTDOWN.",
			},
			"enabled": dschema.BoolAttribute{Computed: true, Description: "Whether this init/shutdown script is enabled to execute."},
			"timeout": dschema.Int64Attribute{
				Computed:    true,
				Description: "Time in seconds to wait for the command/script to complete.",
			},
		},
	}
}

func (d *InitShutdownScriptDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *InitShutdownScriptDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state InitShutdownScriptDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "initshutdownscript.query",
		[]any{[]any{"comment", "=", state.Comment.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query init/shutdown scripts failed", err.Error())
		return
	}

	var results []initShutdownScriptAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Init/shutdown script not found",
			fmt.Sprintf("no init/shutdown script found with comment %q", state.Comment.ValueString()),
		)
		return
	}

	responseToDataSourceModel(&results[0], &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
