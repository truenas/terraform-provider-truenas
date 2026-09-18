// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// datasourceSchema returns the schema for the truenas_enclosure data source.
// enclosure is a DATASOURCE-ONLY namespace in this task's scope: the
// mutable surface TrueNAS exposes for an enclosure is exactly one field,
// its "label" (enclosure.label.set) — modeled by the separate
// truenas_enclosure_label resource, not here (see that package's schema.go).
// Every other field this datasource reports (model, vendor, slot counts,
// etc.) describes fixed physical hardware with no corresponding
// create/update/delete surface at all, so there is nothing else for a
// truenas_enclosure resource to manage (see provider.go's registration
// comment for how this is wired into the provider, mirroring
// docker_network/container_image).
func datasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Looks up a TrueNAS storage enclosure (e.g. the head unit's own chassis, or an " +
			"attached expansion shelf) by id (enclosure2.query). Read-only: describes fixed physical hardware " +
			"except for \"label\", which is user-settable but managed by the separate truenas_enclosure_label " +
			"resource, not here." +
			"\n\n" +
			"\"id\" is the required lookup key, not \"name\" or \"label\": both are user-customizable display " +
			"strings (\"label\" is directly settable via truenas_enclosure_label) with no uniqueness guarantee " +
			"on a system with multiple enclosures (e.g. a head unit plus expansion shelves), unlike \"id\", " +
			"which is also the same identifier enclosure.label.set itself takes." +
			"\n\n" +
			"Probed live on the disposable Enterprise HA test box (TrueNAS 25.10.4): a single shared H-series " +
			"chassis is reported (\"controller\": true — both HA controllers see the same physical enclosure). " +
			"Cross-release probe (TrueNAS 26.0): enclosure2.query returns an EMPTY array on a box with no " +
			"enclosure hardware/license — this datasource surfaces that as a clear \"not found\" error rather " +
			"than crashing." +
			"\n\n" +
			"Does NOT expose per-slot/per-component health detail (enclosure2.query's nested \"elements\" key): " +
			"out of scope per this provider's design (enclosure slot-level operations are explicitly excluded), " +
			"and its dynamic, doubly string-keyed map shape (element-type name, then a numeric slot/component " +
			"index encoded as a JSON object key) does not fit a static Terraform schema without an ad hoc " +
			"conversion nothing in this provider currently needs.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
				Description: "Enclosure identifier to look up (enclosure2.query's own \"id\" field, e.g. " +
					"\"3b0ad6d1c00006e0\" — the same value enclosure.label.set takes). Not a fixed constant: " +
					"probed live, this value was NOT what an earlier probe of the same physical box had recorded, " +
					"so always look it up (e.g. via the TrueNAS UI/API, or by iterating enclosure2.query with no " +
					"filter) rather than assuming a hardcoded id.",
			},
			"name": schema.StringAttribute{
				Computed: true,
				Description: "Fixed hardware/display name for this enclosure (e.g. \"BROADCOM VirtualSES 03\"). " +
					"Distinct from \"label\": probed live, setting \"label\" via enclosure.label.set left \"name\" " +
					"unchanged on the very next read. Before any customization the two happen to hold the " +
					"identical factory-default string, which is why they can look interchangeable on an " +
					"unmodified system.",
			},
			"label": schema.StringAttribute{
				Computed: true,
				Description: "Current user-settable label for this enclosure (enclosure2.query's \"label\" " +
					"field). Managed by the separate truenas_enclosure_label resource, not by this datasource.",
			},
			"model": schema.StringAttribute{
				Computed:    true,
				Description: "Enclosure model string, e.g. \"H10\".",
			},
			"controller": schema.BoolAttribute{
				Computed: true,
				Description: "True when this enclosure is the chassis housing the TrueNAS controller(s) " +
					"themselves (as opposed to an attached expansion shelf/JBOD).",
			},
			"vendor": schema.StringAttribute{
				Computed:    true,
				Description: "Enclosure vendor string, e.g. \"BROADCOM\".",
			},
			"product": schema.StringAttribute{
				Computed:    true,
				Description: "Enclosure product string, e.g. \"VirtualSES\".",
			},
			"revision": schema.StringAttribute{
				Computed:    true,
				Description: "Enclosure hardware revision string.",
			},
			"dmi": schema.StringAttribute{
				Computed:    true,
				Description: "DMI system identification string associating this enclosure with the host chassis, e.g. \"TRUENAS-H10-HA\".",
			},
			"bsg": schema.StringAttribute{
				Computed:    true,
				Description: "Linux \"bsg\" (block SCSI generic) device path for this enclosure's SES target, e.g. \"/dev/bsg/0:0:12:0\".",
			},
			"sg": schema.StringAttribute{
				Computed:    true,
				Description: "Linux SCSI generic device path for this enclosure's SES target, e.g. \"/dev/sg13\".",
			},
			"pci": schema.StringAttribute{
				Computed:    true,
				Description: "PCI address of the controller this enclosure is attached through, e.g. \"0:0:12:0\".",
			},
			"rackmount": schema.BoolAttribute{
				Computed:    true,
				Description: "True when this enclosure is a rackmount chassis.",
			},
			"front_loaded": schema.BoolAttribute{
				Computed:    true,
				Description: "True when this enclosure has front-loaded drive bays.",
			},
			"front_slots": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of front-loaded drive slots.",
			},
			"rear_slots": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of rear-loaded drive slots.",
			},
			"top_loaded": schema.BoolAttribute{
				Computed:    true,
				Description: "True when this enclosure has top-loaded drive bays.",
			},
			"top_slots": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of top-loaded drive slots.",
			},
			"internal_slots": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of internal (non-hot-swap) drive slots.",
			},
			"status": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Current overall status codes for this enclosure, e.g. [\"OK\"].",
			},
		},
	}
}
