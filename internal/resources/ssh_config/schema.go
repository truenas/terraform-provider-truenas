// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ssh_config

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
		Description: "Manages the TrueNAS SSH service configuration. This is a singleton resource — " +
			"there is exactly one SSH configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls ssh.update, and Terraform delete only removes the resource " +
			"from state (the configuration is left in place, since management access to the box may depend on " +
			"it). SSH host keys are server-managed and are not modeled by this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"ssh_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bindiface": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Interfaces to bind the SSH service to. Empty list binds to all interfaces.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"compression": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether SSH compression is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"kerberosauth": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether Kerberos authentication is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra sshd_config options, appended verbatim.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"password_login_groups": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Groups whose members are allowed to authenticate with a password.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"passwordauth": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether password authentication is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"sftp_log_facility": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SFTP subsystem syslog facility. One of \"\" (default), DAEMON, USER, AUTH, LOCAL0-LOCAL7. Not validated by this provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"sftp_log_level": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SFTP subsystem syslog level. One of \"\" (default), QUIET, FATAL, ERROR, INFO, VERBOSE, DEBUG, DEBUG1, DEBUG2, DEBUG3. Not validated by this provider.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tcpfwd": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether TCP forwarding is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"tcpport": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "TCP port the SSH service listens on.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"weak_ciphers": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Weak ciphers to allow. Valid values: AES128-CBC, NONE.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
