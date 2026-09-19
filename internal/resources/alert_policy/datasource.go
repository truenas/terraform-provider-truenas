// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AlertPolicyDataSource{}

// AlertPolicyDataSource implements the truenas_alert_policy data source.
type AlertPolicyDataSource struct{ client *client.Client }

// NewDataSource returns a new AlertPolicyDataSource.
func NewDataSource() datasource.DataSource { return &AlertPolicyDataSource{} }

func (d *AlertPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_policy"
}

func (d *AlertPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS global alert policy (per-alert-class notification level and delivery policy overrides). Takes no arguments: there is exactly one alert policy per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"alert_policy\".",
			},
			"classes": dschema.StringAttribute{
				Computed:    true,
				Description: "JSON object of current alert class overrides, as returned by the API.",
			},
		},
	}
}

func (d *AlertPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AlertPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AlertPolicyDataSourceModel

	raw, err := d.client.CallRead(ctx, "alertclasses.config")
	if err != nil {
		resp.Diagnostics.AddError("Read alert policy failed", err.Error())
		return
	}

	var api alertClassesAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse alertclasses.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
