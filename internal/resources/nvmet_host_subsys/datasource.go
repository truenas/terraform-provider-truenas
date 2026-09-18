// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_subsys

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &HostSubsysDataSource{}

// HostSubsysDataSource implements the truenas_nvmet_host_subsys data
// source.
type HostSubsysDataSource struct{ client *client.Client }

// NewDataSource returns a new HostSubsysDataSource.
func NewDataSource() datasource.DataSource { return &HostSubsysDataSource{} }

func (d *HostSubsysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_host_subsys"
}

func (d *HostSubsysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NVMe-oF host/subsystem association by ID.",
		Attributes: map[string]dschema.Attribute{
			"id":        dschema.Int64Attribute{Required: true, Description: "Numeric NVMe-oF host/subsystem association ID to look up."},
			"host_id":   dschema.Int64Attribute{Computed: true, Description: "ID of the NVMe-oF host."},
			"subsys_id": dschema.Int64Attribute{Computed: true, Description: "ID of the NVMe-oF subsystem."},
		},
	}
}

func (d *HostSubsysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *HostSubsysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state HostSubsysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "nvmet.host_subsys.get_instance", state.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read NVMe-oF host/subsystem association failed", err.Error())
		return
	}

	var apiResp hostSubsysAPI
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
