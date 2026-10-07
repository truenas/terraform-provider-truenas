// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

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
				Validators:  []validator.String{stringvalidator.OneOf("UEFI", "UEFI_CSM")},
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
				Validators:  []validator.String{stringvalidator.OneOf("LOCAL", "UTC")},
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
				Validators:  []validator.String{stringvalidator.OneOf("CUSTOM", "HOST-MODEL", "HOST-PASSTHROUGH")},
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
			// --- Hardware / boot / CPU options (coverage audit) ---
			"machine_type":                  vmStrAttr("QEMU machine type, e.g. \"q35\" or \"i440fx\". Empty/unset uses the TrueNAS default."),
			"arch_type":                     vmStrAttr("Guest CPU architecture. Empty/unset uses the host architecture."),
			"bootloader_ovmf":               vmStrAttrReplace("OVMF firmware image to use (UEFI bootloader). Empty/unset uses the default. Create-only: changing it recreates the VM."),
			"command_line_args":             vmStrAttr("Extra command-line arguments passed to the VM process."),
			"cpuset":                        vmStrAttr("Physical host CPUs to pin the VM to, e.g. \"0-3\" or \"0,2,4\". Requires pin_vcpus for vCPU pinning."),
			"nodeset":                       vmStrAttr("Host NUMA nodes to pin the VM's memory to, e.g. \"0-1\"."),
			"enable_secure_boot":            vmBoolAttrReplace("Enable UEFI Secure Boot. Create-only: changing it recreates the VM."),
			"trusted_platform_module":       vmBoolAttr("Attach an emulated TPM (TPM 2.0) device."),
			"pin_vcpus":                     vmBoolAttr("Pin the VM's vCPUs to the physical CPUs given in cpuset."),
			"hide_from_msr":                 vmBoolAttr("Hide the hypervisor from the guest (for nested virtualisation / GPU passthrough)."),
			"hyperv_enlightenments":         vmBoolAttr("Enable Hyper-V enlightenments for Windows guests."),
			"enable_cpu_topology_extension": vmBoolAttr("Expose an extended CPU topology to the guest."),
			"suspend_on_snapshot":           vmBoolAttr("Suspend the VM while a snapshot of it is taken. Defaults to true server-side."),

			// Computed-only — server generated, changes independently of Terraform.
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current VM status: RUNNING, STOPPED.",
			},
		},
	}
}

// vmStrAttr is an Optional+Computed string attribute (coverage audit scalar VM option).
func vmStrAttr(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

// vmBoolAttr is an Optional+Computed bool attribute (coverage audit scalar VM option).
func vmBoolAttr(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

// vmStrAttrReplace is like vmStrAttr but for a create-only field: a change
// recreates the VM (vm.update rejects it). See vmCreateOnlyFields.
func vmStrAttrReplace(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: desc,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		},
	}
}

// vmBoolAttrReplace is like vmBoolAttr but for a create-only field: a change
// recreates the VM (vm.update rejects it). See vmCreateOnlyFields.
func vmBoolAttrReplace(desc string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:    true,
		Computed:    true,
		Description: desc,
		PlanModifiers: []planmodifier.Bool{
			boolplanmodifier.UseStateForUnknown(),
			boolplanmodifier.RequiresReplace(),
		},
	}
}
