// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config

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
		Description: "Manages the TrueNAS NFS service configuration. This is a singleton resource — " +
			"there is exactly one NFS configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls nfs.update, and Terraform delete only removes the resource " +
			"from state (the configuration is left in place, since existing NFS exports and clients may depend " +
			"on it).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"nfs_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"servers": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of NFS server threads. A value of 0 restores automatic thread management.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"allow_nonroot": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether NFS client mount requests from unprivileged (non-root) ports are allowed.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"protocols": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "NFS protocol versions to serve. One or more of NFSV3, NFSV4.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"v4_domain": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "NFSv4 domain override for ID mapping. Empty string uses the system default.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"v4_krb": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether Kerberos is required for NFSv4 mounts.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"bindip": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "IP addresses to bind the NFS service to. Empty list binds to all addresses.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"mountd_port": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "TCP/UDP port used by mountd. A value of 0 restores the default (dynamic) port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"rpcstatd_port": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "TCP/UDP port used by rpc.statd. A value of 0 restores the default (dynamic) port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"rpclockd_port": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "TCP/UDP port used by rpc.lockd. A value of 0 restores the default (dynamic) port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"mountd_log": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether mountd logging is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"statd_lockd_log": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether rpc.statd and rpc.lockd logging is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"userd_manage_gids": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether supplemental groups are resolved server-side (more than 16 groups).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"rdma": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether NFS over RDMA is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"managed_nfsd": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the NFS server daemon is currently managed/running. Server-computed; never sent to nfs.update.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"v4_krb_enabled": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether Kerberos is currently enabled/active for NFSv4. Server-computed; never sent to nfs.update.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"keytab_has_nfs_spn": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the system keytab currently has an NFS service principal name. Server-computed; never sent to nfs.update.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
