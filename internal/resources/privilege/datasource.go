// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &PrivilegeDataSource{}

// PrivilegeDataSource implements the truenas_privilege data source.
type PrivilegeDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of PrivilegeDataSource.
func NewDataSource() datasource.DataSource { return &PrivilegeDataSource{} }

func (d *PrivilegeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_privilege"
}

func (d *PrivilegeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS privilege by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the privilege."},
			"name": dschema.StringAttribute{Required: true, Description: "Privilege name to look up."},
			"local_groups": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "GIDs of local groups whose members gain this privilege.",
			},
			"ds_groups": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "GIDs of directory-service groups whose members gain this privilege.",
			},
			"roles": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Role names included in this privilege.",
			},
			"web_shell": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether members of the assigned groups may log in to the web shell.",
			},
			"builtin_name": dschema.StringAttribute{
				Computed: true,
				Description: "Internal name of the built-in privilege if this is a system privilege " +
					"(e.g. \"LOCAL_ADMINISTRATOR\"); null for custom privileges.",
			},
		},
	}
}

func (d *PrivilegeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PrivilegeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state PrivilegeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "privilege.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query privileges failed", err.Error())
		return
	}

	var results []privilegeAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Privilege not found",
			fmt.Sprintf("no privilege found with name %q", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
