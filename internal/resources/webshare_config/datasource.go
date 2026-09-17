// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &WebshareConfigDataSource{}

// WebshareConfigDataSource implements the truenas_webshare_config data
// source.
type WebshareConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new WebshareConfigDataSource.
func NewDataSource() datasource.DataSource { return &WebshareConfigDataSource{} }

func (d *WebshareConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webshare_config"
}

func (d *WebshareConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS Webshare service configuration. Takes no arguments: there " +
			"is exactly one Webshare configuration per TrueNAS system. Requires TrueNAS 26.0 or later (see " +
			"the truenas_webshare_config resource's schema description for the version-gate details).",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"webshare_config\".",
			},
			"bindip": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of IP addresses the Webshare HTTP server binds to. An empty list means it " +
					"binds to all addresses.",
			},
			"search": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether search indexing is enabled for Webshare-exposed content.",
			},
			"passkey": dschema.StringAttribute{
				Computed:    true,
				Description: "Passkey authentication mode: ENABLED, DISABLED, or REQUIRED.",
			},
			"groups": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of AD/LDAP group names whose members are granted access to Webshare.",
			},
		},
	}
}

func (d *WebshareConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WebshareConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state WebshareConfigDataSourceModel

	version, err := d.client.ServerVersion(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Version probe failed", err.Error())
		return
	}
	resp.Diagnostics.Append(versionGateDiagnostics(version)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "webshare.config")
	if err != nil {
		resp.Diagnostics.AddError("Read Webshare configuration failed", err.Error())
		return
	}

	var api webshareConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse webshare.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
