// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

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
		Description: "Manages a local user on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric user ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"uid": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "UNIX UID for the user (auto-assigned if omitted). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Login username. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"full_name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "User's full name.",
			},
			"email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "User's email address.",
			},
			"home": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Home directory path.",
			},
			"shell": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Login shell path.",
			},
			"locked": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the user account is locked.",
			},
			"password_disabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether password authentication is disabled.",
			},
			"smb": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the user has SMB authentication enabled.",
			},
			"ssh_password_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether SSH password authentication is enabled.",
			},
			"sshpubkey": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "SSH public key(s) for the user.",
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
			"groups": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "List of additional group IDs the user belongs to.",
			},
			"group": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Primary group ID for the user. Either 'group' or 'group_create' must be set when creating a user.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"group_create": schema.BoolAttribute{
				Optional:    true,
				Description: "When true, creates a new primary group matching the username instead of using an existing group id from 'group'. Write-only: only sent on create, never read back into state.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "User password. Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			// Computed-only — server generated
			"builtin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is a built-in system user.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"immutable": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this user cannot be modified.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"local": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is a local user (vs. directory service).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
