// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SMBConfigDataSource{}

// SMBConfigDataSource implements the truenas_smb_config data source.
type SMBConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new SMBConfigDataSource.
func NewDataSource() datasource.DataSource { return &SMBConfigDataSource{} }

func (d *SMBConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smb_config"
}

func (d *SMBConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS SMB service configuration. Takes no arguments: there " +
			"is exactly one SMB configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"smb_config\".",
			},
			"netbiosname": dschema.StringAttribute{
				Computed:    true,
				Description: "NetBIOS name of the server.",
			},
			"netbiosalias": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "NetBIOS aliases for the server.",
			},
			"workgroup": dschema.StringAttribute{
				Computed:    true,
				Description: "Workgroup name.",
			},
			"description": dschema.StringAttribute{
				Computed:    true,
				Description: "Server description string, shown to SMB clients.",
			},
			"unixcharset": dschema.StringAttribute{
				Computed:    true,
				Description: "UNIX character set.",
			},
			"localmaster": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the server participates in local master browser elections.",
			},
			"syslog": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether SMB logging is also written to syslog.",
			},
			"aapl_extensions": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether Apple SMB2/3 protocol extensions are enabled.",
			},
			"admin_group": dschema.StringAttribute{
				Computed:    true,
				Description: "Group whose members are granted SMB admin (root-equivalent) privileges. Empty string means no admin group is set.",
			},
			"guest": dschema.StringAttribute{
				Computed:    true,
				Description: "Account used for guest access.",
			},
			"filemask": dschema.StringAttribute{
				Computed:    true,
				Description: "Default file creation mask. \"DEFAULT\" uses the built-in default.",
			},
			"dirmask": dschema.StringAttribute{
				Computed:    true,
				Description: "Default directory creation mask. \"DEFAULT\" uses the built-in default.",
			},
			"ntlmv1_auth": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the insecure NTLMv1 authentication protocol is allowed.",
			},
			"multichannel": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether SMB multichannel support is enabled.",
			},
			"encryption": dschema.StringAttribute{
				Computed:    true,
				Description: "SMB encryption requirement. One of DEFAULT, NEGOTIATE, DESIRED, REQUIRED.",
			},
			"bindip": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses to bind the SMB service to. Empty list binds to all addresses.",
			},
			"smb_options": dschema.StringAttribute{
				Computed:    true,
				Description: "Additional smb.conf options, appended verbatim.",
			},
			"debug": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether verbose SMB debug logging is enabled.",
			},
			"stateful_failover": dschema.BoolAttribute{
				Computed: true,
				Description: "Whether stateful SMB failover support is enabled. Reports false on TrueNAS " +
					"releases below 26.0 (the field does not exist there); writable (via the truenas_smb_config " +
					"resource) only on TrueNAS 26.0 and later.",
			},
			"minimum_protocol": dschema.StringAttribute{
				Computed: true,
				Description: "Minimum SMB protocol version accepted. One of SMB1, SMB2, SMB3. Reports an empty " +
					"string on TrueNAS releases below 26.0 (the field does not exist there); writable (via " +
					"the truenas_smb_config resource) only on TrueNAS 26.0 and later.",
			},
			"search_protocols": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional network protocols used for server discovery (e.g. WSD, NSD). Reports " +
					"an empty list on TrueNAS releases below 26.0 (the field does not exist there); " +
					"writable (via the truenas_smb_config resource) only on TrueNAS 26.0 and later.",
			},
			"server_sid": dschema.StringAttribute{
				Computed:    true,
				Description: "Server SID (security identifier). Server-assigned and stable.",
			},
		},
	}
}

func (d *SMBConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SMBConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SMBConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "smb.config")
	if err != nil {
		resp.Diagnostics.AddError("Read SMB configuration failed", err.Error())
		return
	}

	var api smbConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse smb.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
