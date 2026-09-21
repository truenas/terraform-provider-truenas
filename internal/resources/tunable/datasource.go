// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &TunableDataSource{}

// TunableDataSource implements the truenas_tunable data source.
type TunableDataSource struct{ client *client.Client }

// NewDataSource returns a new TunableDataSource.
func NewDataSource() datasource.DataSource { return &TunableDataSource{} }

func (d *TunableDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tunable"
}

func (d *TunableDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS tunable by variable name.",
		Attributes: map[string]dschema.Attribute{
			"id":         dschema.Int64Attribute{Computed: true, Description: "Numeric tunable ID."},
			"var":        dschema.StringAttribute{Required: true, Description: "Name of the tunable variable to look up."},
			"value":      dschema.StringAttribute{Computed: true},
			"type":       dschema.StringAttribute{Computed: true},
			"comment":    dschema.StringAttribute{Computed: true},
			"enabled":    dschema.BoolAttribute{Computed: true},
			"orig_value": dschema.StringAttribute{Computed: true},
		},
	}
}

func (d *TunableDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TunableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state TunableDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by var: tunable.query([["var", "=", "<name>"]])
	queryFilters := []any{[]any{"var", "=", state.Var.ValueString()}}
	raw, err := d.client.CallRead(ctx, "tunable.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query tunables failed", err.Error())
		return
	}

	var results []tunableAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Tunable not found",
			fmt.Sprintf("No tunable named %q was found.", state.Var.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
