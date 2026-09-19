// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SNMPConfigDataSource{}

// SNMPConfigDataSource implements the truenas_snmp_config data source.
type SNMPConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new SNMPConfigDataSource.
func NewDataSource() datasource.DataSource { return &SNMPConfigDataSource{} }

func (d *SNMPConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snmp_config"
}

func (d *SNMPConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS SNMP service configuration. Takes no arguments: there " +
			"is exactly one SNMP configuration per TrueNAS system. Does not expose v3_password or " +
			"v3_privpassphrase: snmp.config never returns usable values for them.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"snmp_config\".",
			},
			"community": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "SNMP community string.",
			},
			"contact": dschema.StringAttribute{
				Computed:    true,
				Description: "Contact information for the SNMP administrator.",
			},
			"location": dschema.StringAttribute{
				Computed:    true,
				Description: "Physical location of the system, exposed via SNMP.",
			},
			"loglevel": dschema.Int64Attribute{
				Computed:    true,
				Description: "SNMP daemon syslog level.",
			},
			"options": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra snmpd.conf options, appended verbatim.",
			},
			"traps": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether SNMP traps are enabled.",
			},
			"zilstat": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether zilstat reporting is enabled.",
			},
			"v3": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether SNMPv3 support is enabled.",
			},
			"v3_username": dschema.StringAttribute{
				Computed:    true,
				Description: "SNMPv3 username.",
			},
			"v3_authtype": dschema.StringAttribute{
				Computed:    true,
				Description: "SNMPv3 authentication type: one of \"\" (none), MD5, SHA.",
			},
			"v3_privproto": dschema.StringAttribute{
				Computed:    true,
				Description: "SNMPv3 privacy protocol: one of AES, DES, or \"\" for none.",
			},
		},
	}
}

func (d *SNMPConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SNMPConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SNMPConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "snmp.config")
	if err != nil {
		resp.Diagnostics.AddError("Read SNMP configuration failed", err.Error())
		return
	}

	var api snmpConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse snmp.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
