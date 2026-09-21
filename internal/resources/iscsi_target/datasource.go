// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ISCSITargetDataSource{}

// ISCSITargetDataSource implements the truenas_iscsi_target data source.
type ISCSITargetDataSource struct{ client *client.Client }

// NewDataSource returns a new ISCSITargetDataSource.
func NewDataSource() datasource.DataSource { return &ISCSITargetDataSource{} }

func (d *ISCSITargetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_target"
}

func (d *ISCSITargetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS iSCSI target by name.",
		Attributes: map[string]dschema.Attribute{
			"id":            dschema.Int64Attribute{Computed: true, Description: "Numeric iSCSI target ID."},
			"name":          dschema.StringAttribute{Required: true, Description: "iSCSI target name to look up."},
			"alias":         dschema.StringAttribute{Computed: true},
			"mode":          dschema.StringAttribute{Computed: true},
			"rel_tgt_id":    dschema.Int64Attribute{Computed: true},
			"auth_networks": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"iscsi_parameters": dschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dschema.Attribute{
					"queued_commands": dschema.Int64Attribute{Computed: true},
				},
			},
			"groups": dschema.ListNestedAttribute{
				Computed: true,
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"portal":     dschema.Int64Attribute{Computed: true},
						"initiator":  dschema.Int64Attribute{Computed: true},
						"auth":       dschema.Int64Attribute{Computed: true},
						"authmethod": dschema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ISCSITargetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ISCSITargetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ISCSITargetModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: iscsi.target.query([["name", "=", "<name>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "iscsi.target.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query iSCSI targets failed", err.Error())
		return
	}

	var results []targetAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"iSCSI target not found",
			fmt.Sprintf("No iSCSI target named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
