// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS audit configuration (audit.config): retention and dataset " +
			"quota settings for the local audit databases. This is a singleton resource — there is exactly one " +
			"audit configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls audit.update, and Terraform delete only removes the resource from state (the " +
			"configuration is left in place). There is no top-level enable/disable toggle: auditing is enabled " +
			"per-service (e.g. SMB share audit settings, sudo configuration) and is not managed by this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"audit_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"retention": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of days to retain local audit messages. Range 1-30.",
				Validators:  []validator.Int64{int64validator.Between(1, 30)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"reservation": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Size in GiB of refreservation to set on the ZFS dataset where the audit databases " +
					"are stored. Range 0-100.",
				Validators: []validator.Int64{int64validator.Between(0, 100)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"quota": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Size in GiB of the maximum amount of space that may be consumed by the dataset " +
					"where the audit databases are stored. Range 0-100.",
				Validators: []validator.Int64{int64validator.Between(0, 100)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"quota_fill_warning": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Percentage used of dataset quota at which to generate a warning alert. Range 5-80.",
				Validators:  []validator.Int64{int64validator.Between(5, 80)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"quota_fill_critical": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Percentage used of dataset quota at which to generate a critical alert. Range 50-95.",
				Validators:  []validator.Int64{int64validator.Between(50, 95)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"remote_logging_enabled": schema.BoolAttribute{
				Computed: true,
				Description: "Read-only: whether logging to a remote syslog server is enabled on TrueNAS, and " +
					"audit logs are included in what is sent remotely. Not settable via audit.update.",
			},
			"space": schema.SingleNestedAttribute{
				Computed: true,
				Description: "Read-only ZFS dataset space accounting for where the audit databases are " +
					"stored, in bytes.",
				Attributes: map[string]schema.Attribute{
					"used":                schema.Int64Attribute{Computed: true, Description: "Total space used by the audit dataset, in bytes."},
					"used_by_dataset":     schema.Int64Attribute{Computed: true, Description: "Space used by the dataset itself (excluding snapshots/reservations), in bytes."},
					"used_by_reservation": schema.Int64Attribute{Computed: true, Description: "Space reserved for the dataset, in bytes."},
					"used_by_snapshots":   schema.Int64Attribute{Computed: true, Description: "Space used by snapshots of the audit dataset, in bytes."},
					"available":           schema.Int64Attribute{Computed: true, Description: "Available space remaining for the audit dataset, in bytes."},
				},
			},
			"enabled_services": schema.SingleNestedAttribute{
				Computed: true,
				Description: "Read-only summary of what is currently being audited per service. Driven by " +
					"other resources (e.g. SMB share audit settings, sudo configuration), not by this resource.",
				Attributes: map[string]schema.Attribute{
					"middleware": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "Middleware audit event types that are currently enabled.",
					},
					"smb": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "SMB share names for which auditing is currently enabled.",
					},
					"sudo": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
						Description: "Sudo commands or users currently being audited.",
					},
				},
			},
		},
	}
}
