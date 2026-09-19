// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &VMwareDataSource{}

// VMwareDataSource implements the truenas_vmware data source.
//
// Looked up by "id": unlike truenas_app_registry ("name") or
// truenas_cloud_backup ("description"), vmware entries have no
// user-assigned display-name field at all (probed shape: id, datastore,
// filesystem, hostname, username, password, state — see model.go) — "id"
// is the only field guaranteed unique, mirroring truenas_enclosure's same
// "id"-as-lookup-key rationale.
//
// This datasource never exposes the VMware host password: its schema and
// model have no "password" attribute (mirrors internal/resources/
// app_registry's datasource, which omits "password" for the same reason).
type VMwareDataSource struct{ client *client.Client }

// NewDataSource returns a new VMwareDataSource.
func NewDataSource() datasource.DataSource { return &VMwareDataSource{} }

func (d *VMwareDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vmware"
}

func (d *VMwareDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Looks up a TrueNAS VMware snapshot integration entry by id. Never exposes the VMware " +
			"host password: this datasource has no \"password\" attribute.",
		Attributes: map[string]dschema.Attribute{
			"id":         dschema.Int64Attribute{Required: true, Description: "Numeric identifier of the VMware configuration to look up."},
			"datastore":  dschema.StringAttribute{Computed: true, Description: "Valid datastore name which exists on the VMware host."},
			"filesystem": dschema.StringAttribute{Computed: true, Description: "ZFS filesystem or dataset used for VMware storage."},
			"hostname":   dschema.StringAttribute{Computed: true, Description: "IP address / hostname of the VMware host."},
			"username":   dschema.StringAttribute{Computed: true, Description: "Credentials used to authorize access to the VMware host."},
			"state": dschema.SingleNestedAttribute{
				Computed:    true,
				Description: "Current connection and synchronization state with the VMware host.",
				Attributes: map[string]dschema.Attribute{
					"state":    dschema.StringAttribute{Computed: true, Description: "VMware host state (PENDING, SUCCESS, ERROR, or BLOCKED)."},
					"error":    dschema.StringAttribute{Computed: true, Description: "Error text from the last snapshot operation, if any."},
					"datetime": dschema.StringAttribute{Computed: true, Description: "Timestamp of the last state update, if any."},
				},
			},
		},
	}
}

func (d *VMwareDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VMwareDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state VMwareDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "vmware.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"VMware configuration not found",
				fmt.Sprintf("No VMware configuration with id %d was found.", state.ID.ValueInt64()),
			)
			return
		}
		resp.Diagnostics.AddError("Read VMware configuration failed", err.Error())
		return
	}

	var apiResp vmwareAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
