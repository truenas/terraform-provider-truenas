// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package mail

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS system mail (email) configuration. This is a singleton " +
			"resource — there is exactly one mail configuration per TrueNAS system, so it is never created " +
			"or deleted on TrueNAS; Terraform create/update calls mail.update, and Terraform delete only " +
			"removes the resource from state (the mail configuration is left in place). OAuth-based mail " +
			"configuration is out of scope for this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"mail\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fromemail": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "\"From\" address used on outgoing mail.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fromname": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "\"From\" display name used on outgoing mail.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"outgoingserver": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Outgoing SMTP server hostname.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"port": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Outgoing SMTP server port.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"security": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SMTP transport security: one of PLAIN, SSL, TLS.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"smtp": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether SMTP authentication is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"user": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SMTP authentication username.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pass": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "SMTP authentication password (never read back from TrueNAS). " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
		},
	}
}
