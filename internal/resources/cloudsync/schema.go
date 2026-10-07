// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a cloud sync task on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Task description. Cloud sync tasks have no name field; description serves as the task label.",
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Local filesystem path to sync.",
			},
			"credentials": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the cloud sync credentials to use.",
			},
			"direction": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("PULL", "PUSH")},
				Required:    true,
				Description: "PUSH or PULL.",
			},
			"transfer_mode": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("COPY", "MOVE", "SYNC")},
				Required:    true,
				Description: "SYNC, COPY, or MOVE.",
			},
			"attributes": schema.StringAttribute{
				Required:    true,
				Description: `JSON document of provider-specific attributes, e.g. {"bucket": "...", "folder": "..."}.`,
			},
			"schedule": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute (e.g. 0, */15)."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week."},
				},
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"snapshot": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Transfer options (coverage audit).
			"transfers": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Maximum number of parallel file transfers. Null uses the rclone default.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"follow_symlinks": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Follow symbolic links and sync the files they point to.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"create_empty_src_dirs": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Create empty directories in the destination that exist in the source.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Client-side encryption (rclone crypt).
			"encryption": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Encrypt file contents before uploading (rclone crypt). Requires encryption_password.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"filename_encryption": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Also encrypt file and directory names (only meaningful when encryption is enabled).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"encryption_password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Password for client-side encryption. Write-only: never stored in Terraform state or read back. Requires Terraform >= 1.11.",
			},
			"encryption_salt": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Salt for client-side encryption key derivation. Write-only: never stored in Terraform state or read back. Requires Terraform >= 1.11.",
			},
			"bwlimit": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Bandwidth-limit schedule. Each entry sets a limit that takes effect at a time of day.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"time": schema.StringAttribute{
							Required:    true,
							Description: "Time of day the limit takes effect, 24-hour \"HH:MM\" (e.g. \"18:00\").",
						},
						"bandwidth": schema.Int64Attribute{
							Optional:    true,
							Description: "Bandwidth limit in bytes per second. Null/omitted means no limit from this time.",
						},
					},
				},
			},
			"include": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"exclude": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"pre_script": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"post_script": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
