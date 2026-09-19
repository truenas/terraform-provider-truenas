// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NTP server on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NTP server ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"address": schema.StringAttribute{
				Required:    true,
				Description: "Hostname or IP address of the NTP server.",
			},
			"burst": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Send a burst of packets when the server is reachable, improving initial synchronization.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"iburst": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Send a burst of packets when the server is unreachable, speeding up initial synchronization.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"prefer": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Prefer this server over other configured NTP servers.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"minpoll": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Minimum polling interval, as a power of 2 in seconds.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"maxpoll": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Maximum polling interval, as a power of 2 in seconds.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"force": schema.BoolAttribute{
				Optional:    true,
				Description: "Bypass validation that checks whether the server is reachable. Write-only: not stored, not read back.",
			},
		},
	}
}
