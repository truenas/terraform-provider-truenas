package replication

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a replication task on TrueNAS SCALE.",
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
				Required:    true,
				Description: "PUSH or PULL. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"transport": schema.StringAttribute{
				Required:    true,
				Description: "SSH, SSH+NETCAT, or LOCAL. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ssh_credentials": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Keychain SSH credential ID. Unset (0) for LOCAL transport.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"sudo": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Use sudo for ZFS commands on the remote system.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
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
				Optional:    true,
				Computed:    true,
				Description: "HOUR, DAY, WEEK, MONTH, or YEAR. Unset (\"\") when retention_policy is not CUSTOM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"readonly": schema.StringAttribute{
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
		},
	}
}
