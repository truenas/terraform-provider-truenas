// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS global alert policy: per-alert-class overrides of notification level " +
			"and delivery policy. This is a singleton resource — there is exactly one alert policy per TrueNAS " +
			"system, so it is never created or deleted on TrueNAS; Terraform create/update calls " +
			"alertclasses.update, and Terraform delete resets classes back to \"{}\" (TrueNAS defaults).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"alert_policy\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"classes": schema.StringAttribute{
				Required: true,
				Description: "JSON object mapping alert class names to their notification overrides, e.g. " +
					"{\"UPSBatteryLow\": {\"level\": \"CRITICAL\", \"policy\": \"IMMEDIATELY\"}}. May be \"{}\" to " +
					"leave every class at its TrueNAS default. This attribute represents the FULL set of class " +
					"overrides: any class omitted from the JSON reverts to its default the next time this resource " +
					"is applied. Per-class fields: level is one of INFO, NOTICE, WARNING, ERROR, CRITICAL, ALERT, " +
					"EMERGENCY; policy is one of IMMEDIATELY, HOURLY, DAILY, NEVER.",
			},
		},
	}
}
