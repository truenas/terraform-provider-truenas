// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS SMB service configuration. This is a singleton resource — " +
			"there is exactly one SMB configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls smb.update, and Terraform delete only removes the resource " +
			"from state (the configuration is left in place, since shares and domain membership may depend on it).\n\n" +
			"`stateful_failover`, `minimum_protocol`, and `search_protocols` are writable only on TrueNAS " +
			"26.0 and later: none of the three exist on smb.update below TrueNAS 26.0 (probed live) — setting any " +
			"of them explicitly in configuration against a pre-26.0 target is an apply-time error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"smb_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"netbiosname": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "NetBIOS name of the server.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"netbiosalias": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "NetBIOS aliases for the server.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"workgroup": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Workgroup name.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Server description string, shown to SMB clients.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"unixcharset": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UNIX character set. One of UTF-8, GB2312, HZ-GB-2312, CP1361, and others. Not validated by this provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"localmaster": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the server participates in local master browser elections.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"syslog": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether SMB logging is also written to syslog.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"aapl_extensions": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether Apple SMB2/3 protocol extensions are enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"admin_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Group whose members are granted SMB admin (root-equivalent) privileges. Empty string clears the admin group (sent to TrueNAS as null).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"guest": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Account used for guest access.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"filemask": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Default file creation mask. \"DEFAULT\" uses the built-in default.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dirmask": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Default directory creation mask. \"DEFAULT\" uses the built-in default.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ntlmv1_auth": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the insecure NTLMv1 authentication protocol is allowed.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"multichannel": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether SMB multichannel support is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"encryption": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SMB encryption requirement. One of DEFAULT, NEGOTIATE, DESIRED, REQUIRED.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bindip": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "IP addresses to bind the SMB service to. Empty list binds to all addresses.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"smb_options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Additional smb.conf options, appended verbatim.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"debug": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether verbose SMB debug logging is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"stateful_failover": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether stateful SMB failover support is enabled. Writable only on TrueNAS " +
					"26.0 and later — it does not exist on smb.update below TrueNAS 26.0, so explicitly setting it " +
					"there is an apply-time error.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"minimum_protocol": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Minimum SMB protocol version accepted. One of SMB1, SMB2, SMB3. Writable only on " +
					"TrueNAS 26.0 and later — it does not exist on smb.update below TrueNAS 26.0, so " +
					"explicitly setting it there is an apply-time error.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"search_protocols": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional network protocols used for server discovery (e.g. WSD, NSD). Writable " +
					"only on TrueNAS 26.0 and later — it does not exist on smb.update below TrueNAS 26.0, so " +
					"explicitly setting it there is an apply-time error.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"server_sid": schema.StringAttribute{
				Computed:      true,
				Description:   "Server SID (security identifier). Server-assigned and stable; never sent to TrueNAS.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
