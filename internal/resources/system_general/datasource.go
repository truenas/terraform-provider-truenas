// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SystemGeneralDataSource{}

// SystemGeneralDataSource implements the truenas_system_general data
// source.
type SystemGeneralDataSource struct{ client *client.Client }

// NewDataSource returns a new SystemGeneralDataSource.
func NewDataSource() datasource.DataSource { return &SystemGeneralDataSource{} }

func (d *SystemGeneralDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_general"
}

func (d *SystemGeneralDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS system general configuration (management UI, timezone, " +
			"keyboard map, usage collection). Takes no arguments: there is exactly one system general " +
			"configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"system_general\".",
			},
			"ds_auth": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether directory service credentials may be used to log in to the web UI.",
			},
			"kbdmap": dschema.StringAttribute{
				Computed:    true,
				Description: "System console keyboard layout/map.",
			},
			"timezone": dschema.StringAttribute{
				Computed:    true,
				Description: "System timezone (e.g. \"America/Los_Angeles\").",
			},
			"ui_address": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IPv4 addresses the web UI binds to.",
			},
			"ui_allowlist": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses/networks allowed to access the web UI. Empty list allows all.",
			},
			"ui_certificate": dschema.Int64Attribute{
				Computed:    true,
				Description: "Numeric ID of the certificate used by the web UI for HTTPS.",
			},
			"ui_consolemsg": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether console messages are shown on the login screen.",
			},
			"ui_httpsport": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP port the web UI listens on for HTTPS.",
			},
			"ui_httpsprotocols": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "TLS protocol versions accepted by the web UI (e.g. TLSv1.2, TLSv1.3).",
			},
			"ui_httpsredirect": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether HTTP requests to the web UI are redirected to HTTPS.",
			},
			"ui_port": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP port the web UI listens on for HTTP.",
			},
			"ui_v6address": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IPv6 addresses the web UI binds to.",
			},
			"ui_x_frame_options": dschema.StringAttribute{
				Computed:    true,
				Description: "X-Frame-Options header value sent by the web UI. One of SAMEORIGIN, DENY, ALLOW_ALL.",
			},
			"usage_collection": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether anonymous usage statistics are collected.",
			},
			"ui_certificate_name": dschema.StringAttribute{
				Computed:    true,
				Description: "Name of the certificate currently assigned to the web UI.",
			},
			"usage_collection_is_set": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether usage_collection has been explicitly set by an administrator.",
			},
			"wizardshown": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the initial setup wizard has been shown.",
			},
		},
	}
}

func (d *SystemGeneralDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SystemGeneralDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SystemGeneralDataSourceModel

	raw, err := d.client.CallRead(ctx, "system.general.config")
	if err != nil {
		resp.Diagnostics.AddError("Read system general configuration failed", err.Error())
		return
	}

	var api systemGeneralAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse system.general.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
