// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &CredentialsDataSource{}

// CredentialsDataSource implements the truenas_cloudsync_credentials data
// source.
type CredentialsDataSource struct{ client *client.Client }

// NewDataSource returns a new CredentialsDataSource.
func NewDataSource() datasource.DataSource { return &CredentialsDataSource{} }

func (d *CredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudsync_credentials"
}

func (d *CredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches TrueNAS cloud sync credentials by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric cloud sync credentials ID."},
			"name": dschema.StringAttribute{Required: true, Description: "Name of the cloud sync credentials to look up."},
			"provider_config": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "JSON document of provider settings, as returned by the API.",
			},
		},
	}
}

func (d *CredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CredentialsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: cloudsync.credentials.query([["name", "=", "<name>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "cloudsync.credentials.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query cloudsync credentials failed", err.Error())
		return
	}

	var results []credentialsAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Cloud sync credentials not found",
			fmt.Sprintf("No cloud sync credentials named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
