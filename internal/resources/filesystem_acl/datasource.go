// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &FilesystemAclDataSource{}

// FilesystemAclDataSource implements the truenas_filesystem_acl data
// source.
type FilesystemAclDataSource struct{ client *client.Client }

// NewDataSource returns a new FilesystemAclDataSource.
func NewDataSource() datasource.DataSource { return &FilesystemAclDataSource{} }

func (d *FilesystemAclDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystem_acl"
}

func (d *FilesystemAclDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current ACL of a filesystem path via filesystem.getacl.",
		Attributes: map[string]dschema.Attribute{
			"id":      dschema.StringAttribute{Computed: true, Description: "Same as \"path\"."},
			"path":    dschema.StringAttribute{Required: true, Description: "Absolute filesystem path to look up."},
			"acltype": dschema.StringAttribute{Computed: true, Description: "ACL type in effect: \"NFS4\" or \"POSIX1E\"."},
			"entries": dschema.StringAttribute{Computed: true, Description: "JSON array of Access Control Entries. See the resource schema for the exact shape."},
			"uid":     dschema.Int64Attribute{Computed: true, Description: "Numeric user ID of the path's owner."},
			"gid":     dschema.Int64Attribute{Computed: true, Description: "Numeric group ID of the path's group owner."},
		},
	}
}

func (d *FilesystemAclDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FilesystemAclDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state FilesystemAclDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "filesystem.getacl", state.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read filesystem ACL failed", err.Error())
		return
	}

	var api fsGetAclAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse filesystem.getacl response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
