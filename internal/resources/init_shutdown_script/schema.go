// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package init_shutdown_script

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an init/shutdown script (initshutdownscript) on TrueNAS: a command or script " +
			"executed at a chosen point in the boot or shutdown sequence.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the init/shutdown script.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Type of task: COMMAND (execute a single command) or SCRIPT (execute a script file).",
				Validators:  []validator.String{stringvalidator.OneOf("COMMAND", "SCRIPT")},
			},
			"command": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Shell command to execute. Required when type is COMMAND; ignored otherwise. " +
					"Defaults to an empty string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"script": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Path to a script to execute. Required when type is SCRIPT; ignored otherwise. " +
					"Defaults to an empty string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"when": schema.StringAttribute{
				Required: true,
				Description: "When to execute: PREINIT (early in boot, before most services start), POSTINIT " +
					"(late in boot, after most services have started), or SHUTDOWN.",
				Validators: []validator.String{stringvalidator.OneOf("PREINIT", "POSTINIT", "SHUTDOWN")},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether this init/shutdown script is enabled to execute. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Time in seconds to wait for the command/script to complete before it is considered " +
					"timed out. Defaults to 10.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"comment": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional comment describing the purpose of this script. Defaults to an empty string.",
				Validators:  []validator.String{stringvalidator.LengthAtMost(255)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
