// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &FilesystemPermissionsDataSource{}

// FilesystemPermissionsDataSource implements the
// truenas_filesystem_permissions data source.
type FilesystemPermissionsDataSource struct{ client *client.Client }

// NewDataSource returns a new FilesystemPermissionsDataSource.
func NewDataSource() datasource.DataSource { return &FilesystemPermissionsDataSource{} }

func (d *FilesystemPermissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystem_permissions"
}

func (d *FilesystemPermissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current UNIX mode/owner/group of a filesystem path via filesystem.stat.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.StringAttribute{Computed: true, Description: "Same as \"path\"."},
			"path": dschema.StringAttribute{Required: true, Description: "Absolute filesystem path to look up."},
			"mode": dschema.StringAttribute{Computed: true, Description: "UNIX permission bits in octal string form, e.g. \"0750\"."},
			"uid":  dschema.Int64Attribute{Computed: true, Description: "Numeric user ID of the path's owner."},
			"gid":  dschema.Int64Attribute{Computed: true, Description: "Numeric group ID of the path's group owner."},
		},
	}
}

func (d *FilesystemPermissionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FilesystemPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state FilesystemPermissionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "filesystem.stat", state.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read filesystem permissions failed", err.Error())
		return
	}

	var api fsStatAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse filesystem.stat response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&api, state.Path.ValueString(), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
