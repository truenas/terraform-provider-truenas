// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS UPS service configuration. This is a singleton resource — " +
			"there is exactly one UPS configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls ups.update, and Terraform delete only removes the resource " +
			"from state (the configuration is left in place). monpwd is write-only: it is never read back from " +
			"TrueNAS and is not stored in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"ups_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"identifier": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Identifier of the UPS.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"mode": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UPS mode: MASTER or SLAVE.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"remotehost": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Hostname or IP address of the remote UPS (SLAVE mode).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"remoteport": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Port of the remote UPS (SLAVE mode).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"driver": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UPS driver to use.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"port": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Port or address the UPS driver connects to.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra ups.conf options, appended verbatim.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"optionsupsd": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra upsd.conf options, appended verbatim.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Description of the UPS configuration.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"shutdown": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Condition triggering shutdown: LOWBATT or BATT.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"shutdowntimer": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Seconds on battery before shutdown when shutdown is BATT.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"shutdowncmd": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Command executed to shut down the system, or null/empty for the default.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"monuser": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Username used to monitor the UPS.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"monpwd": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Password used to monitor the UPS (never read back from TrueNAS). " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"extrausers": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra users allowed to monitor the UPS, appended to upsd.users.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"rmonitor": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether remote monitoring is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"powerdown": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the UPS is powered down after shutdown.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"hostsync": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Seconds to wait for the master to shut down before a slave shuts down.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"nocommwarntime": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Seconds without UPS communication before a warning is raised, or null/0 for the default.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"complete_identifier": schema.StringAttribute{
				Computed:      true,
				Description:   "Server-derived, fully-qualified UPS identifier. Read-only: never sent to TrueNAS.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
