// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &StaticRouteDataSource{}

// StaticRouteDataSource implements the truenas_static_route data source.
type StaticRouteDataSource struct{ client *client.Client }

// NewDataSource returns a new StaticRouteDataSource.
func NewDataSource() datasource.DataSource { return &StaticRouteDataSource{} }

func (d *StaticRouteDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_static_route"
}

func (d *StaticRouteDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS static route by destination.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.Int64Attribute{Computed: true, Description: "Numeric static route ID assigned by TrueNAS."},
			"destination": dschema.StringAttribute{Required: true, Description: "Destination network in CIDR notation to look up, e.g. \"10.20.0.0/16\"."},
			"gateway":     dschema.StringAttribute{Computed: true},
			"description": dschema.StringAttribute{Computed: true},
		},
	}
}

func (d *StaticRouteDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *StaticRouteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state StaticRouteModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryFilters := []any{[]any{"destination", "=", state.Destination.ValueString()}}
	raw, err := d.client.CallRead(ctx, "staticroute.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query static routes failed", err.Error())
		return
	}

	var results []staticRouteAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Static route not found",
			fmt.Sprintf("No static route with destination %q was found.", state.Destination.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
