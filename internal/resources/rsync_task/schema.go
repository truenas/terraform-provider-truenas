// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an rsync task (rsynctask) on TrueNAS: a scheduled or manually-triggered rsync " +
			"of a local path to/from a remote rsync module (MODULE mode) or a remote host over SSH (SSH mode).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the rsync task.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Local filesystem path to synchronize.",
			},
			"user": schema.StringAttribute{
				Required:    true,
				Description: "Username to run the rsync task as.",
			},
			"mode": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Operating mechanism for rsync: MODULE (rsync module/daemon protocol) or SSH. Defaults to MODULE.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"remotehost": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "IP address or hostname of the remote system. If the username differs on the remote " +
					"host, use \"username@remote_host\" format. Null when unset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"remoteport": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Port number for the SSH connection. Only applies when mode is SSH. Null when unset.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"remotemodule": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the remote rsync module. Should be set when mode is MODULE. Null when unset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ssh_credentials": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Keychain credential ID (of type SSH_CREDENTIALS) used to connect to the remote host " +
					"in SSH mode. Null (the default) means the run-as user's own SSH keys are used.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"remotepath": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Path on the remote system to synchronize with.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"direction": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether data is PUSHed to or PULLed from the remote system. Defaults to PUSH.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"desc": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the rsync task.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"schedule": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cron schedule for when the rsync task should run.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute (e.g. \"00\", \"*/15\")."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week (cron format, 7=Sunday)."},
				},
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Recursively transfer subdirectories. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"times": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Preserve modification times of files. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"compress": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Reduce the size of the data to be transmitted. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"archive": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Make rsync run recursively, preserving symlinks, permissions, modification times, " +
					"group, and special files. Defaults to false.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"delete": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Delete files in the destination directory that do not exist in the source directory. Defaults to false.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"quiet": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Suppress informational messages from rsync. Defaults to false.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"preserveperm": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Preserve original file permissions. Defaults to false.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"preserveattr": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Preserve extended attributes of files. Defaults to false.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"delayupdates": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Delay updating destination files until all transfers are complete. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"extra": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional rsync command-line options. Defaults to an empty list.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether this rsync task is enabled. Defaults to true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"validate_rpath": schema.BoolAttribute{
				Optional: true,
				Description: "Validate the existence of the remote path at create/update time. Write-only: " +
					"rsynctask.query never returns this flag, so it is not read back from the API and is not " +
					"verified on import.",
			},
			"ssh_keyscan": schema.BoolAttribute{
				Optional: true,
				Description: "Automatically add the remote host key to the run-as user's known_hosts file at " +
					"create/update time. Write-only: rsynctask.query never returns this flag, so it is not read " +
					"back from the API and is not verified on import.",
			},
		},
	}
}
