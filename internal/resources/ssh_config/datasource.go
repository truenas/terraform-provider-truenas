// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ssh_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &SSHConfigDataSource{}

// SSHConfigDataSource implements the truenas_ssh_config data source.
type SSHConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new SSHConfigDataSource.
func NewDataSource() datasource.DataSource { return &SSHConfigDataSource{} }

func (d *SSHConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_config"
}

func (d *SSHConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS SSH service configuration. Takes no arguments: there " +
			"is exactly one SSH configuration per TrueNAS system. SSH host keys are server-managed and are not " +
			"exposed by this datasource.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"ssh_config\".",
			},
			"bindiface": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Interfaces to bind the SSH service to. Empty list binds to all interfaces.",
			},
			"compression": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether SSH compression is enabled.",
			},
			"kerberosauth": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether Kerberos authentication is enabled.",
			},
			"options": dschema.StringAttribute{
				Computed:    true,
				Description: "Extra sshd_config options, appended verbatim.",
			},
			"password_login_groups": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Groups whose members are allowed to authenticate with a password.",
			},
			"passwordauth": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether password authentication is enabled.",
			},
			"sftp_log_facility": dschema.StringAttribute{
				Computed:    true,
				Description: "SFTP subsystem syslog facility. One of \"\" (default), DAEMON, USER, AUTH, LOCAL0-LOCAL7.",
			},
			"sftp_log_level": dschema.StringAttribute{
				Computed:    true,
				Description: "SFTP subsystem syslog level. One of \"\" (default), QUIET, FATAL, ERROR, INFO, VERBOSE, DEBUG, DEBUG1, DEBUG2, DEBUG3.",
			},
			"tcpfwd": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether TCP forwarding is enabled.",
			},
			"tcpport": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP port the SSH service listens on.",
			},
			"weak_ciphers": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Weak ciphers to allow. Valid values: AES128-CBC, NONE.",
			},
		},
	}
}

func (d *SSHConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SSHConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SSHConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "ssh.config")
	if err != nil {
		resp.Diagnostics.AddError("Read SSH configuration failed", err.Error())
		return
	}

	var api sshConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse ssh.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
