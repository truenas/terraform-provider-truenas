// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a Webshare share on TrueNAS (sharing.webshare.*: create/update/delete/" +
			"get_instance/query) — a read-only HTTP file browser for a ZFS dataset path. Requires TrueNAS " +
			"26.0 or later: the sharing.webshare namespace does not exist on earlier releases (probed live — " +
			"TrueNAS 25.10 exposes 0 webshare.*/sharing.webshare.* methods via core.get_methods). Using this " +
			"resource against an older server fails with a clean error during Create/Read/Update rather than a " +
			"raw API error." +
			"\n\n" +
			"Unlike truenas_smb_share/truenas_nfs_share, Webshare shares have no \"comment\" field (probed live: " +
			"sharing.webshare.create/update reject an extra \"comment\" key with a clean EINVAL). See " +
			"truenas_webshare_config for the service-wide bind address/search/authentication configuration this " +
			"resource's shares are served under.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric Webshare share ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Webshare share name. Mutable: sharing.webshare.update accepts and applies a rename (probed live).",
			},
			"path": schema.StringAttribute{
				Required: true,
				Description: "Local server path to share by using the Webshare protocol. Must start with " +
					"\"/mnt/\" and be in a ZFS pool (e.g. /mnt/tank/share). Changing it replaces the share (this " +
					"provider's convention for share paths, mirroring truenas_smb_share/truenas_nfs_share, even " +
					"though sharing.webshare.update does accept a \"path\" key).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(true),
				Description:   "Whether the share is available. Defaults to true (matches the API's own default).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"is_home_base": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "If set, this share is used as the base path for user home directories. Only one " +
					"share on the system can have this enabled — setting it here on more than one " +
					"truenas_webshare resource, or on a system where another share already has it set outside " +
					"Terraform, produces undefined API-side behavior. Defaults to false.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			// Computed only — server generated, changes independently of Terraform.
			"dataset": schema.StringAttribute{
				Computed: true,
				Description: "Dataset name component of path (e.g. \"tank/share\"), or null if it cannot be " +
					"resolved. Probed live: this can read back null immediately after create — see the resource " +
					"documentation's known-quirks note — even though it settles to a real value on a subsequent " +
					"read or update.",
			},
			"relative_path": schema.StringAttribute{
				Computed: true,
				Description: "Relative path component within the dataset (e.g. \"subdir/data\", or \"\" when " +
					"path IS the dataset's own mountpoint), or null if it cannot be resolved. Subject to the same " +
					"create-time null quirk as \"dataset\" — see its description.",
			},
			"locked": schema.BoolAttribute{
				Computed: true,
				Description: "Whether the share path is currently on a locked (encrypted, unmounted) dataset, " +
					"or null if lock status is unavailable.",
			},
		},
	}
}
