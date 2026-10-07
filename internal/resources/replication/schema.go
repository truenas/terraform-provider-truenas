// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a replication task on TrueNAS. Supports LOCAL replication (within the " +
			"same system) and remote replication over SSH (transport = \"SSH\", authenticating via a " +
			"truenas_keychain_ssh_connection credential referenced by \"ssh_credentials\").",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric replication task ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the replication task.",
			},
			"direction": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("PULL", "PUSH")},
				Required:    true,
				Description: "PUSH or PULL. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"transport": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "LOCAL (default) replicates within the same system; SSH replicates to/from a " +
					"remote system over a truenas_keychain_ssh_connection credential (\"ssh_credentials\"); " +
					"SSH+NETCAT authenticates over SSH but transfers data over an unencrypted netcat " +
					"connection for higher throughput on trusted networks (configure the netcat_* attributes). " +
					"Changing this forces a new resource.",
				Validators: []validator.String{stringvalidator.OneOf("LOCAL", "SSH", "SSH+NETCAT")},
				Default:    stringdefault.StaticString("LOCAL"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ssh_credentials": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Numeric id of a truenas_keychain_ssh_connection (keychaincredential of type " +
					"SSH_CREDENTIALS) to replicate over. Required when transport = \"SSH\"; must be unset (0) " +
					"for transport = \"LOCAL\".",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"sudo": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Use sudo (expected to be passwordless on the remote system) to run zfs commands " +
					"over SSH. Only meaningful for transport = \"SSH\".",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"compression": schema.StringAttribute{
				Optional: true,
				Description: "Compresses the SSH stream: LZ4, PIGZ, or PLZIP. Available only for transport = " +
					"\"SSH\"; must be unset for transport = \"LOCAL\".",
				Validators: []validator.String{stringvalidator.OneOf("LZ4", "PIGZ", "PLZIP")},
			},
			"speed_limit": schema.Int64Attribute{
				Optional: true,
				Description: "Limits the speed of the SSH stream, in bytes per second. Available only for " +
					"transport = \"SSH\"; must be unset for transport = \"LOCAL\".",
				Validators: []validator.Int64{int64validator.AtLeast(1)},
			},
			"netcat_active_side": schema.StringAttribute{
				Optional: true,
				Description: "For transport = \"SSH+NETCAT\", which side actively opens the netcat data " +
					"connection: LOCAL or REMOTE. Required for SSH+NETCAT; must be unset for other transports.",
				Validators: []validator.String{stringvalidator.OneOf("LOCAL", "REMOTE")},
			},
			"netcat_active_side_listen_address": schema.StringAttribute{
				Optional: true,
				Description: "For transport = \"SSH+NETCAT\", the IP address the active side listens on. " +
					"Only valid for SSH+NETCAT.",
			},
			"netcat_active_side_port_min": schema.Int64Attribute{
				Optional: true,
				Description: "For transport = \"SSH+NETCAT\", the low end of the port range the active side " +
					"may listen on (1-65535). Only valid for SSH+NETCAT.",
				Validators: []validator.Int64{int64validator.Between(1, 65535)},
			},
			"netcat_active_side_port_max": schema.Int64Attribute{
				Optional: true,
				Description: "For transport = \"SSH+NETCAT\", the high end of the port range the active side " +
					"may listen on (1-65535). Only valid for SSH+NETCAT.",
				Validators: []validator.Int64{int64validator.Between(1, 65535)},
			},
			"netcat_passive_side_connect_address": schema.StringAttribute{
				Optional: true,
				Description: "For transport = \"SSH+NETCAT\", the IP address the passive side connects to. " +
					"Only valid for SSH+NETCAT.",
			},
			"source_datasets": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Source dataset paths to replicate.",
			},
			"target_dataset": schema.StringAttribute{
				Required:    true,
				Description: "Target dataset path.",
			},
			"recursive": schema.BoolAttribute{
				Required:    true,
				Description: "Replicate child datasets recursively.",
			},
			"exclude": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Dataset paths to exclude from a recursive replication.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"properties": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Include dataset properties in the replication stream.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"replicate": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Replicate the full dataset tree.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"periodic_snapshot_tasks": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "IDs of periodic snapshot tasks that feed this replication task.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"naming_schema": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Naming schemas of snapshots to replicate. Mutually exclusive with name_regex.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"also_include_naming_schema": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional naming schemas to include. Mutually exclusive with name_regex.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"name_regex": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Regular expression matching snapshot names to replicate. Mutually exclusive with naming_schema/also_include_naming_schema.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"auto": schema.BoolAttribute{
				Required:    true,
				Description: "Run this replication task automatically on a schedule.",
			},
			"schedule": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Cron schedule for automatic replication runs.",
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute (e.g. 0, */15)."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week."},
				},
			},
			"retention_policy": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("CUSTOM", "NONE", "SOURCE")},
				Required:    true,
				Description: "SOURCE, CUSTOM, or NONE.",
			},
			"lifetime_value": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Retention lifetime value. Unset (0) when retention_policy is not CUSTOM.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"lifetime_unit": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("DAY", "HOUR", "MONTH", "WEEK", "YEAR")},
				Optional:    true,
				Computed:    true,
				Description: "HOUR, DAY, WEEK, MONTH, or YEAR. Unset (\"\") when retention_policy is not CUSTOM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"readonly": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("IGNORE", "REQUIRE", "SET")},
				Optional:    true,
				Computed:    true,
				Description: "SET, REQUIRE, or IGNORE.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the replication task is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"retries": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of retries on failure.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},

			// --- Send-stream / behaviour options (coverage audit) ---
			"compressed":             replBoolAttr("Enable compressed ZFS send streams (zfs send -c)."),
			"embed":                  replBoolAttr("Enable embedded-block ZFS send streams (zfs send -e)."),
			"large_block":            replBoolAttr("Enable large-block ZFS send streams (zfs send -L)."),
			"allow_from_scratch":     replBoolAttr("Destroy all snapshots on the target and replicate everything from scratch if the incremental base is missing."),
			"hold_pending_snapshots": replBoolAttr("Prevent source snapshots from being deleted by retention while a replication is pending."),
			"only_matching_schedule": replBoolAttr("Only replicate snapshots that match `schedule` or `restrict_schedule`."),
			"logging_level": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("DEBUG", "ERROR", "INFO", "WARNING")},
				Optional:    true,
				Computed:    true,
				Description: "Log verbosity for task execution, e.g. DEBUG, INFO, WARNING, ERROR. Null uses the system default.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"properties_exclude": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Dataset property names to exclude from replication.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},

			// --- Encryption of the replicated target datasets ---
			"encryption":         replBoolAttr("Create the target datasets as encrypted. Requires encryption_key (or encryption_inherit)."),
			"encryption_inherit": replBoolAttr("Inherit encryption from the target's parent dataset instead of supplying a key."),
			"encryption_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Encryption key (hex string, or passphrase per encryption_key_format). Write-only: never stored in Terraform state or read back. Requires Terraform >= 1.11.",
			},
			"encryption_key_format": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("HEX", "PASSPHRASE")},
				Optional:    true,
				Computed:    true,
				Description: "Format of encryption_key: HEX or PASSPHRASE.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"encryption_key_location": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Path on the target system where the encryption key is stored ($TrueNAS-managed location if omitted).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"restrict_schedule": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Only replicate snapshots taken within this cron window (used with only_matching_schedule). Same fields as schedule, plus optional begin/end times.",
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week."},
					"begin":  schema.StringAttribute{Optional: true, Description: "Start of the daily window, \"HH:MM\"."},
					"end":    schema.StringAttribute{Optional: true, Description: "End of the daily window, \"HH:MM\"."},
				},
			},
			"properties_override": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "ZFS properties to force to specific values on the target datasets (property name -> value).",
			},
			"lifetimes": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Per-schedule snapshot retention rules on the target (used with retention_policy = CUSTOM). Each rule keeps snapshots matching its schedule for lifetime_value lifetime_units.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"schedule": schema.SingleNestedAttribute{
							Required:    true,
							Description: "Cron schedule this retention rule applies to.",
							Attributes: map[string]schema.Attribute{
								"minute": schema.StringAttribute{Required: true},
								"hour":   schema.StringAttribute{Required: true},
								"dom":    schema.StringAttribute{Required: true},
								"month":  schema.StringAttribute{Required: true},
								"dow":    schema.StringAttribute{Required: true},
							},
						},
						"lifetime_value": schema.Int64Attribute{Required: true, Description: "How many lifetime_units to keep matching snapshots."},
						"lifetime_unit": schema.StringAttribute{
							Required:    true,
							Description: "HOUR, DAY, WEEK, MONTH, or YEAR.",
							Validators:  []validator.String{stringvalidator.OneOf("HOUR", "DAY", "WEEK", "MONTH", "YEAR")},
						},
					},
				},
			},
		},
	}
}

// replBoolAttr is an Optional+Computed bool attribute for the GH-coverage-audit
// replication send-stream/behaviour options.
func replBoolAttr(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}
