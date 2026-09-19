// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ISCSIAuthDataSource{}

// ISCSIAuthDataSource implements the truenas_iscsi_auth data source.
//
// This datasource never exposes CHAP secrets: its schema and model have no
// "secret" or "peersecret" attributes.
type ISCSIAuthDataSource struct{ client *client.Client }

// NewDataSource returns a new ISCSIAuthDataSource.
func NewDataSource() datasource.DataSource { return &ISCSIAuthDataSource{} }

func (d *ISCSIAuthDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_auth"
}

func (d *ISCSIAuthDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Looks up a TrueNAS iSCSI CHAP auth credential group by tag. " +
			"Never exposes CHAP secrets: this datasource has no \"secret\" or \"peersecret\" attribute.",
		Attributes: map[string]dschema.Attribute{
			"id":             dschema.Int64Attribute{Computed: true, Description: "Numeric iSCSI auth entry ID assigned by TrueNAS."},
			"tag":            dschema.Int64Attribute{Required: true, Description: "Group ID for this CHAP credential to look up."},
			"user":           dschema.StringAttribute{Computed: true, Description: "CHAP username presented by the initiator."},
			"peeruser":       dschema.StringAttribute{Computed: true, Description: "Peer username for mutual CHAP."},
			"discovery_auth": dschema.StringAttribute{Computed: true, Description: "Discovery authentication method (NONE, CHAP, CHAP_MUTUAL)."},
		},
	}
}

func (d *ISCSIAuthDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ISCSIAuthDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ISCSIAuthDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryFilters := []any{[]any{"tag", "=", state.Tag.ValueInt64()}}
	raw, err := d.client.CallRead(ctx, "iscsi.auth.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query iSCSI auth entries failed", err.Error())
		return
	}

	var results []iscsiAuthAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"iSCSI auth entry not found",
			fmt.Sprintf("No iSCSI auth entry with tag %d was found.", state.Tag.ValueInt64()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
