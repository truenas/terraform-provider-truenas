// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &ReportingExporterDataSource{}

// ReportingExporterDataSource implements the truenas_reporting_exporter data source.
type ReportingExporterDataSource struct{ client *client.Client }

// NewDataSource returns a new ReportingExporterDataSource.
func NewDataSource() datasource.DataSource { return &ReportingExporterDataSource{} }

func (d *ReportingExporterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reporting_exporter"
}

func (d *ReportingExporterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS reporting exporter by name.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric reporting exporter ID."},
			"name":    dschema.StringAttribute{Required: true, Description: "Name of the reporting exporter to look up."},
			"enabled": dschema.BoolAttribute{Computed: true, Description: "Whether this exporter is enabled and active."},
			"attributes": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "GRAPHITE exporter settings (the only exporter type TrueNAS currently supports).",
				Attributes: map[string]dschema.Attribute{
					"destination_ip":            dschema.StringAttribute{Computed: true, Description: "IP address of the Graphite server."},
					"destination_port":          dschema.Int64Attribute{Computed: true, Description: "Port number of the Graphite server."},
					"namespace":                 dschema.StringAttribute{Computed: true, Description: "Namespace to organize metrics under."},
					"prefix":                    dschema.StringAttribute{Computed: true, Description: "Prefix to prepend to all metric names."},
					"update_every":              dschema.Int64Attribute{Computed: true, Description: "Interval in seconds between metric updates."},
					"buffer_on_failures":        dschema.Int64Attribute{Computed: true, Description: "Number of updates to buffer when the Graphite server is unavailable."},
					"send_names_instead_of_ids": dschema.BoolAttribute{Computed: true, Description: "Whether to send human-readable names instead of internal IDs."},
					"matching_charts":           dschema.StringAttribute{Computed: true, Description: "Pattern to match charts for export (supports wildcards)."},
				},
			},
		},
	}
}

func (d *ReportingExporterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReportingExporterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ReportingExporterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "reporting.exporters.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query reporting exporters failed", err.Error())
		return
	}

	var results []reportingExporterAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Reporting exporter not found",
			fmt.Sprintf("No reporting exporter named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
