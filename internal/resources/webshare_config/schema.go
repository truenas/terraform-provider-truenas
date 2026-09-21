// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config

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
		Description: "Manages the TrueNAS Webshare service configuration (webshare.config/webshare.update): " +
			"the IP addresses the Webshare HTTP server binds to, whether search indexing is enabled, the passkey " +
			"authentication mode, and the AD/LDAP groups granted access. This is a singleton resource — there is " +
			"exactly one Webshare configuration per TrueNAS system, so it is never created or deleted on TrueNAS; " +
			"Terraform create/update calls webshare.update (probed job:false), and Terraform delete only removes " +
			"the resource from state (the configuration is left in place)." +
			"\n\n" +
			"Requires TrueNAS 26.0 or later: the webshare namespace does not exist on earlier releases " +
			"(probed live — TrueNAS 25.10 exposes 0 webshare.* methods via core.get_methods). Using this resource " +
			"against an older server fails with a clean error during Create/Read/Update rather than a raw API " +
			"error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"webshare_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bindip": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of IP addresses the Webshare HTTP server binds to. An empty list means it " +
					"binds to all addresses. See the webshare.bindip_choices API method on the target box for the " +
					"set of valid values; this schema does not validate them at plan time.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"search": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Enable search indexing for Webshare-exposed content.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"passkey": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Passkey authentication mode: ENABLED, DISABLED, or REQUIRED.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"groups": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "List of AD/LDAP group names whose members are granted access to Webshare.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
