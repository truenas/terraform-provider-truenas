// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS system general configuration (management UI, timezone, " +
			"keyboard map, usage collection). This is a singleton resource — there is exactly one system " +
			"general configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls system.general.update, and Terraform delete only removes the resource from " +
			"state (the configuration is left in place, since it controls management UI access). WARNING: " +
			"several attributes on this resource control how the management UI is reached over the network " +
			"(ui_address, ui_allowlist, ui_port, ui_httpsport, ui_v6address); changing them can cut off " +
			"management access to the system.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"system_general\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ds_auth": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether directory service credentials may be used to log in to the web UI.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"kbdmap": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "System console keyboard layout/map.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"timezone": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "System timezone (e.g. \"America/Los_Angeles\").",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ui_address": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "IPv4 addresses the web UI binds to. WARNING: changing UI network settings can " +
					"cut off management access to the system.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"ui_allowlist": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses/networks allowed to access the web UI. Empty list allows all. " +
					"WARNING: changing UI network settings can cut off management access to the system.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"ui_certificate": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Numeric ID of the certificate used by the web UI for HTTPS. A value of 0 clears " +
					"the assigned certificate (null on the wire).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"ui_consolemsg": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether console messages are shown on the login screen.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"ui_httpsport": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "TCP port the web UI listens on for HTTPS. WARNING: changing this can cut off " +
					"management access to the system.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"ui_httpsprotocols": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "TLS protocol versions accepted by the web UI (e.g. TLSv1.2, TLSv1.3).",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"ui_httpsredirect": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether HTTP requests to the web UI are redirected to HTTPS.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"ui_port": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "TCP port the web UI listens on for HTTP. WARNING: changing this can cut off " +
					"management access to the system.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"ui_v6address": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "IPv6 addresses the web UI binds to.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"ui_x_frame_options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "X-Frame-Options header value sent by the web UI. One of SAMEORIGIN, DENY, ALLOW_ALL.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"usage_collection": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether anonymous usage statistics are collected.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			// Computed-only (server-generated)
			"ui_certificate_name": schema.StringAttribute{
				Computed: true,
				// No UseStateForUnknown: this value is derived from
				// ui_certificate, so it legitimately changes when the user
				// reassigns the UI certificate; carrying the prior state
				// forward would trip Terraform's post-apply consistency check.
				Description: "Name of the certificate currently assigned to the web UI. Server-computed; never sent to system.general.update.",
			},
			"usage_collection_is_set": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether usage_collection has been explicitly set by an administrator. Server-computed; never sent to system.general.update.",
			},
			"wizardshown": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the initial setup wizard has been shown. Server-computed; never sent to system.general.update.",
			},
		},
	}
}
