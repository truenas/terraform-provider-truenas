// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &TargetExtentDataSource{}

// TargetExtentDataSource implements the truenas_iscsi_targetextent data
// source.
type TargetExtentDataSource struct{ client *client.Client }

// NewDataSource returns a new TargetExtentDataSource.
func NewDataSource() datasource.DataSource { return &TargetExtentDataSource{} }

func (d *TargetExtentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_targetextent"
}

func (d *TargetExtentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS iSCSI target/extent association (LUN mapping) by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":     dschema.Int64Attribute{Required: true, Description: "Numeric iSCSI target/extent association ID to look up."},
			"target": dschema.Int64Attribute{Computed: true, Description: "ID of the iSCSI target."},
			"extent": dschema.Int64Attribute{Computed: true, Description: "ID of the iSCSI extent."},
			"lunid":  dschema.Int64Attribute{Computed: true, Description: "LUN ID for this association."},
		},
	}
}

func (d *TargetExtentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TargetExtentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state TargetExtentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "iscsi.targetextent.get_instance", state.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read iSCSI target/extent association failed", err.Error())
		return
	}

	var apiResp targetExtentAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	responseToDataSourceModel(&apiResp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
