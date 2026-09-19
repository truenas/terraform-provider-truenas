// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Attaches a device to a TrueNAS LXC container (container.device.*: create/update/" +
			"delete/query/get_instance). Requires TrueNAS 26.0 or later: the container.device namespace " +
			"does not exist on earlier releases (probed live — TrueNAS 25.10 exposes 0 container.device.* methods " +
			"via core.get_methods, matching truenas_container's own absence there). Using this resource against " +
			"an older server fails with a clean error during Create/Read/Update/Delete rather than a raw API " +
			"error." +
			"\n\n" +
			"\"attributes\" is a JSON document (jsonencode(...)) rather than typed nested attributes, mirroring " +
			"truenas_vm_device: container.device.create's own accepts schema is a \"dtype\"-discriminated union. " +
			"It must always include a \"dtype\" key. Probed live (TrueNAS 26.0) field shapes, by dtype:\n" +
			"  - FILESYSTEM (live-tested, full create/update/delete/query round trip): " +
			"\"source\" (host path, must resolve under a pool mount point, e.g. \"/mnt/tank/mydata\" — despite " +
			"the API's own schema marking it optional with a bogus \"/usr/bin/zsh\" default, omitting it is " +
			"always rejected live with \"[EINVAL] attributes.path: The path must reside within a pool mount " +
			"point\", so treat it as required in practice) and \"target\" (in-container mount point, e.g. " +
			"\"/data\" — genuinely optional; the API applies that same bogus \"/usr/bin/zsh\" literal when " +
			"omitted, a confirmed live API-side default bug, so always set it explicitly).\n" +
			"  - NIC (live-tested create/delete only): \"nic_attach\" (host bridge or interface name, e.g. " +
			"\"truenasbr0\" — see container.device.nic_attach_choices; null for no attachment), \"type\" " +
			"(\"E1000\" or \"VIRTIO\", defaults to \"E1000\"), \"mac\" (explicit MAC, or null to auto-generate — " +
			"confirmed live), \"trust_guest_rx_filters\" (bool, defaults to false).\n" +
			"  - USB (live-tested create/delete only): either \"device\" (host USB device path) or \"usb\" " +
			"(object: \"vendor_id\"/\"product_id\", hex strings like \"0x046b\"/\"0xff10\" — see " +
			"container.device.usb_choices) must be set; despite the API's schema marking both nullable/optional " +
			"with a \"controller only\" default description, omitting both is always rejected live with " +
			"\"[EINVAL] attributes.usb: Either device or product_id and vendor_id must be specified\".\n" +
			"  - GPU is NOT covered by this resource's probe evidence: container.device.gpu_choices returned no " +
			"entries in the environment this resource was verified against (no GPU hardware present), and " +
			"container.device.create validates \"pci_address\"/\"gpu_type\" against the real host GPU inventory " +
			"(confirmed live — a fabricated PCI address was rejected with several hardware-specific errors), so " +
			"no synthetic value could be safely probe-verified. This resource does not fabricate or document " +
			"GPU field mappings; on hardware with a real GPU, dtype=\"GPU\" with \"gpu_type\"/\"pci_address\" " +
			"keys may still work (container.device.create's schema does describe them) but is unverified by " +
			"this provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric container device ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"container": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Description: "ID of the truenas_container this device belongs to. Changing it replaces the " +
					"device (this provider's convention for parent/foreign-key fields, mirroring " +
					"truenas_webshare's \"path\", even though container.device.update's own accepts schema does " +
					"technically list \"container\" as an optional key, probed live).",
			},
			"attributes": schema.StringAttribute{
				Required:    true,
				Description: "JSON document of device attributes. Must include \"dtype\": FILESYSTEM, NIC, or USB. See the resource description above for probed field shapes.",
			},
		},
	}
}
