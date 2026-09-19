// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_subsys

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &PortSubsysDataSource{}

// PortSubsysDataSource implements the truenas_nvmet_port_subsys data
// source.
type PortSubsysDataSource struct{ client *client.Client }

// NewDataSource returns a new PortSubsysDataSource.
func NewDataSource() datasource.DataSource { return &PortSubsysDataSource{} }

func (d *PortSubsysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_port_subsys"
}

func (d *PortSubsysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NVMe-oF port/subsystem association by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":        dschema.Int64Attribute{Required: true, Description: "Numeric NVMe-oF port/subsystem association ID to look up."},
			"port_id":   dschema.Int64Attribute{Computed: true, Description: "ID of the NVMe-oF port."},
			"subsys_id": dschema.Int64Attribute{Computed: true, Description: "ID of the NVMe-oF subsystem."},
		},
	}
}

func (d *PortSubsysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PortSubsysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state PortSubsysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "nvmet.port_subsys.get_instance", state.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read NVMe-oF port/subsystem association failed", err.Error())
		return
	}

	var apiResp portSubsysAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
