// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SMBShareDataSource{}

// SMBShareDataSource implements the truenas_smb_share data source.
type SMBShareDataSource struct{ client *client.Client }

// NewDataSource returns a new SMBShareDataSource.
func NewDataSource() datasource.DataSource { return &SMBShareDataSource{} }

func (d *SMBShareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smb_share"
}

func (d *SMBShareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS SMB share by name.",
		Attributes: map[string]dschema.Attribute{
			"id":                dschema.Int64Attribute{Computed: true, Description: "Numeric SMB share ID."},
			"path":              dschema.StringAttribute{Computed: true},
			"name":              dschema.StringAttribute{Required: true, Description: "SMB share name to look up."},
			"comment":           dschema.StringAttribute{Computed: true},
			"ro":                dschema.BoolAttribute{Computed: true},
			"browsable":         dschema.BoolAttribute{Computed: true},
			"recyclebin":        dschema.BoolAttribute{Computed: true},
			"guestok":           dschema.BoolAttribute{Computed: true},
			"hostsallow":        dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"hostsdeny":         dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"abe":               dschema.BoolAttribute{Computed: true},
			"acl":               dschema.BoolAttribute{Computed: true},
			"durablehandle":     dschema.BoolAttribute{Computed: true},
			"streams":           dschema.BoolAttribute{Computed: true},
			"timemachine":       dschema.BoolAttribute{Computed: true},
			"timemachine_quota": dschema.Int64Attribute{Computed: true},
			"enabled":           dschema.BoolAttribute{Computed: true},
			"home":              dschema.BoolAttribute{Computed: true},
			"purpose":           dschema.StringAttribute{Computed: true},
			"audit": dschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dschema.Attribute{
					"enable":      dschema.BoolAttribute{Computed: true},
					"watch_list":  dschema.ListAttribute{Computed: true, ElementType: types.StringType},
					"ignore_list": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
				},
			},
			"vuid":   dschema.StringAttribute{Computed: true},
			"locked": dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *SMBShareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SMBShareDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SMBModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: sharing.smb.query([["name", "=", "<sharename>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "sharing.smb.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query SMB shares failed", err.Error())
		return
	}

	var results []smbAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"SMB share not found",
			fmt.Sprintf("No SMB share named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
