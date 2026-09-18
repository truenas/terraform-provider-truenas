// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &AlertServiceDataSource{}

// AlertServiceDataSource implements the truenas_alert_service data source.
type AlertServiceDataSource struct{ client *client.Client }

// NewDataSource returns a new AlertServiceDataSource.
func NewDataSource() datasource.DataSource { return &AlertServiceDataSource{} }

func (d *AlertServiceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_service"
}

func (d *AlertServiceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS alert service by name.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric alert service ID."},
			"name":    dschema.StringAttribute{Required: true, Description: "Name of the alert service to look up."},
			"level":   dschema.StringAttribute{Computed: true, Description: "Minimum alert level that triggers this service."},
			"enabled": dschema.BoolAttribute{Computed: true, Description: "Whether the alert service is enabled."},
			"attributes": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "JSON document of service settings, as returned by the API.",
			},
		},
	}
}

func (d *AlertServiceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AlertServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state AlertServiceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: alertservice.query([["name", "=", "<name>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "alertservice.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query alert service failed", err.Error())
		return
	}

	var results []alertServiceAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Alert service not found",
			fmt.Sprintf("No alert service named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
