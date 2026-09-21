// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NVMeTGlobalDataSource{}

// NVMeTGlobalDataSource implements the truenas_nvmet_global data source.
type NVMeTGlobalDataSource struct{ client *client.Client }

// NewDataSource returns a new NVMeTGlobalDataSource.
func NewDataSource() datasource.DataSource { return &NVMeTGlobalDataSource{} }

func (d *NVMeTGlobalDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_global"
}

func (d *NVMeTGlobalDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS global NVMe-oF target configuration. Takes no arguments: " +
			"there is exactly one NVMe-oF global configuration per TrueNAS system.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"nvmet_global\".",
			},
			"basenqn": dschema.StringAttribute{
				Computed:    true,
				Description: "Base NVMe Qualified Name (NQN) prefix used when generating subsystem NQNs.",
			},
			"ana": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether Asymmetric Namespace Access (ANA) is enabled.",
			},
			"kernel": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the kernel-mode NVMe-oF target implementation is used.",
			},
			"rdma": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether NVMe-oF RDMA transport is enabled.",
			},
			"xport_referral": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether transport referral is enabled.",
			},
		},
	}
}

func (d *NVMeTGlobalDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NVMeTGlobalDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NVMeTGlobalDataSourceModel

	raw, err := d.client.CallRead(ctx, "nvmet.global.config")
	if err != nil {
		resp.Diagnostics.AddError("Read NVMe-oF global configuration failed", err.Error())
		return
	}

	var api nvmetGlobalAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse nvmet.global.config response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
