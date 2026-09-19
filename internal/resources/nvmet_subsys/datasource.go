// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NVMetSubsysDataSource{}

// NVMetSubsysDataSource implements the truenas_nvmet_subsys data source.
type NVMetSubsysDataSource struct{ client *client.Client }

// NewDataSource returns a new NVMetSubsysDataSource.
func NewDataSource() datasource.DataSource { return &NVMetSubsysDataSource{} }

func (d *NVMetSubsysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_subsys"
}

func (d *NVMetSubsysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS NVMe-oF subsystem by name.",
		Attributes: map[string]dschema.Attribute{
			"id":             dschema.Int64Attribute{Computed: true, Description: "Numeric NVMe-oF subsystem ID."},
			"name":           dschema.StringAttribute{Required: true, Description: "NVMe-oF subsystem name to look up."},
			"subnqn":         dschema.StringAttribute{Computed: true, Description: "NVMe Qualified Name (NQN) for the subsystem."},
			"allow_any_host": dschema.BoolAttribute{Computed: true, Description: "Allow any host to connect to this subsystem without an explicit host mapping."},
			"ana":            dschema.BoolAttribute{Computed: true, Description: "Whether Asymmetric Namespace Access (ANA) is enabled for this subsystem."},
			"ieee_oui":       dschema.StringAttribute{Computed: true, Description: "IEEE Organizationally Unique Identifier used when generating namespace identifiers."},
			"pi_enable":      dschema.BoolAttribute{Computed: true, Description: "Whether end-to-end data protection (PI) is enabled for this subsystem."},
			"qid_max":        dschema.Int64Attribute{Computed: true, Description: "Maximum number of queues supported by this subsystem."},
			"serial":         dschema.StringAttribute{Computed: true, Description: "Serial number assigned by TrueNAS."},
		},
	}
}

func (d *NVMetSubsysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NVMetSubsysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NVMetSubsysDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by name: nvmet.subsys.query([["name","=","<name>"]])
	queryFilters := []any{[]any{"name", "=", state.Name.ValueString()}}
	raw, err := d.client.CallRead(ctx, "nvmet.subsys.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query NVMe-oF subsystems failed", err.Error())
		return
	}

	var results []nvmetSubsysAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"NVMe-oF subsystem not found",
			fmt.Sprintf("No NVMe-oF subsystem named %q was found.", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
