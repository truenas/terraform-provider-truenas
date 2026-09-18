// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS FTP service configuration. This is a singleton resource — " +
			"there is exactly one FTP configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls ftp.update, and Terraform delete only removes the resource " +
			"from state (the configuration is left in place).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"ftp_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"port": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Port the FTP service listens on.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"clients": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maximum number of simultaneous connections.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"ipconnections": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maximum number of connections per IP address (0 for unlimited).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"loginattempt": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maximum number of allowed login attempts before disconnecting.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"timeout": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maximum idle time in seconds before disconnecting.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"timeout_notransfer": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Maximum time in seconds a client is allowed to spend between transfers.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"localuserbw": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Local user upload bandwidth in KiB/s (0 for unlimited).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"localuserdlbw": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Local user download bandwidth in KiB/s (0 for unlimited).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"anonuserbw": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Anonymous user upload bandwidth in KiB/s (0 for unlimited).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"anonuserdlbw": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Anonymous user download bandwidth in KiB/s (0 for unlimited).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"passiveportsmin": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Lower bound of the passive port range (0 for default OS-assigned range).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"passiveportsmax": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Upper bound of the passive port range (0 for default OS-assigned range).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"defaultroot": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether local users are restricted to their home directory by default.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"onlyanonymous": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether only anonymous logins are allowed.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"onlylocal": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether only local user logins are allowed (no anonymous access).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"ident": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether IDENT authentication is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"fxp": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether File eXchange Protocol (server-to-server transfers) is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"resume": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether resuming interrupted transfers is allowed.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"reversedns": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether reverse DNS lookups are performed on client connections.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether TLS/SSL is enabled for FTP (FTPS).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_allow_client_renegotiations": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: allow client renegotiations.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_allow_dot_login": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: allow dot login.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_allow_per_user": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: allow per-user certificates.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_common_name_required": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: require a common name in the client certificate.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_dns_name_required": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: require a DNS name in the client certificate.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_enable_diags": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: enable diagnostic logging.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_export_cert_data": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: export certificate data to the environment.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_ip_address_required": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: require the IP address in the client certificate to match.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_no_empty_fragments": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: disable empty fragments (workaround for buggy clients).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_no_session_reuse_required": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: require a new SSL session on data connections.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tls_opt_stdenvvars": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS option: export standard environment variables.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"masqaddress": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Public IP address or hostname to masquerade as for passive connections behind NAT.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"banner": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Banner text displayed to clients on connection.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra proftpd.conf options, appended verbatim.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dirmask": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Octal umask applied to directories created via FTP.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"filemask": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Octal umask applied to files created via FTP.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tls_policy": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "TLS policy. One of: on, off, data, !data, auth, ctrl+data, ctrl+!data, auth+data, auth+!data. Not validated by this provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ssltls_certificate": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "ID of the certificate used for TLS/SSL. 0 (or omitted) means no certificate is set (API null).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"anonpath": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Path exposed to anonymous FTP users. \"\" (or omitted) means unset (API null).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
