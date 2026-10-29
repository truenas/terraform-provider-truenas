// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a TrueNAS reporting exporter (reporting.exporters): a destination that " +
			"periodically receives reporting/metrics data. Only the GRAPHITE exporter type currently exists on " +
			"TrueNAS (probed via reporting.exporters.exporter_schemas on both TrueNAS 25.10 and 26.0), so " +
			"\"attributes\" exposes GRAPHITE's fields directly as a typed nested block rather than a free-form " +
			"JSON document.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric reporting exporter ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "User defined name of the exporter configuration.",
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether this exporter is enabled and active.",
			},
			"attributes": schema.SingleNestedAttribute{
				Required:    true,
				Description: "GRAPHITE exporter settings (the only exporter type TrueNAS currently supports).",
				Attributes: map[string]schema.Attribute{
					"destination_ip": schema.StringAttribute{
						Required:    true,
						Description: "IP address of the Graphite server.",
						Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
					},
					"destination_port": schema.Int64Attribute{
						Required:    true,
						Description: "Port number of the Graphite server.",
						Validators:  []validator.Int64{int64validator.Between(1, 65535)},
					},
					"namespace": schema.StringAttribute{
						Required:    true,
						Description: "Namespace to organize metrics under.",
						Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
					},
					"prefix": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Description: "Prefix to prepend to all metric names. Defaults to \"scale\"; removing it from " +
							"the configuration sets it back to \"scale\".",
						Default: stringdefault.StaticString("scale"),
					},
					"update_every": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Description: "Interval in seconds between metric updates. Defaults to 1 (also when removed from the configuration).",
						Validators:  []validator.Int64{int64validator.AtLeast(1)},
						Default:     int64default.StaticInt64(1),
					},
					"buffer_on_failures": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Description: "Number of updates to buffer when the Graphite server is unavailable. Defaults to 10 (also when removed from the configuration).",
						Validators:  []validator.Int64{int64validator.AtLeast(1)},
						Default:     int64default.StaticInt64(10),
					},
					"send_names_instead_of_ids": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Description: "Whether to send human-readable names instead of internal IDs. Defaults to true (also when removed from the configuration).",
						Default:     booldefault.StaticBool(true),
					},
					"matching_charts": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: "Pattern to match charts for export (supports wildcards). Defaults to \"*\" (also when removed from the configuration).",
						Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
						Default:     stringdefault.StaticString("*"),
					},
				},
			},
		},
	}
}
