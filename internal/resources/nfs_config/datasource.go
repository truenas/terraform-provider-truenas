// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NFSConfigDataSource{}

// NFSConfigDataSource implements the truenas_nfs_config data source.
type NFSConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new NFSConfigDataSource.
func NewDataSource() datasource.DataSource { return &NFSConfigDataSource{} }

func (d *NFSConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nfs_config"
}

func (d *NFSConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS NFS service configuration. Takes no arguments: there is " +
			"exactly one NFS configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"nfs_config\".",
			},
			"servers": dschema.Int64Attribute{
				Computed:    true,
				Description: "Number of NFS server threads. 0 means automatic thread management.",
			},
			"allow_nonroot": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether NFS client mount requests from unprivileged (non-root) ports are allowed.",
			},
			"protocols": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "NFS protocol versions served. One or more of NFSV3, NFSV4.",
			},
			"v4_domain": dschema.StringAttribute{
				Computed:    true,
				Description: "NFSv4 domain override for ID mapping. Empty string uses the system default.",
			},
			"v4_krb": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether Kerberos is required for NFSv4 mounts.",
			},
			"bindip": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses the NFS service is bound to. Empty list means all addresses.",
			},
			"mountd_port": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP/UDP port used by mountd. 0 means the default (dynamic) port.",
			},
			"rpcstatd_port": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP/UDP port used by rpc.statd. 0 means the default (dynamic) port.",
			},
			"rpclockd_port": dschema.Int64Attribute{
				Computed:    true,
				Description: "TCP/UDP port used by rpc.lockd. 0 means the default (dynamic) port.",
			},
			"mountd_log": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether mountd logging is enabled.",
			},
			"statd_lockd_log": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether rpc.statd and rpc.lockd logging is enabled.",
			},
			"userd_manage_gids": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether supplemental groups are resolved server-side (more than 16 groups).",
			},
			"rdma": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether NFS over RDMA is enabled.",
			},
			"managed_nfsd": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the NFS server daemon is currently managed/running.",
			},
			"v4_krb_enabled": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether Kerberos is currently enabled/active for NFSv4.",
			},
			"keytab_has_nfs_spn": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the system keytab currently has an NFS service principal name.",
			},
		},
	}
}

func (d *NFSConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NFSConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NFSConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "nfs.config")
	if err != nil {
		resp.Diagnostics.AddError("Read NFS configuration failed", err.Error())
		return
	}

	var api nfsConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse nfs.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
