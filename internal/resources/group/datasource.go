// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &GroupDataSource{}

// GroupDataSource implements the truenas_group data source.
type GroupDataSource struct{ client *client.Client }

// NewDataSource returns a new GroupDataSource.
func NewDataSource() datasource.DataSource { return &GroupDataSource{} }

func (d *GroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *GroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS local group by name.",
		Attributes: map[string]dschema.Attribute{
			"id":                     dschema.Int64Attribute{Computed: true, Description: "Numeric group ID."},
			"gid":                    dschema.Int64Attribute{Computed: true, Description: "UNIX GID."},
			"name":                   dschema.StringAttribute{Required: true, Description: "Group name to look up."},
			"smb":                    dschema.BoolAttribute{Computed: true},
			"sudo_commands":          dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"sudo_commands_nopasswd": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"builtin":                dschema.BoolAttribute{Computed: true},
			"immutable":              dschema.BoolAttribute{Computed: true},
			"local":                  dschema.BoolAttribute{Computed: true},
		},
	}
}

func (d *GroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state GroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by group name: group.query([["group", "=", "<name>"]])
	queryFilters := []any{[]any{"group", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "group.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query groups failed", err.Error())
		return
	}

	var results []groupAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Group not found",
			fmt.Sprintf("No group named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
