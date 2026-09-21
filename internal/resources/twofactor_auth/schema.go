// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS two-factor authentication configuration (auth.twofactor.config): " +
			"whether 2FA is required system-wide, the TOTP validation window, and which services additionally " +
			"require it. This is a singleton resource — there is exactly one two-factor authentication " +
			"configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls auth.twofactor.update, and Terraform delete only removes the resource from " +
			"state (the configuration is left in place). SAFETY: \"enabled\" controls system-wide two-factor " +
			"authentication; API-key authentication (used by this provider) is unaffected by it, but changing " +
			"it can lock out password-based logins for users who have not enrolled a TOTP secret. Change it " +
			"deliberately.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"twofactor_auth\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether two-factor authentication is enabled system-wide. Does not affect " +
					"API-key authentication (verified live: a fresh API-key connection continues to " +
					"authenticate normally with this enabled).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"window": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Time window in seconds for TOTP token validation (minimum 0).",
				Validators:  []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"services": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Configuration for which services require two-factor authentication.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"ssh": schema.BoolAttribute{
						Required:    true,
						Description: "Whether two-factor authentication is required for SSH connections.",
					},
				},
			},
		},
	}
}
