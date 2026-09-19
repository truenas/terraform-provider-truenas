// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ReplicationDataSource{}

// ReplicationDataSource implements the truenas_replication_task data source.
type ReplicationDataSource struct{ client *client.Client }

// NewDataSource returns a new ReplicationDataSource.
func NewDataSource() datasource.DataSource { return &ReplicationDataSource{} }

func (d *ReplicationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_replication_task"
}

func (d *ReplicationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS replication task by name.",
		Attributes: map[string]dschema.Attribute{
			"id":                                  dschema.Int64Attribute{Computed: true},
			"name":                                dschema.StringAttribute{Required: true, Description: "Name of the replication task to look up."},
			"direction":                           dschema.StringAttribute{Computed: true},
			"transport":                           dschema.StringAttribute{Computed: true},
			"ssh_credentials":                     dschema.Int64Attribute{Computed: true},
			"sudo":                                dschema.BoolAttribute{Computed: true},
			"compression":                         dschema.StringAttribute{Computed: true},
			"speed_limit":                         dschema.Int64Attribute{Computed: true},
			"netcat_active_side":                  dschema.StringAttribute{Computed: true},
			"netcat_active_side_listen_address":   dschema.StringAttribute{Computed: true},
			"netcat_active_side_port_min":         dschema.Int64Attribute{Computed: true},
			"netcat_active_side_port_max":         dschema.Int64Attribute{Computed: true},
			"netcat_passive_side_connect_address": dschema.StringAttribute{Computed: true},
			"source_datasets":                     dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"target_dataset":                      dschema.StringAttribute{Computed: true},
			"recursive":                           dschema.BoolAttribute{Computed: true},
			"exclude":                             dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"properties":                          dschema.BoolAttribute{Computed: true},
			"replicate":                           dschema.BoolAttribute{Computed: true},
			"periodic_snapshot_tasks":             dschema.ListAttribute{Computed: true, ElementType: types.Int64Type},
			"naming_schema":                       dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"also_include_naming_schema":          dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"name_regex":                          dschema.StringAttribute{Computed: true},
			"auto":                                dschema.BoolAttribute{Computed: true},
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
			"retention_policy": dschema.StringAttribute{Computed: true},
			"lifetime_value":   dschema.Int64Attribute{Computed: true},
			"lifetime_unit":    dschema.StringAttribute{Computed: true},
			"readonly":         dschema.StringAttribute{Computed: true},
			"enabled":          dschema.BoolAttribute{Computed: true},
			"retries":          dschema.Int64Attribute{Computed: true},
		},
	}
}

func (d *ReplicationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReplicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ReplicationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "replication.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query replication tasks failed", err.Error())
		return
	}

	var results []replicationAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Replication task not found",
			fmt.Sprintf("no replication task found with name %q", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
