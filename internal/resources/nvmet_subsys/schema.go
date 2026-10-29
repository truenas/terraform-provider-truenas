// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NVMe-oF subsystem on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NVMe-oF subsystem ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Name of the NVMe-oF subsystem. Renamed in place: the subsystem keeps its serial, " +
					"subnqn, namespaces and host/port mappings.",
			},
			"subnqn": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "NVMe Qualified Name (NQN) for the subsystem. Left unset, TrueNAS derives " +
					"`<basenqn>:<name>` at create time and keeps it afterwards (a rename does not change it). " +
					"\"\" is rejected: the server never stores an empty NQN, it derives one instead.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"allow_any_host": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow any host to connect to this subsystem without an explicit host mapping.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ana": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Asymmetric Namespace Access (ANA) for this subsystem.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ieee_oui": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IEEE Organizationally Unique Identifier used when generating namespace identifiers, as hex with a `0x` prefix (for example `0x005056`). Set to `\"\"` to clear.",
				Validators: []validator.String{
					// The kernel parses the value with base auto-detection: hex
					// without 0x (AABBCC) is silently ignored, and an all-digit
					// value is decimal (and the server drops leading zeros), so
					// "005056" would become OUI 0x0013c0. Only 0x-hex is
					// unambiguous and round-trips unchanged.
					stringvalidator.RegexMatches(regexp.MustCompile(`^(0[xX][0-9a-fA-F]{1,6})?$`),
						"must be a hex OUI with a 0x prefix, such as 0x005056, or \"\" to clear"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pi_enable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable end-to-end data protection (PI) for this subsystem.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"qid_max": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Maximum number of queues supported by this subsystem.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			// Computed-only (server-generated)
			"serial": schema.StringAttribute{
				Computed:    true,
				Description: "Serial number assigned by TrueNAS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
