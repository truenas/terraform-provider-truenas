// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &TnConnectConfigDataSource{}

// TnConnectConfigDataSource implements the truenas_tn_connect_config data
// source.
type TnConnectConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new TnConnectConfigDataSource.
func NewDataSource() datasource.DataSource { return &TnConnectConfigDataSource{} }

func (d *TnConnectConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tn_connect_config"
}

func (d *TnConnectConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS Connect service configuration and status. Takes " +
			"no arguments: there is exactly one TrueNAS Connect configuration per TrueNAS system. Makes no " +
			"changes — see the truenas_tn_connect_config resource's schema description for the safety notes " +
			"around the \"enabled\" field and which attributes are release-specific.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"tn_connect_config\".",
			},
			"enabled": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the TrueNAS Connect cloud service is currently enabled.",
			},
			"status": dschema.StringAttribute{
				Computed:    true,
				Description: "Current operational status of the TrueNAS Connect service (e.g. DISABLED, CONFIGURED).",
			},
			"status_reason": dschema.StringAttribute{
				Computed:    true,
				Description: "Human-readable explanation of the current status.",
			},
			"certificate": dschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the SSL certificate used for TrueNAS Connect communications. Null when using the default.",
			},
			"account_service_base_url": dschema.StringAttribute{
				Computed:    true,
				Description: "Base URL for the TrueNAS Connect account service API.",
			},
			"leca_service_base_url": dschema.StringAttribute{
				Computed:    true,
				Description: "Base URL for the Let's Encrypt Certificate Authority service used by TrueNAS Connect.",
			},
			"tnc_base_url": dschema.StringAttribute{
				Computed:    true,
				Description: "Base URL for the TrueNAS Connect service.",
			},
			"heartbeat_url": dschema.StringAttribute{
				Computed:    true,
				Description: "URL endpoint for sending heartbeat signals to maintain connection status.",
			},
			"registration_details": dschema.StringAttribute{
				Computed: true,
				Description: "Registration information and credentials for TrueNAS Connect, as a JSON-encoded " +
					"object (\"{}\" when not enrolled).",
			},
			"tier": dschema.StringAttribute{
				Computed: true,
				Description: "TrueNAS Connect tier (FOUNDATION, PLUS, or BUSINESS). Only present on TrueNAS " +
					"26.0+; reads as null on TrueNAS 25.10.",
			},
			"last_heartbeat_failure_datetime": dschema.StringAttribute{
				Computed: true,
				Description: "Datetime the current heartbeat failure streak began, or null if heartbeat is " +
					"not currently failing. Only present on TrueNAS 26.0+; reads as null on TrueNAS 25.10.",
			},
			"ips": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses TrueNAS Connect binds to and advertises. Only present on TrueNAS " +
					"25.10; reads as null on TrueNAS 26.0.",
			},
			"interfaces": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Network interface names TrueNAS Connect uses. Only present on TrueNAS 25.10; " +
					"reads as null on TrueNAS 26.0.",
			},
			"interfaces_ips": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses associated with the selected interfaces. Only present on TrueNAS " +
					"25.10; reads as null on TrueNAS 26.0.",
			},
			"use_all_interfaces": dschema.BoolAttribute{
				Computed: true,
				Description: "Whether TrueNAS Connect automatically uses all available network interfaces. " +
					"Only present on TrueNAS 25.10; reads as null on TrueNAS 26.0.",
			},
		},
	}
}

func (d *TnConnectConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TnConnectConfigDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state TnConnectConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "tn_connect.config")
	if err != nil {
		resp.Diagnostics.AddError("Read TrueNAS Connect configuration failed", err.Error())
		return
	}

	var api tnConnectConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse tn_connect.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
