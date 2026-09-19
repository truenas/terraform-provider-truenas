// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &VMDataSource{}

// VMDataSource implements the truenas_vm data source.
type VMDataSource struct{ client *client.Client }

// NewDataSource returns a new VMDataSource.
func NewDataSource() datasource.DataSource { return &VMDataSource{} }

func (d *VMDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (d *VMDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS VM by name.",
		Attributes: map[string]dschema.Attribute{
			"id":               dschema.Int64Attribute{Computed: true, Description: "Numeric VM ID."},
			"name":             dschema.StringAttribute{Required: true, Description: "VM name to look up."},
			"description":      dschema.StringAttribute{Computed: true},
			"memory":           dschema.Int64Attribute{Computed: true, Description: "Memory allocated to the VM, in bytes."},
			"min_memory":       dschema.Int64Attribute{Computed: true, Description: "Minimum memory for ballooning, in bytes (0 = unset)."},
			"vcpus":            dschema.Int64Attribute{Computed: true},
			"cores":            dschema.Int64Attribute{Computed: true},
			"threads":          dschema.Int64Attribute{Computed: true},
			"bootloader":       dschema.StringAttribute{Computed: true},
			"autostart":        dschema.BoolAttribute{Computed: true},
			"time":             dschema.StringAttribute{Computed: true},
			"shutdown_timeout": dschema.Int64Attribute{Computed: true},
			"cpu_mode":         dschema.StringAttribute{Computed: true},
			"cpu_model":        dschema.StringAttribute{Computed: true},
			"running":          dschema.BoolAttribute{Computed: true},
			"status":           dschema.StringAttribute{Computed: true, Description: "Current VM status: RUNNING, STOPPED."},
		},
	}
}

func (d *VMDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VMDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state VMModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "vm.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query VMs failed", err.Error())
		return
	}

	var results []vmAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"VM not found",
			fmt.Sprintf("No VM named %q was found.", state.Name.ValueString()),
		)
		return
	}

	responseToModel(&results[0], &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
