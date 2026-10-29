// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
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
				Required:    true,
				Description: "User's full name. Required: user.create rejects a user without one.",
			},
			"email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "User's email address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"home": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Home directory path.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"home_mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Octal permission mode for the user's home directory, e.g. \"700\". Sent on create, and on " +
					"update only when it changes. user.query does not return it, so it is not drift-detected; on import " +
					"it is read from the mode of the home directory when that is under /mnt.",
				CustomType: normalize.OctalModeType{},
				PlanModifiers: []planmodifier.String{
					normalize.KeepStateIfEquivalent(normalize.OctalModeType{}),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"shell": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Login shell path.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"locked": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the user account is locked.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"password_disabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether password authentication is disabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"smb": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the user has SMB authentication enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"webshare": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Grant the user access to the web-based file share. Requires TrueNAS 26.0 or newer; setting it to true on an older release is a plan-time error.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ssh_password_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether SSH password authentication is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"sshpubkey": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "SSH public key(s) for the user. Surrounding whitespace (such as the trailing newline of a file() read) is ignored when comparing with the server, which strips it.",
				CustomType:  normalize.TrimmedType{},
				PlanModifiers: []planmodifier.String{
					normalize.KeepStateIfEquivalent(normalize.TrimmedType{}),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sudo_commands": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of allowed sudo commands.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"sudo_commands_nopasswd": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of sudo commands allowed without password.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"groups": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "List of additional group IDs the user belongs to.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
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
				Optional: true,
				Computed: true,
				Description: "When true, the user's primary group is a group named after the username (TrueNAS creates it, or " +
					"reuses an existing local group of that name) instead of the existing group id in 'group'. Only used " +
					"when the user is created: changing it later has no effect on the server (the plan shows a warning). " +
					"Read back as whether the primary group's name equals the username.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "User password. Write-only: never stored in Terraform state. Requires Terraform >= 1.11. Sent on create; after that only when `password_wo_version` changes (see there).",
			},
			"password_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Rotation trigger for the write-only 'password'. The password is sent when the user is created and, on later applies, only when this integer changes (or when `smb` is turned on, which TrueNAS only allows together with the password). A write-only value is absent from state, so Terraform cannot detect that the password changed: bump this integer whenever you change the password. An update that does not change it leaves the stored password, its hash and `last_password_change` alone, so password aging (`max_password_age`) works for Terraform-managed users.",
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
