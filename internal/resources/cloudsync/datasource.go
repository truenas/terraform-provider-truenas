// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &CloudSyncDataSource{}

// CloudSyncDataSource implements the truenas_cloudsync_task data source.
type CloudSyncDataSource struct{ client *client.Client }

// NewDataSource returns a new CloudSyncDataSource.
func NewDataSource() datasource.DataSource { return &CloudSyncDataSource{} }

func (d *CloudSyncDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloudsync_task"
}

func (d *CloudSyncDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS cloud sync task by description. Cloud sync tasks have no name field; description serves as the lookup key.",
		Attributes: map[string]dschema.Attribute{
			"id":            dschema.Int64Attribute{Computed: true},
			"description":   dschema.StringAttribute{Required: true, Description: "Task description to look up."},
			"path":          dschema.StringAttribute{Computed: true},
			"credentials":   dschema.Int64Attribute{Computed: true},
			"direction":     dschema.StringAttribute{Computed: true},
			"transfer_mode": dschema.StringAttribute{Computed: true},
			"attributes":    dschema.StringAttribute{Computed: true},
			"schedule": dschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dschema.Attribute{
					"minute": dschema.StringAttribute{Computed: true},
					"hour":   dschema.StringAttribute{Computed: true},
					"dom":    dschema.StringAttribute{Computed: true},
					"month":  dschema.StringAttribute{Computed: true},
					"dow":    dschema.StringAttribute{Computed: true},
				},
			},
			"enabled":     dschema.BoolAttribute{Computed: true},
			"snapshot":    dschema.BoolAttribute{Computed: true},
			"include":     dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"exclude":     dschema.ListAttribute{Computed: true, ElementType: types.StringType},
			"pre_script":  dschema.StringAttribute{Computed: true},
			"post_script": dschema.StringAttribute{Computed: true},
		},
	}
}

func (d *CloudSyncDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CloudSyncDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CloudSyncModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "cloudsync.query",
		[]any{[]any{"description", "=", state.Description.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query cloud sync tasks failed", err.Error())
		return
	}

	var results []cloudSyncAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Cloud sync task not found",
			fmt.Sprintf("No cloud sync task with description %q was found.", state.Description.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrJSON, diags := apiAttributesJSON(&results[0])
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = types.StringValue(attrJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
