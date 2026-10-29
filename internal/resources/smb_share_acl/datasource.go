// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

var _ datasource.DataSource = &SMBShareACLDataSource{}

// SMBShareACLDataSource implements the truenas_smb_share_acl data source.
type SMBShareACLDataSource struct{ client *client.Client }

// NewDataSource returns a new SMBShareACLDataSource.
func NewDataSource() datasource.DataSource { return &SMBShareACLDataSource{} }

func (d *SMBShareACLDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smb_share_acl"
}

func (d *SMBShareACLDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the share-level ACL of an existing SMB share via sharing.smb.getacl.",
		Attributes: map[string]dschema.Attribute{
			"id":         dschema.StringAttribute{Computed: true, Description: "Same as \"share_name\"."},
			"share_name": dschema.StringAttribute{Required: true, Description: "Name of the SMB share to look up."},
			"share_acl": dschema.ListNestedAttribute{
				Computed:    true,
				Description: "Ordered list of share ACL entries. Each entry carries every principal field the server resolved: SID, Unix ID (null when the SID maps to none) and name.",
				NestedObject: dschema.NestedAttributeObject{
					Attributes: map[string]dschema.Attribute{
						"ae_perm":    dschema.StringAttribute{Computed: true, Description: "Permission level: FULL, CHANGE, READ, or CUSTOM."},
						"ae_type":    dschema.StringAttribute{Computed: true, Description: "ALLOWED or DENIED."},
						"ae_who_sid": dschema.StringAttribute{Computed: true, CustomType: normalize.CaseInsensitiveType{}, Description: "Principal SID."},
						"ae_who_id": dschema.SingleNestedAttribute{
							Computed:    true,
							Description: "Principal Unix ID, or null when the SID maps to none.",
							Attributes: map[string]dschema.Attribute{
								"id_type": dschema.StringAttribute{Computed: true, Description: "USER or GROUP."},
								"id":      dschema.Int64Attribute{Computed: true, Description: "Numeric UID or GID."},
							},
						},
						"ae_who_str": dschema.StringAttribute{Computed: true, Description: "Principal user or group name, or null when it does not resolve."},
					},
				},
			},
		},
	}
}

func (d *SMBShareACLDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SMBShareACLDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state SMBShareACLDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	shareName := state.ShareName.ValueString()
	raw, err := d.client.CallRead(ctx, "sharing.smb.getacl", map[string]any{"share_name": shareName})
	if err != nil {
		resp.Diagnostics.AddError("Read SMB share ACL failed", err.Error())
		return
	}

	var api smbGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse sharing.smb.getacl response", err.Error())
		return
	}

	list, d2 := apiEntriesToList(ctx, api.ShareACL)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(idFor(shareName))
	state.ShareName = types.StringValue(shareName)
	state.ShareACL = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
