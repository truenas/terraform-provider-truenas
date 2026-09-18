// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS system dataset configuration (the dataset that holds core " +
			"system state such as logs, reporting, syslog, and samba4 data). This is a singleton resource — " +
			"there is exactly one system dataset configuration per TrueNAS system, so it is never created or " +
			"deleted on TrueNAS; Terraform create/update calls systemdataset.update (a long-running job that " +
			"may migrate the system dataset between pools), and Terraform delete only removes the resource " +
			"from state (the configuration is left in place). WARNING: changing pool moves the system dataset " +
			"to a different pool, which is a disruptive, long-running operation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"system_dataset\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pool": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Name of the pool the system dataset lives on. WARNING: changing this migrates the system dataset to the new pool, a disruptive, long-running operation.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pool_exclude": schema.StringAttribute{
				Optional: true,
				Description: "Name of a pool to exclude from automatic selection when migrating the system " +
					"dataset. Write-only: never read back from TrueNAS, and only sent to systemdataset.update " +
					"when explicitly set.",
			},
			// Computed-only (server-generated)
			"basename": schema.StringAttribute{
				Computed:    true,
				Description: "Base path of the system dataset (e.g. \"tank/.system\"). Changes whenever pool changes; server-computed, never sent to systemdataset.update.",
			},
			"path": schema.StringAttribute{
				Computed:    true,
				Description: "Filesystem path the system dataset is mounted at (e.g. \"/var/db/system\"). Server-computed, never sent to systemdataset.update.",
			},
			"uuid": schema.StringAttribute{
				Computed:      true,
				Description:   "UUID of the system dataset. Server-computed, never sent to systemdataset.update.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pool_set": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the system dataset pool has been explicitly set. Server-computed, never sent to systemdataset.update.",
			},
		},
	}
}
