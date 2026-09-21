// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &FailoverConfigDataSource{}

// FailoverConfigDataSource implements the truenas_failover_config data
// source.
type FailoverConfigDataSource struct{ client *client.Client }

// NewDataSource returns a new FailoverConfigDataSource.
func NewDataSource() datasource.DataSource { return &FailoverConfigDataSource{} }

func (d *FailoverConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_failover_config"
}

func (d *FailoverConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Reads the current TrueNAS Enterprise HA failover configuration and live status. " +
			"Takes no arguments: there is exactly one failover configuration per TrueNAS system. Makes no " +
			"changes — see the truenas_failover_config resource's schema description for the safety notes " +
			"around the \"disabled\" and \"master\" fields. Reads cleanly even on a box that isn't licensed " +
			"for Enterprise HA (probed live): \"status\" reads \"SINGLE\" and \"node\" reads \"MANUAL\" in " +
			"that case.",
		Attributes: map[string]dschema.Attribute{
			"id": dschema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for this singleton: always \"failover_config\".",
			},
			"disabled": dschema.BoolAttribute{
				Computed:    true,
				Description: "Whether HA failover is currently administratively disabled on this system.",
			},
			"master": dschema.BoolAttribute{
				Computed: true,
				Description: "Whether this node in the chassis is currently marked master (active); the " +
					"standby node has the opposite value.",
			},
			"timeout": dschema.Int64Attribute{
				Computed: true,
				Description: "Time to wait, in seconds, before a failover occurs after a network event on an " +
					"interface marked critical for failover.",
			},
			"status": dschema.StringAttribute{
				Computed: true,
				Description: "Current HA status of this node (failover.status): one of \"MASTER\", " +
					"\"BACKUP\", \"ELECTING\", \"IMPORTING\", \"ERROR\", or \"SINGLE\" (a box that isn't " +
					"licensed/configured for HA).",
			},
			"node": dschema.StringAttribute{
				Computed: true,
				Description: "Slot position of this controller in the chassis (failover.node): \"A\", \"B\", " +
					"or \"MANUAL\" if the position could not be determined (e.g. a non-HA box).",
			},
			"disabled_reasons": dschema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Reasons why failover is currently disabled/non-functional " +
					"(failover.disabled.reasons). An empty list when failover is fully enabled and functional.",
			},
		},
	}
}

func (d *FailoverConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FailoverConfigDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state FailoverConfigDataSourceModel

	raw, err := d.client.CallRead(ctx, "failover.config")
	if err != nil {
		resp.Diagnostics.AddError("Read failover configuration failed", err.Error())
		return
	}
	var api failoverConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse failover.config response", err.Error())
		return
	}

	statusRaw, err := d.client.CallRead(ctx, "failover.status")
	if err != nil {
		resp.Diagnostics.AddError("Read failover.status failed", err.Error())
		return
	}
	var status string
	if err := json.Unmarshal(statusRaw, &status); err != nil {
		resp.Diagnostics.AddError("Parse failover.status response", err.Error())
		return
	}

	nodeRaw, err := d.client.CallRead(ctx, "failover.node")
	if err != nil {
		resp.Diagnostics.AddError("Read failover.node failed", err.Error())
		return
	}
	var node string
	if err := json.Unmarshal(nodeRaw, &node); err != nil {
		resp.Diagnostics.AddError("Parse failover.node response", err.Error())
		return
	}

	reasonsRaw, err := d.client.CallRead(ctx, "failover.disabled.reasons")
	if err != nil {
		resp.Diagnostics.AddError("Read failover.disabled.reasons failed", err.Error())
		return
	}
	var disabledReasons []string
	if err := json.Unmarshal(reasonsRaw, &disabledReasons); err != nil {
		resp.Diagnostics.AddError("Parse failover.disabled.reasons response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &api, status, node, disabledReasons, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
