// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// hhmmRe matches a 24-hour "HH:MM" time in the zero-padded form the server
// stores and returns ("8:00" would be read back as "08:00").
var hhmmRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// windowAttr is a schedule begin/end time. Optional+Computed: the server
// fills 00:00/23:59 when it is not sent, and the prior value is kept in the
// plan so an update always sends the window back (the server replaces the
// whole schedule object).
func windowAttr(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: desc,
		Validators: []validator.String{
			stringvalidator.RegexMatches(hhmmRe, `must be 24-hour "HH:MM" with a leading zero, e.g. "08:00"`),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a periodic snapshot task on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Dataset path to snapshot.",
			},
			"recursive": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"exclude": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"lifetime_value": schema.Int64Attribute{
				Required: true,
			},
			"lifetime_unit": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("DAY", "HOUR", "MONTH", "WEEK", "YEAR")},
				Required:    true,
				Description: "HOUR, DAY, WEEK, MONTH, or YEAR.",
			},
			"naming_schema": schema.StringAttribute{
				Required:    true,
				Description: `Snapshot naming pattern, e.g. "auto-%Y-%m-%d_%H-%M".`,
			},
			"schedule": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute (e.g. 0, */15)."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week."},
					"begin":  windowAttr("Start of the daily window in which snapshots are taken, 24-hour \"HH:MM\". Defaults to \"00:00\" on create; when omitted later, the current value is kept."),
					"end":    windowAttr("End of the daily window in which snapshots are taken, 24-hour \"HH:MM\". Defaults to \"23:59\" on create; when omitted later, the current value is kept."),
				},
			},
			"allow_empty": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
