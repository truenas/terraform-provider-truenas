// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &UPSConfigDataSource{}

// UPSConfigDataSource implements the truenas_ups_config data source.
type UPSConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new UPSConfigDataSource.
func NewDataSource() datasource.DataSource { return &UPSConfigDataSource{} }

func (d *UPSConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ups_config"
}

func (d *UPSConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS UPS service configuration. Takes no arguments: there " +
			"is exactly one UPS configuration per TrueNAS system. Does not expose monpwd: ups.config never " +
			"returns a usable value for it.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"ups_config\".",
			},
			"identifier": dschema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the UPS.",
			},
			"mode": dschema.StringAttribute{
				Computed:    true,
				Description: "UPS mode: MASTER or SLAVE.",
			},
			"remotehost": dschema.StringAttribute{
				Computed:    true,
				Description: "Hostname or IP address of the remote UPS (SLAVE mode).",
			},
			"remoteport": dschema.Int64Attribute{
				Computed:    true,
				Description: "Port of the remote UPS (SLAVE mode).",
			},
			"driver": dschema.StringAttribute{
				Computed:    true,
				Description: "UPS driver to use.",
			},
			"port": dschema.StringAttribute{
				Computed:    true,
				Description: "Port or address the UPS driver connects to.",
			},
			"options": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra ups.conf options, appended verbatim.",
			},
			"optionsupsd": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra upsd.conf options, appended verbatim.",
			},
			"description": dschema.StringAttribute{
				Computed:    true,
				Description: "Description of the UPS configuration.",
			},
			"shutdown": dschema.StringAttribute{
				Computed:    true,
				Description: "Condition triggering shutdown: LOWBATT or BATT.",
			},
			"shutdowntimer": dschema.Int64Attribute{
				Computed:    true,
				Description: "Seconds on battery before shutdown when shutdown is BATT.",
			},
			"shutdowncmd": dschema.StringAttribute{
				Computed:    true,
				Description: "Command executed to shut down the system, or \"\" for the default.",
			},
			"monuser": dschema.StringAttribute{
				Computed:    true,
				Description: "Username used to monitor the UPS.",
			},
			"extrausers": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra users allowed to monitor the UPS, appended to upsd.users.",
			},
			"rmonitor": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether remote monitoring is enabled.",
			},
			"powerdown": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the UPS is powered down after shutdown.",
			},
			"hostsync": dschema.Int64Attribute{
				Computed:    true,
				Description: "Seconds to wait for the master to shut down before a slave shuts down.",
			},
			"nocommwarntime": dschema.Int64Attribute{
				Computed:    true,
				Description: "Seconds without UPS communication before a warning is raised, or 0 for the default.",
			},
			"complete_identifier": dschema.StringAttribute{
				Computed:    true,
				Description: "Server-derived, fully-qualified UPS identifier.",
			},
		},
	}
}

func (d *UPSConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UPSConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state UPSConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "ups.config")
	if err != nil {
		resp.Diagnostics.AddError("Read UPS configuration failed", err.Error())
		return
	}

	var api upsConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse ups.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
