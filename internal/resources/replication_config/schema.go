// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS replication configuration (system-wide replication task " +
			"concurrency). This is a singleton resource — there is exactly one replication configuration per " +
			"TrueNAS system, so it is never created or deleted on TrueNAS; Terraform create/update calls " +
			"replication.config.update, and Terraform delete only removes the resource from state (the " +
			"configuration is left in place).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"replication_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"max_parallel_replication_tasks": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Maximum number of replication tasks that may run in parallel. A value of 0 " +
					"clears the limit (unlimited, null on the wire).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}
