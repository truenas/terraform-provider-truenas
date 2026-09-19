// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &NVMetHostDataSource{}

// NVMetHostDataSource implements the truenas_nvmet_host data source.
//
// This datasource never exposes DH-CHAP secrets: its schema and model have
// no "dhchap_key" or "dhchap_ctrl_key" attributes.
type NVMetHostDataSource struct{ client *client.Client }

// NewDataSource returns a new NVMetHostDataSource.
func NewDataSource() datasource.DataSource { return &NVMetHostDataSource{} }

func (d *NVMetHostDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_host"
}

func (d *NVMetHostDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Looks up a TrueNAS NVMe-oF host (initiator) by hostnqn. " +
			"Never exposes DH-CHAP secrets: this datasource has no \"dhchap_key\" or \"dhchap_ctrl_key\" attribute.",
		Attributes: map[string]dschema.Attribute{
			"id":             dschema.Int64Attribute{Computed: true, Description: "Numeric NVMe-oF host ID assigned by TrueNAS."},
			"hostnqn":        dschema.StringAttribute{Required: true, Description: "NVMe Qualified Name (NQN) of the host (initiator) to look up."},
			"description":    dschema.StringAttribute{Computed: true, Description: "Free-form description of the host."},
			"dhchap_dhgroup": dschema.StringAttribute{Computed: true, Description: "DH-CHAP Diffie-Hellman group used for bidirectional authentication."},
			"dhchap_hash":    dschema.StringAttribute{Computed: true, Description: "DH-CHAP hash algorithm (SHA-256, SHA-384, SHA-512)."},
		},
	}
}

func (d *NVMetHostDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NVMetHostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state NVMetHostDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query by hostnqn: nvmet.host.query([["hostnqn","=","<hostnqn>"]])
	queryFilters := []any{[]any{"hostnqn", "=", state.HostNQN.ValueString()}}
	raw, err := d.client.CallRead(ctx, "nvmet.host.query", queryFilters)
	if err != nil {
		resp.Diagnostics.AddError("Query NVMe-oF hosts failed", err.Error())
		return
	}

	var results []nvmetHostAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"NVMe-oF host not found",
			fmt.Sprintf("No NVMe-oF host with hostnqn %q was found.", state.HostNQN.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
