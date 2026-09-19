// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config

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
		Description: "Manages the TrueNAS pool resilver priority schedule (pool.resilver). This is a " +
			"singleton resource — there is exactly one resilver schedule per TrueNAS system, so it is never " +
			"created or deleted on TrueNAS; Terraform create/update calls pool.resilver.update, and Terraform " +
			"delete only removes the resource from state (the configuration is left in place).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"resilver_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"begin": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Time when the resilver operations window begins, 24-hour \"HH:MM\" format " +
					"(e.g. \"18:00\").",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"end": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Time when the resilver operations window ends, 24-hour \"HH:MM\" format " +
					"(e.g. \"09:00\"). If earlier than begin, the window rolls over midnight.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the resilver schedule is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"weekday": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Weekdays when resilver operations are allowed, crontab(5) values " +
					"1 (Monday) through 7 (Sunday).",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
