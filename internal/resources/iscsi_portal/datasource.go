// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ISCSIPortalDataSource{}

// ISCSIPortalDataSourceModel is the Terraform state model for the
// truenas_iscsi_portal data source. It is a distinct struct from
// ISCSIPortalModel (even though the fields currently match) so that
// resource and data source schemas/models can diverge independently.
type ISCSIPortalDataSourceModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Comment types.String `tfsdk:"comment"`
	Listen  types.List   `tfsdk:"listen"`
	Tag     types.Int64  `tfsdk:"tag"`
}

// responseToDataSourceModel maps an API response onto the data source model.
func responseToDataSourceModel(ctx context.Context, api *portalAPI, m *ISCSIPortalDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Comment = types.StringValue(api.Comment)
	m.Tag = types.Int64Value(api.Tag)

	var listenModels []ListenModel
	for _, l := range api.Listen {
		listenModels = append(listenModels, ListenModel{
			IP:   types.StringValue(l.IP),
			Port: types.Int64Value(l.Port),
		})
	}
	if listenModels == nil {
		listenModels = []ListenModel{}
	}
	listenList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: listenAttrTypes}, listenModels)
	diags.Append(d...)
	m.Listen = listenList

	return diags
}

// ISCSIPortalDataSource implements the truenas_iscsi_portal data source.
type ISCSIPortalDataSource struct{ client *client.Client }

// NewDataSource returns a new ISCSIPortalDataSource.
func NewDataSource() datasource.DataSource { return &ISCSIPortalDataSource{} }

func (d *ISCSIPortalDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_portal"
}

func (d *ISCSIPortalDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS iSCSI portal by comment.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric iSCSI portal ID."},
			"comment": dschema.StringAttribute{Required: true, Description: "Comment used to look up the portal."},
			"tag":     dschema.Int64Attribute{Computed: true, Description: "Portal group tag assigned by TrueNAS."},
			"listen": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of IP/port pairs the portal listens on.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"ip":   dschema.StringAttribute{Computed: true},
						"port": dschema.Int64Attribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ISCSIPortalDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ISCSIPortalDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ISCSIPortalDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryFilters := []any{[]any{"comment", "=", state.Comment.ValueString()}}
	raw, err := d.client.CallRead(ctx, "iscsi.portal.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query iSCSI portals failed", err.Error())
		return
	}

	var results []portalAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"iSCSI portal not found",
			fmt.Sprintf("No iSCSI portal with comment %q was found.", state.Comment.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
