// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
				Validators:  []validator.String{stringvalidator.OneOf("ALERT", "CRITICAL", "EMERGENCY", "ERROR", "INFO", "NOTICE", "WARNING")},
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
				Description: "JSON document of service settings. Must include \"type\" (e.g. Mail, SNMPTrap, Slack, PagerDuty). May contain secrets such as webhook URLs or SNMP v3 keys; use attributes_secrets_wo to keep secret keys out of state.",
			},
			"attributes_secrets_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only JSON object of secret attributes (e.g. Slack/PagerDuty/AWS " +
					"credentials) merged over attributes when sending to TrueNAS. Never stored in state, " +
					"and not read back on refresh. Requires attributes_secrets_wo_version.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("attributes_secrets_wo_version")),
				},
			},
			"attributes_secrets_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for attributes_secrets_wo. Bump to re-send a changed " +
					"write-only secret overlay. Required when attributes_secrets_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("attributes_secrets_wo")),
				},
			},
		},
	}
}
