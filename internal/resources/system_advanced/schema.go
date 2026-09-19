// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced

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
		Description: "Manages the TrueNAS system advanced configuration (syslog, console, kernel " +
			"debugging, SED, GPU isolation). This is a singleton resource — there is exactly one system " +
			"advanced configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls system.advanced.update, and Terraform delete only removes the resource from " +
			"state (the configuration is left in place, since it controls syslog and console access). " +
			"sed_passwd is write-only: it is never read back from TrueNAS and is not stored in state.\n\n" +
			"`nvidia` is writable only on TrueNAS 26.0 and later: it does not exist on " +
			"system.advanced.update below TrueNAS 26.0 (probed live) — setting it explicitly in configuration " +
			"against a pre-26.0 target is an apply-time error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"system_advanced\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"advancedmode": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether advanced mode is shown in the web UI.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"anonstats": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether anonymous usage statistics are sent.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"autotune": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the autotune script runs at boot to tune system parameters.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"boot_scrub": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Interval, in days, between automatic boot pool scrubs.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"consolemenu": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the console setup menu is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"consolemsg": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether kernel messages are shown in the console during startup.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"debugkernel": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the debug kernel is used on boot.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"fqdn_syslog": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the fully-qualified domain name is used in syslog messages.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"kdump_enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether kdump (kernel crash dump capture) is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"kernel_extra_options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra kernel command-line options appended at boot.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"login_banner": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Text displayed as a login banner before authentication.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"motd": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Message of the day, displayed after a successful console/SSH login.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nvidia": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the NVIDIA driver is installed/enabled. Writable only on TrueNAS " +
					"26.0 and later — it does not exist on system.advanced.update below TrueNAS 26.0, so " +
					"explicitly setting it there is an apply-time error.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"overprovision": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Amount, in GiB, of swap-on-ZFS overprovisioning. A value of 0 clears the " +
					"overprovision setting (null on the wire).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"powerdaemon": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the power management daemon (powerd) is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"sed_passwd": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Global password for Self-Encrypting Drives (SED) (never read back from " +
					"TrueNAS). Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"sed_user": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SED user used to unlock drives: one of USER or MASTER.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"serialconsole": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the serial console is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"serialport": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Serial port device used for the serial console (e.g. \"ttyS0\").",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"serialspeed": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Serial console baud rate: one of 9600, 19200, 38400, 57600, or 115200.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"syslog_audit": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether audit logs are sent to the configured syslog servers.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"sysloglevel": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Minimum severity of messages sent to syslog: one of F_EMERG, F_ALERT, F_CRIT, " +
					"F_ERR, F_WARNING, F_NOTICE, F_INFO.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"syslogservers": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Remote syslog servers messages are forwarded to.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"traceback": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether a Python traceback is displayed on a middleware crash.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"uploadcrash": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether crash dumps and telemetry are automatically uploaded to TrueNAS.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			// Computed-only (server-generated)
			"anonstats_token": schema.StringAttribute{
				Computed:    true,
				Description: "Token used when submitting anonymous usage statistics. Server-computed; never sent to system.advanced.update.",
			},
			"isolated_gpu_pci_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "PCI IDs of GPUs isolated from the host for passthrough. Server-computed; managed " +
					"via the separate system.advanced.update_gpu_pci_ids method (out of scope for this " +
					"resource); never sent to system.advanced.update.",
			},
		},
	}
}
