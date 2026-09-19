// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AcmeDnsAuthenticatorDataSource{}

// AcmeDnsAuthenticatorDataSource implements the truenas_acme_dns_authenticator data source.
type AcmeDnsAuthenticatorDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of AcmeDnsAuthenticatorDataSource.
func NewDataSource() datasource.DataSource { return &AcmeDnsAuthenticatorDataSource{} }

func (d *AcmeDnsAuthenticatorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acme_dns_authenticator"
}

func (d *AcmeDnsAuthenticatorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS ACME DNS authenticator by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the ACME DNS authenticator."},
			"name": dschema.StringAttribute{Required: true, Description: "Name to look up."},
			"attributes": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "JSON document of DNS provider credentials/config, as returned by the API.",
			},
		},
	}
}

func (d *AcmeDnsAuthenticatorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AcmeDnsAuthenticatorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AcmeDnsAuthenticatorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "acme.dns.authenticator.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query ACME DNS authenticators failed", err.Error())
		return
	}

	var results []acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"ACME DNS authenticator not found",
			fmt.Sprintf("no ACME DNS authenticator found with name %q", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
