// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_initiator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ISCSIInitiatorDataSource{}

// ISCSIInitiatorDataSource implements the truenas_iscsi_initiator data source.
type ISCSIInitiatorDataSource struct{ client *client.Client }

// NewDataSource returns a new ISCSIInitiatorDataSource.
func NewDataSource() datasource.DataSource { return &ISCSIInitiatorDataSource{} }

func (d *ISCSIInitiatorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_initiator"
}

func (d *ISCSIInitiatorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS iSCSI initiator group by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":         dschema.Int64Attribute{Required: true, Description: "Numeric iSCSI initiator ID to look up."},
			"comment":    dschema.StringAttribute{Computed: true},
			"initiators": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
	}
}

func (d *ISCSIInitiatorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ISCSIInitiatorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ISCSIInitiatorModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by ID: iscsi.initiator.query([["id","=",id]])
	queryFilters := []any{[]any{"id", "=", state.ID.ValueInt64()}}
	raw, err := d.client.CallRead(ctx, "iscsi.initiator.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query iSCSI initiators failed", err.Error())
		return
	}

	var results []initiatorAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"iSCSI initiator not found",
			fmt.Sprintf("No iSCSI initiator with ID %d was found.", state.ID.ValueInt64()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
