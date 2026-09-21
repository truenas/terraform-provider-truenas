// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the user-settable label of a single pre-existing TrueNAS storage enclosure " +
			"(enclosure.label.set, read back via enclosure2.query). Enclosures themselves are fixed physical (or " +
			"virtual, e.g. VirtualSES) hardware — never created or destroyed via this API, only relabeled — so " +
			"\"id\" identifies which existing enclosure this resource manages rather than an assignable " +
			"property, exactly mirroring the truenas_ipmi_lan resource's \"channel\". Terraform create/update " +
			"both call enclosure.label.set; see \"RESTORE ON DESTROY\" below for what Terraform delete does." +
			"\n\n" +
			"Probed live (TrueNAS 25.10.4 Enterprise HA): enclosure.label.set is synchronous (job: false) and the " +
			"new label is visible in the very next enclosure2.query call — no BMC-style settle-time/polling " +
			"behavior like truenas_ipmi_lan's \"vlan\". \"label\" is independent of the enclosure's fixed " +
			"\"name\": a live round trip (set a throwaway label, re-query, restore) left \"name\" completely " +
			"unchanged. Before any customization, \"label\" and \"name\" happen to hold the identical " +
			"factory-default string, which is why they can look interchangeable on an unmodified system." +
			"\n\n" +
			"RESTORE ON DESTROY: Create captures the enclosure's label AS FOUND, immediately before ever " +
			"calling enclosure.label.set, into Terraform's private resource state (see model.go's " +
			"originalLabelPrivateKey doc comment for why private state rather than a Computed attribute). " +
			"Terraform delete reads that captured value back and calls enclosure.label.set to restore it, " +
			"BEFORE the resource is removed from Terraform state — so running `terraform apply` then " +
			"`terraform destroy` on this resource leaves the enclosure's label exactly as it was found, never " +
			"stuck on whatever this resource last set it to. If no captured original can be found (should not " +
			"happen in ordinary use — every Create and Import populates it), Delete emits a WARNING and leaves " +
			"the label as its last-managed value rather than blocking the destroy outright, since Terraform " +
			"must always be able to complete a resource removal." +
			"\n\n" +
			"IMPORT: supported. `terraform import truenas_enclosure_label.example <id>` captures the label " +
			"found AT IMPORT TIME into private state, via the identical mechanism Create uses. This means a " +
			"Destroy that follows an Import restores the enclosure to \"whatever its label was the moment you " +
			"started letting Terraform manage it\" — which is the same guarantee Create provides (Create, too, " +
			"only ever observes the value immediately before it starts managing the resource, never some more " +
			"distant \"factory-original\" value). If the enclosure's label had already been customized before " +
			"import, a subsequent destroy is correctly a no-op restore to that pre-import value, not a reset to " +
			"the hardware's factory default.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
				Description: "Enclosure identifier to manage (enclosure2.query's own \"id\" field, e.g. " +
					"\"3b0ad6d1c00006e0\" — the same value enclosure.label.set takes; see the truenas_enclosure " +
					"datasource to look one up). Identifies a pre-existing enclosure; changing it targets a " +
					"different enclosure entirely rather than renaming this one, so it forces replacement.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"label": schema.StringAttribute{
				Required: true,
				Description: "New label to assign to the enclosure. Always sent on every apply — unlike " +
					"truenas_ipmi_lan's Optional+Computed fields, there is no meaningful \"leave the label " +
					"unmanaged\" state for a resource whose entire purpose is setting one. See the resource " +
					"description above for what happens to this value on destroy.",
			},
			"name": schema.StringAttribute{
				Computed: true,
				Description: "Fixed hardware/display name for this enclosure, read back from enclosure2.query " +
					"for context. Distinct from \"label\" — see the resource description above.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
