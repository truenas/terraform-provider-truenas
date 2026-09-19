// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NTPServerDataSource{}

// NTPServerDataSource implements the truenas_ntp_server data source.
type NTPServerDataSource struct{ client *client.Client }

// NewDataSource returns a new NTPServerDataSource.
func NewDataSource() datasource.DataSource { return &NTPServerDataSource{} }

func (d *NTPServerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ntp_server"
}

func (d *NTPServerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NTP server by address.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric NTP server ID assigned by TrueNAS."},
			"address": dschema.StringAttribute{Required: true, Description: "Hostname or IP address of the NTP server to look up."},
			"burst":   dschema.BoolAttribute{Computed: true},
			"iburst":  dschema.BoolAttribute{Computed: true},
			"prefer":  dschema.BoolAttribute{Computed: true},
			"minpoll": dschema.Int64Attribute{Computed: true},
			"maxpoll": dschema.Int64Attribute{Computed: true},
		},
	}
}

func (d *NTPServerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NTPServerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NTPServerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queryFilters := []any{[]any{"address", "=", state.Address.ValueString()}}
	raw, err := d.client.CallRead(ctx, "system.ntpserver.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query NTP servers failed", err.Error())
		return
	}

	var results []ntpServerAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"NTP server not found",
			fmt.Sprintf("No NTP server with address %q was found.", state.Address.ValueString()),
		)
		return
	}

	responseToDataSourceModel(&results[0], &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
