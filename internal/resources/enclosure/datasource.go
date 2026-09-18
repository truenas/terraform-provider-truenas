// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &EnclosureDataSource{}

// EnclosureDataSource implements the truenas_enclosure data source.
// enclosure is a DATASOURCE-ONLY namespace: see schema.go's
// datasourceSchema doc comment for why there is no corresponding
// truenas_enclosure resource (label management lives in the separate
// truenas_enclosure_label resource instead).
type EnclosureDataSource struct{ client *client.Client }

// NewDataSource returns a new EnclosureDataSource.
func NewDataSource() datasource.DataSource { return &EnclosureDataSource{} }

func (d *EnclosureDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enclosure"
}

func (d *EnclosureDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceSchema()
}

func (d *EnclosureDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EnclosureDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state EnclosureDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	raw, err := d.client.CallRead(ctx, "enclosure2.query", enclosureQueryArgs(id)...)
	if err != nil {
		resp.Diagnostics.AddError("Query enclosures failed", err.Error())
		return
	}

	var results []enclosureAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse enclosure2.query response", err.Error())
		return
	}

	// A zero-result id-filtered query is a clean "not found", not an API
	// error — probed live, this is exactly what enclosure2.query returns on
	// a box with no enclosure hardware/license at all (see model.go's doc
	// comment for the cross-release evidence), and it's equally the correct
	// outcome for a bad/stale id on a box that does have enclosures.
	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Enclosure not found",
			fmt.Sprintf("No enclosure with id %q was found. Note enclosure ids are not fixed constants across "+
				"boots/probes on some hardware (see the truenas_enclosure datasource's schema description); "+
				"look up the current id (e.g. via enclosure2.query with no filter) rather than assuming a "+
				"previously-recorded value is still valid. On a box with no enclosure hardware or Enterprise HA "+
				"license, enclosure2.query returns an empty result for every id.", id),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
