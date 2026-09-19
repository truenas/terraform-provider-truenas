// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ISCSIExtentDataSource{}

// ISCSIExtentDataSource implements the truenas_iscsi_extent data source.
type ISCSIExtentDataSource struct{ client *client.Client }

// NewDataSource returns a new ISCSIExtentDataSource.
func NewDataSource() datasource.DataSource { return &ISCSIExtentDataSource{} }

func (d *ISCSIExtentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_extent"
}

func (d *ISCSIExtentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS iSCSI extent by name.",
		Attributes: map[string]dschema.Attribute{
			"id":              dschema.Int64Attribute{Computed: true, Description: "Numeric iSCSI extent ID."},
			"name":            dschema.StringAttribute{Required: true, Description: "iSCSI extent name to look up."},
			"type":            dschema.StringAttribute{Computed: true},
			"disk":            dschema.StringAttribute{Computed: true},
			"path":            dschema.StringAttribute{Computed: true},
			"filesize":        dschema.Int64Attribute{Computed: true},
			"comment":         dschema.StringAttribute{Computed: true},
			"blocksize":       dschema.Int64Attribute{Computed: true},
			"pblocksize":      dschema.BoolAttribute{Computed: true},
			"avail_threshold": dschema.Int64Attribute{Computed: true},
			"insecure_tpc":    dschema.BoolAttribute{Computed: true},
			"xen":             dschema.BoolAttribute{Computed: true},
			"ro":              dschema.BoolAttribute{Computed: true},
			"rpm":             dschema.StringAttribute{Computed: true},
			"enabled":         dschema.BoolAttribute{Computed: true},
			"naa":             dschema.StringAttribute{Computed: true},
			"serial":          dschema.StringAttribute{Computed: true},
			"product_id":      dschema.StringAttribute{Computed: true},
			"vendor":          dschema.StringAttribute{Computed: true},
			"locked":          dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *ISCSIExtentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ISCSIExtentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ISCSIExtentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: iscsi.extent.query([["name","=","<name>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "iscsi.extent.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query iSCSI extents failed", err.Error())
		return
	}

	var results []extentAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"iSCSI extent not found",
			fmt.Sprintf("No iSCSI extent named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
