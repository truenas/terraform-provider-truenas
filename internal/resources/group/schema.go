// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a group on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric group ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"gid": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "UNIX GID for the group (auto-assigned if omitted). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Group name. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"smb": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the group has SMB authentication enabled.",
			},
			"sudo_commands": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of allowed sudo commands.",
			},
			"sudo_commands_nopasswd": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of sudo commands allowed without password.",
			},
			// Computed-only — server generated
			"builtin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is a built-in system group.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"immutable": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this group cannot be modified.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"local": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is a local group (vs. directory service).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
