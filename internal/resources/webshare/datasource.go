// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &WebshareDataSource{}

// WebshareDataSource implements the truenas_webshare data source.
type WebshareDataSource struct{ client *client.Client }

// NewDataSource returns a new WebshareDataSource.
func NewDataSource() datasource.DataSource { return &WebshareDataSource{} }

func (d *WebshareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webshare"
}

func (d *WebshareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS Webshare share by name. Requires TrueNAS 26.0 or later (see the " +
			"truenas_webshare resource's schema description for the version-gate details).",
		Attributes: map[string]dschema.Attribute{
			"id":            dschema.Int64Attribute{Computed: true, Description: "Numeric Webshare share ID."},
			"name":          dschema.StringAttribute{Required: true, Description: "Webshare share name to look up."},
			"path":          dschema.StringAttribute{Computed: true, Description: "Local server path being shared."},
			"enabled":       dschema.BoolAttribute{Computed: true, Description: "Whether the share is available."},
			"is_home_base":  dschema.BoolAttribute{Computed: true, Description: "Whether this share is the user home-directory base."},
			"dataset":       dschema.StringAttribute{Computed: true, Description: "Dataset name component of path, or null if unresolved."},
			"relative_path": dschema.StringAttribute{Computed: true, Description: "Relative path component within the dataset, or null if unresolved."},
			"locked":        dschema.BoolAttribute{Computed: true, Description: "Whether the share path is on a locked dataset, or null if unavailable."},
		},
	}
}

func (d *WebshareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WebshareDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state WebshareDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Version probe failed", err.Error())
		return
	}
	resp.Diagnostics.Append(versionGateDiagnostics(version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: sharing.webshare.query([["name", "=", "<sharename>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "sharing.webshare.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query Webshare shares failed", err.Error())
		return
	}

	var results []webshareAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Webshare share not found",
			fmt.Sprintf("No Webshare share named %q was found.", state.Name.ValueString()),
		)
		return
	}

	responseToDataSourceModel(&results[0], &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
