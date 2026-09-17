// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SystemAdvancedDataSource{}

// SystemAdvancedDataSource implements the truenas_system_advanced data
// source.
type SystemAdvancedDataSource struct{ client *client.Client }

// NewDataSource returns a new SystemAdvancedDataSource.
func NewDataSource() datasource.DataSource { return &SystemAdvancedDataSource{} }

func (d *SystemAdvancedDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_advanced"
}

func (d *SystemAdvancedDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS system advanced configuration (syslog, console, kernel " +
			"debugging, SED, GPU isolation). Takes no arguments: there is exactly one system advanced " +
			"configuration per TrueNAS system. Has no sed_passwd attribute: TrueNAS never returns a usable " +
			"value for it.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"system_advanced\".",
			},
			"advancedmode": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether advanced mode is shown in the web UI.",
			},
			"anonstats": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether anonymous usage statistics are sent.",
			},
			"autotune": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the autotune script runs at boot to tune system parameters.",
			},
			"boot_scrub": dschema.Int64Attribute{
				Computed:    true,
				Description: "Interval, in days, between automatic boot pool scrubs.",
			},
			"consolemenu": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the console setup menu is enabled.",
			},
			"consolemsg": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether kernel messages are shown in the console during startup.",
			},
			"debugkernel": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the debug kernel is used on boot.",
			},
			"fqdn_syslog": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the fully-qualified domain name is used in syslog messages.",
			},
			"kdump_enabled": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether kdump (kernel crash dump capture) is enabled.",
			},
			"kernel_extra_options": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra kernel command-line options appended at boot.",
			},
			"login_banner": dschema.StringAttribute{
				Computed:    true,
				Description: "Text displayed as a login banner before authentication.",
			},
			"motd": dschema.StringAttribute{
				Computed:    true,
				Description: "Message of the day, displayed after a successful console/SSH login.",
			},
			"nvidia": dschema.BoolAttribute{
				Computed: true,
				Description: "Whether the NVIDIA driver is installed/enabled. Reports false on TrueNAS " +
					"releases below 26.0 (the field does not exist there); writable (via the " +
					"truenas_system_advanced resource) only on TrueNAS 26.0 and later.",
			},
			"overprovision": dschema.Int64Attribute{
				Computed:    true,
				Description: "Amount, in GiB, of swap-on-ZFS overprovisioning.",
			},
			"powerdaemon": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the power management daemon (powerd) is enabled.",
			},
			"sed_user": dschema.StringAttribute{
				Computed:    true,
				Description: "SED user used to unlock drives: one of USER or MASTER.",
			},
			"serialconsole": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the serial console is enabled.",
			},
			"serialport": dschema.StringAttribute{
				Computed:    true,
				Description: "Serial port device used for the serial console (e.g. \"ttyS0\").",
			},
			"serialspeed": dschema.StringAttribute{
				Computed:    true,
				Description: "Serial console baud rate: one of 9600, 19200, 38400, 57600, or 115200.",
			},
			"syslog_audit": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether audit logs are sent to the configured syslog servers.",
			},
			"sysloglevel": dschema.StringAttribute{
				Computed: true,
				Description: "Minimum severity of messages sent to syslog: one of F_EMERG, F_ALERT, F_CRIT, " +
					"F_ERR, F_WARNING, F_NOTICE, F_INFO.",
			},
			"syslogservers": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Remote syslog servers messages are forwarded to.",
			},
			"traceback": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether a Python traceback is displayed on a middleware crash.",
			},
			"uploadcrash": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether crash dumps and telemetry are automatically uploaded to TrueNAS.",
			},
			"anonstats_token": dschema.StringAttribute{
				Computed:    true,
				Description: "Token used when submitting anonymous usage statistics.",
			},
			"isolated_gpu_pci_ids": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "PCI IDs of GPUs isolated from the host for passthrough.",
			},
		},
	}
}

func (d *SystemAdvancedDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *SystemAdvancedDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SystemAdvancedDataSourceModel

	raw, err := d.client.CallRead(ctx, "system.advanced.config")
	if err != nil {
		resp.Diagnostics.AddError("Read system advanced configuration failed", err.Error())
		return
	}

	var api systemAdvancedAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse system.advanced.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
