// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AppRegistryDataSource{}

// AppRegistryDataSource implements the truenas_app_registry data source.
//
// This datasource never exposes the registry password: its schema and
// model have no "password" attribute (mirrors internal/resources/iscsi_auth's
// datasource, which omits "secret"/"peersecret" for the same reason).
type AppRegistryDataSource struct{ client *client.Client }

// NewDataSource returns a new AppRegistryDataSource.
func NewDataSource() datasource.DataSource { return &AppRegistryDataSource{} }

func (d *AppRegistryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_registry"
}

func (d *AppRegistryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Looks up a TrueNAS app container registry entry by name. Never exposes the registry " +
			"password: this datasource has no \"password\" attribute.",
		Attributes: map[string]dschema.Attribute{
			"id":          dschema.Int64Attribute{Computed: true, Description: "Numeric app registry entry ID assigned by TrueNAS."},
			"name":        dschema.StringAttribute{Required: true, Description: "Name of the container registry to look up."},
			"description": dschema.StringAttribute{Computed: true, Description: "Description of the container registry."},
			"uri":         dschema.StringAttribute{Computed: true, Description: "Container registry URI endpoint."},
			"username":    dschema.StringAttribute{Computed: true, Description: "Username for registry authentication."},
		},
	}
}

func (d *AppRegistryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AppRegistryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AppRegistryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "app.registry.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query app registry entries failed", err.Error())
		return
	}

	var results []appRegistryAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"App registry entry not found",
			fmt.Sprintf("No app registry entry with name %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
