// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &TwoFactorAuthDataSource{}

// TwoFactorAuthDataSource implements the truenas_twofactor_auth data
// source.
type TwoFactorAuthDataSource struct{ client *client.Client }

// NewDataSource returns a new TwoFactorAuthDataSource.
func NewDataSource() datasource.DataSource { return &TwoFactorAuthDataSource{} }

func (d *TwoFactorAuthDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_twofactor_auth"
}

func (d *TwoFactorAuthDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS two-factor authentication configuration. Takes no " +
			"arguments: there is exactly one two-factor authentication configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"twofactor_auth\".",
			},
			"enabled": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether two-factor authentication is enabled system-wide.",
			},
			"window": dschema.Int64Attribute{
				Computed:    true,
				Description: "Time window in seconds for TOTP token validation.",
			},
			"services": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Configuration for which services require two-factor authentication.",
				Attributes: map[string]dschema.Attribute{
					"ssh": dschema.BoolAttribute{
						Computed:    true,
						Description: "Whether two-factor authentication is required for SSH connections.",
					},
				},
			},
		},
	}
}

func (d *TwoFactorAuthDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TwoFactorAuthDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state TwoFactorAuthDataSourceModel

	raw, err := d.client.CallRead(ctx, "auth.twofactor.config")
	if err != nil {
		resp.Diagnostics.AddError("Read two-factor authentication configuration failed", err.Error())
		return
	}

	var api twoFactorAuthAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse auth.twofactor.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
