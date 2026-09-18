// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a virtual machine on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric VM ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "VM name.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional VM description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"memory": schema.Int64Attribute{
				Required:    true,
				Description: "Memory allocated to the VM, in bytes.",
			},
			"min_memory": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Minimum memory for ballooning, in bytes (0 = unset).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"vcpus": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of virtual CPUs.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"cores": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of cores per virtual CPU.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"threads": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of threads per core.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"bootloader": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Bootloader type: UEFI, UEFI_CSM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"autostart": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Start this VM automatically at boot.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"time": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Guest clock timezone: LOCAL, UTC.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Seconds to wait for a graceful shutdown before forcing it.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"cpu_mode": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "CPU mode: CUSTOM, HOST-MODEL, HOST-PASSTHROUGH.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cpu_model": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Custom CPU model, applicable when cpu_mode is CUSTOM (\"\" = unset).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"running": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the VM is currently running. Set to true to start, false to stop.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Computed-only — server generated, changes independently of Terraform.
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current VM status: RUNNING, STOPPED.",
			},
		},
	}
}
