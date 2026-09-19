// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NFSShareDataSource{}

type NFSShareDataSource struct{ client *client.Client }

func NewDataSource() datasource.DataSource { return &NFSShareDataSource{} }

func (d *NFSShareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nfs_share"
}

func (d *NFSShareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NFS share by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":               dschema.StringAttribute{Required: true, Description: "NFS share ID."},
			"path":             dschema.StringAttribute{Computed: true},
			"comment":          dschema.StringAttribute{Computed: true},
			"enabled":          dschema.BoolAttribute{Computed: true},
			"ro":               dschema.BoolAttribute{Computed: true},
			"networks":         dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"hosts":            dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"maproot_user":     dschema.StringAttribute{Computed: true},
			"maproot_group":    dschema.StringAttribute{Computed: true},
			"mapall_user":      dschema.StringAttribute{Computed: true},
			"mapall_group":     dschema.StringAttribute{Computed: true},
			"security":         dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"expose_snapshots": dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *NFSShareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NFSShareDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NFSShareModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", state.ID.ValueString())
		return
	}

	raw, err := d.client.CallRead(ctx, "sharing.nfs.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Read NFS share failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	r := &NFSShareResource{}
	resp.Diagnostics.Append(r.responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
