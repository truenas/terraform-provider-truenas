// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &APIKeyDataSource{}

// APIKeyDataSource implements the truenas_api_key data source.
type APIKeyDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of APIKeyDataSource.
func NewDataSource() datasource.DataSource { return &APIKeyDataSource{} }

func (d *APIKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *APIKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS API key by name. Never exposes the plaintext key value: " +
			"api_key.query does not return it, so there is nothing for this data source to surface " +
			"(see the truenas_api_key resource for the create-time-only key attribute).",
		Attributes: map[string]dschema.Attribute{
			"id":         dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the API key."},
			"name":       dschema.StringAttribute{Required: true, Description: "API key name to look up."},
			"username":   dschema.StringAttribute{Computed: true, Description: "Local username that owns this API key."},
			"expires_at": dschema.StringAttribute{Computed: true, Description: "RFC3339 UTC expiration timestamp, or null if the key never expires."},
			"created_at": dschema.StringAttribute{Computed: true, Description: "RFC3339 UTC timestamp of when the API key was created."},
			"local":      dschema.BoolAttribute{Computed: true, Description: "Whether this API key is for local system use only."},
			"revoked":    dschema.BoolAttribute{Computed: true, Description: "Whether the API key has been revoked and is no longer valid."},
		},
	}
}

func (d *APIKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *APIKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state APIKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "api_key.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query API keys failed", err.Error())
		return
	}

	var results []apiKeyAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"API key not found",
			fmt.Sprintf("no API key found with name %q", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
