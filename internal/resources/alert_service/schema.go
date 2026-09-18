// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages TrueNAS alert services (notification targets such as Mail, SNMPTrap, Slack, PagerDuty, etc).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric alert service ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the alert service.",
			},
			"level": schema.StringAttribute{
				Required:    true,
				Description: "Minimum alert level that triggers this service. One of: INFO, NOTICE, WARNING, ERROR, CRITICAL, ALERT, EMERGENCY.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the alert service is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"attributes": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON document of service settings. Must include \"type\" (e.g. Mail, SNMPTrap, Slack, PagerDuty). May contain secrets such as webhook URLs or SNMP v3 keys.",
			},
		},
	}
}
