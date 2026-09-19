// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &FTPConfigDataSource{}

// FTPConfigDataSource implements the truenas_ftp_config data source.
type FTPConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new FTPConfigDataSource.
func NewDataSource() datasource.DataSource { return &FTPConfigDataSource{} }

func (d *FTPConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ftp_config"
}

func (d *FTPConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS FTP service configuration. Takes no arguments: there " +
			"is exactly one FTP configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"ftp_config\".",
			},
			"port": dschema.Int64Attribute{
				Computed:    true,
				Description: "Port the FTP service listens on.",
			},
			"clients": dschema.Int64Attribute{
				Computed:    true,
				Description: "Maximum number of simultaneous connections.",
			},
			"ipconnections": dschema.Int64Attribute{
				Computed:    true,
				Description: "Maximum number of connections per IP address (0 for unlimited).",
			},
			"loginattempt": dschema.Int64Attribute{
				Computed:    true,
				Description: "Maximum number of allowed login attempts before disconnecting.",
			},
			"timeout": dschema.Int64Attribute{
				Computed:    true,
				Description: "Maximum idle time in seconds before disconnecting.",
			},
			"timeout_notransfer": dschema.Int64Attribute{
				Computed:    true,
				Description: "Maximum time in seconds a client is allowed to spend between transfers.",
			},
			"localuserbw": dschema.Int64Attribute{
				Computed:    true,
				Description: "Local user upload bandwidth in KiB/s (0 for unlimited).",
			},
			"localuserdlbw": dschema.Int64Attribute{
				Computed:    true,
				Description: "Local user download bandwidth in KiB/s (0 for unlimited).",
			},
			"anonuserbw": dschema.Int64Attribute{
				Computed:    true,
				Description: "Anonymous user upload bandwidth in KiB/s (0 for unlimited).",
			},
			"anonuserdlbw": dschema.Int64Attribute{
				Computed:    true,
				Description: "Anonymous user download bandwidth in KiB/s (0 for unlimited).",
			},
			"passiveportsmin": dschema.Int64Attribute{
				Computed:    true,
				Description: "Lower bound of the passive port range (0 for default OS-assigned range).",
			},
			"passiveportsmax": dschema.Int64Attribute{
				Computed:    true,
				Description: "Upper bound of the passive port range (0 for default OS-assigned range).",
			},
			"defaultroot": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether local users are restricted to their home directory by default.",
			},
			"onlyanonymous": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether only anonymous logins are allowed.",
			},
			"onlylocal": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether only local user logins are allowed (no anonymous access).",
			},
			"ident": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether IDENT authentication is enabled.",
			},
			"fxp": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether File eXchange Protocol (server-to-server transfers) is enabled.",
			},
			"resume": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether resuming interrupted transfers is allowed.",
			},
			"reversedns": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether reverse DNS lookups are performed on client connections.",
			},
			"tls": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether TLS/SSL is enabled for FTP (FTPS).",
			},
			"tls_opt_allow_client_renegotiations": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: allow client renegotiations.",
			},
			"tls_opt_allow_dot_login": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: allow dot login.",
			},
			"tls_opt_allow_per_user": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: allow per-user certificates.",
			},
			"tls_opt_common_name_required": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: require a common name in the client certificate.",
			},
			"tls_opt_dns_name_required": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: require a DNS name in the client certificate.",
			},
			"tls_opt_enable_diags": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: enable diagnostic logging.",
			},
			"tls_opt_export_cert_data": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: export certificate data to the environment.",
			},
			"tls_opt_ip_address_required": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: require the IP address in the client certificate to match.",
			},
			"tls_opt_no_empty_fragments": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: disable empty fragments (workaround for buggy clients).",
			},
			"tls_opt_no_session_reuse_required": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: require a new SSL session on data connections.",
			},
			"tls_opt_stdenvvars": dschema.BoolAttribute{
				Computed:    true,
				Description: "TLS option: export standard environment variables.",
			},
			"masqaddress": dschema.StringAttribute{
				Computed:    true,
				Description: "Public IP address or hostname to masquerade as for passive connections behind NAT.",
			},
			"banner": dschema.StringAttribute{
				Computed:    true,
				Description: "Banner text displayed to clients on connection.",
			},
			"options": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra proftpd.conf options, appended verbatim.",
			},
			"dirmask": dschema.StringAttribute{
				Computed:    true,
				Description: "Octal umask applied to directories created via FTP.",
			},
			"filemask": dschema.StringAttribute{
				Computed:    true,
				Description: "Octal umask applied to files created via FTP.",
			},
			"tls_policy": dschema.StringAttribute{
				Computed:    true,
				Description: "TLS policy. One of: on, off, data, !data, auth, ctrl+data, ctrl+!data, auth+data, auth+!data.",
			},
			"ssltls_certificate": dschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the certificate used for TLS/SSL. 0 means no certificate is set (API null).",
			},
			"anonpath": dschema.StringAttribute{
				Computed:    true,
				Description: "Path exposed to anonymous FTP users. \"\" means unset (API null).",
			},
		},
	}
}

func (d *FTPConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FTPConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state FTPConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "ftp.config")
	if err != nil {
		resp.Diagnostics.AddError("Read FTP configuration failed", err.Error())
		return
	}

	var api ftpConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse ftp.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
