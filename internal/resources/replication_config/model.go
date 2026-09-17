// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// replicationConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one replication configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const replicationConfigResourceID = "replication_config"

// ReplicationConfigModel is the Terraform state model for
// truenas_replication_config.
type ReplicationConfigModel struct {
	ID                          types.String `tfsdk:"id"`                             // fixed: "replication_config"
	MaxParallelReplicationTasks types.Int64  `tfsdk:"max_parallel_replication_tasks"` // nullable, three-way
}

// ReplicationConfigDataSourceModel is the read-only model for the
// truenas_replication_config datasource.
type ReplicationConfigDataSourceModel struct {
	ID                          types.String `tfsdk:"id"`
	MaxParallelReplicationTasks types.Int64  `tfsdk:"max_parallel_replication_tasks"`
}

// replicationConfigAPI mirrors the JSON object returned by
// replication.config.config and accepted (as a subset) by
// replication.config.update. max_parallel_replication_tasks is nullable on
// the wire (null means "unlimited"), so it is modeled as a pointer.
type replicationConfigAPI struct {
	ID                          int64  `json:"id"`
	MaxParallelReplicationTasks *int64 `json:"max_parallel_replication_tasks"`
}

// responseToModel maps an API response onto a Terraform model. A nil
// max_parallel_replication_tasks on the wire maps to Int64Value(0), the
// sentinel meaning "unlimited", to preserve the explicit-0 round trip: user
// sets 0 → payload sends nil → server stores nil → read-back maps to 0.
func responseToModel(_ context.Context, api *replicationConfigAPI, m *ReplicationConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(replicationConfigResourceID)

	if api.MaxParallelReplicationTasks != nil {
		m.MaxParallelReplicationTasks = types.Int64Value(*api.MaxParallelReplicationTasks)
	} else {
		m.MaxParallelReplicationTasks = types.Int64Value(0)
	}

	return diags
}

// responseToDataSourceModel maps an API response onto a
// ReplicationConfigDataSourceModel using the same nil-pointer-to-0 rule
// as responseToModel.
func responseToDataSourceModel(_ context.Context, api *replicationConfigAPI, m *ReplicationConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(replicationConfigResourceID)

	if api.MaxParallelReplicationTasks != nil {
		m.MaxParallelReplicationTasks = types.Int64Value(*api.MaxParallelReplicationTasks)
	} else {
		m.MaxParallelReplicationTasks = types.Int64Value(0)
	}

	return diags
}

// updatePayload builds the replication.config.update argument.
// max_parallel_replication_tasks is nullable on the wire: it is omitted
// entirely when null/unknown, sent as JSON nil when the model holds an
// explicit 0 (meaning "unlimited"), and sent as its value otherwise.
func (m *ReplicationConfigModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.MaxParallelReplicationTasks.IsNull() && !m.MaxParallelReplicationTasks.IsUnknown() {
		if v := m.MaxParallelReplicationTasks.ValueInt64(); v != 0 {
			p["max_parallel_replication_tasks"] = v
		} else {
			p["max_parallel_replication_tasks"] = nil
		}
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the replication configuration controls
// system-wide replication task concurrency, so removing this resource from
// Terraform state must never rewrite the box's configuration. Splitting
// this into its own function keeps Delete's "no client calls" contract
// independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Replication configuration left in place",
		"Replication configuration left in place; removed from Terraform state only",
	)
	return diags
}
