// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a privilege on TrueNAS: a named bundle of roles granted to the members of one " +
			"or more local/directory-service groups, controlling their administrative access to the web UI and API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the privilege.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the privilege (must be unique). Updatable in place.",
			},
			"local_groups": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "GIDs of local groups whose members gain this privilege. Defaults to an empty list.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"ds_groups": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "GIDs of directory-service groups whose members gain this privilege. Defaults to an empty list.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"roles": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Role names included in this privilege (see TrueNAS's privilege.roles API method for " +
					"the full list it supports). Defaults to an empty list.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"web_shell": schema.BoolAttribute{
				Required:    true,
				Description: "Whether members of the assigned groups may log in to the web shell.",
			},
			// Computed only — server generated
			"builtin_name": schema.StringAttribute{
				Computed: true,
				Description: "Internal name of the built-in privilege if this is a system privilege " +
					"(e.g. \"LOCAL_ADMINISTRATOR\"); null for custom privileges such as the ones this resource creates.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
