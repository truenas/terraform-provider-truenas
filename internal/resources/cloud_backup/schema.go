// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a cloud backup task (cloud_backup.*) on TrueNAS: a restic-based, " +
			"snapshot-and-encrypt backup of a local path to a cloud storage bucket, distinct from " +
			"truenas_cloudsync_task (an rclone-based file sync). cloud_backup.create validates the " +
			"credential/bucket against the actual remote endpoint at apply time (probed live: an S3 credential " +
			"with a bogus access key was rejected with \"InvalidAccessKeyId ... GetBucketLocation\" before any " +
			"local state was created) — a plan that references a non-working credential or a bucket the " +
			"credential can't reach fails at apply, not silently.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the cloud backup task.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the task to display in the UI. Defaults to an empty string.",
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "The local path to back up, beginning with /mnt or /dev/zvol. Updatable in place.",
			},
			"credentials": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the cloud sync credentials (truenas_cloudsync_credentials) to use for each backup.",
			},
			"attributes": schema.StringAttribute{
				Required: true,
				Description: `JSON document of provider-specific attributes. Must include "bucket" ` +
					`(non-empty); "folder" and other provider-specific keys (fast_list, bucket_policy_only, ` +
					`chunk_size, acknowledge_abuse, region, encryption, storage_class) are optional, e.g. ` +
					`{"bucket": "...", "folder": "backups"}.`,
			},
			"schedule": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cron schedule dictating when the task should run.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute (e.g. \"00\", \"*/15\")."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week (\"1\" Monday - \"7\" Sunday)."},
				},
			},
			"pre_script": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "A Bash script to run immediately before every backup. Defaults to an empty string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"post_script": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "A Bash script to run immediately after every backup if it succeeds. Defaults to an empty string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"snapshot": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to create a temporary snapshot of the dataset before every backup. Defaults to false. Cannot be true when absolute_paths is true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"include": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Paths to pass to `restic backup --include`. Defaults to an empty list.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"exclude": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Paths to pass to `restic backup --exclude`. Defaults to an empty list.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the task is enabled. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"password": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Password for the restic repository (RESTIC_PASSWORD). Marked Sensitive (kept " +
					"out of plan/apply output and logs), but — unlike a WriteOnly attribute — it IS stored in " +
					"state and can be read back on refresh/import: the wire field is a pydantic Secret " +
					"(Secret[NonEmptyString], probed via middleware source — cloud_backup.py's CloudBackupEntry) " +
					"whose value is only masked to \"********\" for a caller whose session lacks FULL_ADMIN and " +
					"the CLOUD_BACKUP_WRITE role; this provider's usual admin-scoped API key session sees the " +
					"real value on cloud_backup.get_instance/query (verified against middleware's " +
					"dump_result()/remove_secrets() logic, since no working credential was available to trigger " +
					"a live create — see cloud_backup.create's credential/bucket validation, documented on the " +
					"resource). Never sent to cloud_backup.sync (this provider never calls it).",
			},
			"keep_last": schema.Int64Attribute{
				Required:    true,
				Description: "How many of the most recent backup snapshots to keep after each backup. Must be at least 1.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
			},
			"transfer_setting": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Transfer performance profile, one of the values returned by " +
					"cloud_backup.transfer_setting_choices (probed live: DEFAULT, PERFORMANCE, FAST_STORAGE). " +
					"Defaults to DEFAULT.",
				Validators: []validator.String{stringvalidator.OneOf("DEFAULT", "PERFORMANCE", "FAST_STORAGE")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"absolute_paths": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Preserve absolute paths in each backup (cannot be true when snapshot is true). " +
					"Defaults to false. Immutable: excluded from cloud_backup.update's accepted fields " +
					"(probed against core.get_methods), so changing this forces a new resource.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
			},
			"cache_path": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Local path used to cache restic metadata. If not set, performance may degrade. Null when unset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"rate_limit": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Maximum upload/download rate in KiB/s, applied to cloud_backup.sync/restore. " +
					"Null (the default) means no rate limit. Must be positive when set.",
				Validators: []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
