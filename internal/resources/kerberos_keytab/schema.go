// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a Kerberos keytab on TrueNAS (kerberos.keytab). A keytab holds one or more " +
			"Kerberos principal/key entries that are merged into the system keytab at /etc/krb5.keytab. Keytabs " +
			"are normally populated automatically during an Active Directory or IPA domain join (under reserved " +
			"names such as AD_MACHINE_ACCOUNT / IPA_MACHINE_ACCOUNT), but additional entries can also be managed " +
			"directly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the Kerberos keytab entry.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Name of the Kerberos keytab entry. This identifies the keytab entry itself, not " +
					"the name of any file — it is unrelated to the principal names inside the keytab data. Some " +
					"names are reserved for internal use (e.g. AD_MACHINE_ACCOUNT, IPA_MACHINE_ACCOUNT). " +
					"Updatable in place (renaming does not replace the resource).",
			},
			"file": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Base64-encoded Kerberos keytab data to merge into the system keytab. Passed " +
					"through as-is — do not base64-encode it again. " +
					"Marked Sensitive (kept out of plan/apply output and logs), but — unlike a WriteOnly " +
					"attribute — it IS stored in Terraform state and IS read back on refresh/import: a live " +
					"create -> get_instance -> query -> update round trip (using a real keytab exported from a " +
					"Samba AD domain controller via `samba-tool domain exportkeytab`) showed " +
					"kerberos.keytab.query/get_instance returning this value byte-for-byte intact, never " +
					"redacted or omitted, so the normal Sensitive+Computed-free modeling applies rather than the " +
					"WriteOnly pattern used for genuinely one-way secrets (e.g. truenas_user's password).",
			},
		},
	}
}
