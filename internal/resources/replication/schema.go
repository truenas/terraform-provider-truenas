// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
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
				Description: "LOCAL (the default, or SSH when only ssh_credentials is set) replicates within the same system; SSH replicates to/from a " +
					"remote system over a truenas_keychain_ssh_connection credential (\"ssh_credentials\"); " +
					"SSH+NETCAT authenticates over SSH but transfers data over an unencrypted netcat " +
					"connection for higher throughput on trusted networks (configure the netcat_* attributes). " +
					"Changing this forces a new resource.",
				Validators: []validator.String{stringvalidator.OneOf("LOCAL", "SSH", "SSH+NETCAT")},
				PlanModifiers: []planmodifier.String{
					// F9: a plain Default "LOCAL" planned an imported SSH task back to
					// LOCAL — and, with the replace-on-change below, a destroy/recreate.
					planmod.ResetWhenUnsetString("LOCAL"),
					replaceIfTransportChanged(),
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
				CustomType:  normalize.UnorderedListOf(types.Int64Type),
				Description: "IDs of periodic snapshot tasks that feed this replication task. Order does not matter (the server sorts them).",
				PlanModifiers: []planmodifier.List{
					normalize.KeepStateIfEquivalentList(normalize.UnorderedListOf(types.Int64Type)),
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
				Description: "Cron schedule for automatic replication runs. Removing it clears the schedule on the server (required when auto = false).",
				Attributes: map[string]schema.Attribute{
					"minute": schema.StringAttribute{Required: true, Description: "Cron minute (e.g. 0, */15)."},
					"hour":   schema.StringAttribute{Required: true, Description: "Cron hour."},
					"dom":    schema.StringAttribute{Required: true, Description: "Day of month."},
					"month":  schema.StringAttribute{Required: true, Description: "Month."},
					"dow":    schema.StringAttribute{Required: true, Description: "Day of week."},
					"begin":  windowAttr("Start of the daily window in which runs may start, 24-hour \"HH:MM\". Defaults to \"00:00\"; when omitted after being set, the current value is kept."),
					"end":    windowAttr("End of the daily window in which runs may start, 24-hour \"HH:MM\". Defaults to \"23:59\"; when omitted after being set, the current value is kept."),
				},
			},
			"retention_policy": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("CUSTOM", "NONE", "SOURCE")},
				Required:    true,
				Description: "SOURCE, CUSTOM, or NONE.",
			},
			// lifetime_value/lifetime_unit are Optional, not Computed: they only
			// apply to retention_policy = CUSTOM, and removing them from config
			// must plan null so the update clears them (the server rejects them
			// under SOURCE/NONE).
			"lifetime_value": schema.Int64Attribute{
				Optional:    true,
				Description: "Retention lifetime value. Only valid with retention_policy = CUSTOM; null otherwise.",
			},
			"lifetime_unit": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("DAY", "HOUR", "MONTH", "WEEK", "YEAR")},
				Optional:    true,
				Description: "HOUR, DAY, WEEK, MONTH, or YEAR. Only valid with retention_policy = CUSTOM; null otherwise.",
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
				Description: "Encryption key (hex string, or passphrase per encryption_key_format). Write-only: never stored in Terraform state or read back; to change it, also bump encryption_key_wo_version. Requires Terraform >= 1.11.",
			},
			"encryption_key_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for encryption_key. It is write-only, so changing it alone is not a " +
					"change Terraform can see; bump this to send the new key. The key is sent on create and, after that, " +
					"only when this integer changes or encryption with its own key (not inherited) is turned on; other " +
					"updates leave the stored key alone.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("encryption_key")),
				},
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
					"begin":  windowAttr("Start of the daily window, 24-hour \"HH:MM\". Defaults to \"00:00\"; when omitted after being set, the current value is kept."),
					"end":    windowAttr("End of the daily window, 24-hour \"HH:MM\". Defaults to \"23:59\"; when omitted after being set, the current value is kept."),
				},
			},
			"properties_override": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "ZFS properties to force to specific values on the target datasets (property name -> value). Omit it (rather than {}) for none.",
				// The server stores {} as "no overrides" and it reads back as
				// null, so an explicit {} could never match.
				Validators: []validator.Map{mapvalidator.SizeAtLeast(1)},
			},
			"lifetimes": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Per-schedule snapshot retention rules on the target (used with retention_policy = CUSTOM). Each rule keeps snapshots matching its schedule for lifetime_value lifetime_units. Omit it (rather than []) for none.",
				// The server stores [] as "no rules" and it reads back as null,
				// so an explicit [] could never match.
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
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

// hhmmRe matches a 24-hour "HH:MM" time in the zero-padded form the server
// stores and returns ("8:00" would be read back as "08:00").
var hhmmRe = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// windowAttr is a schedule/restrict_schedule begin/end time. Optional+Computed:
// the server fills 00:00/23:59 when it is not sent, and the prior value is
// kept in the plan so an update sends the window back (the server replaces
// the whole schedule object).
func windowAttr(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: desc,
		Validators: []validator.String{
			stringvalidator.RegexMatches(hhmmRe, `must be 24-hour "HH:MM" with a leading zero, e.g. "08:00"`),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
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
