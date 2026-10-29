// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
				Optional:  true,
				Sensitive: true,
				Description: "Base64-encoded Kerberos keytab data to merge into the system keytab. Passed " +
					"through as-is — do not base64-encode it again. Marked Sensitive (kept out of plan/apply " +
					"output), but — unlike file_wo — it IS stored in state and read back on refresh (the API " +
					"returns it unmasked to a full admin; a read-only admin's masked value is ignored). Use file_wo " +
					"to keep the keytab out of state. Import never reads the keytab into state, since it cannot " +
					"tell whether the configuration uses file or file_wo: after importing a file-managed keytab, " +
					"the next apply records file with a one-time in-place update. Exactly one of file or file_wo " +
					"must be set. Generated configuration (`terraform plan -generate-config-out`) has this as null, because Terraform never writes sensitive or write-only values there; fill it in before applying.",
			},
			"file_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only alternative to file: the base64-encoded keytab read from configuration " +
					"and never stored in state, and not read back on refresh. Requires file_wo_version. Exactly " +
					"one of file or file_wo must be set.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("file_wo_version")),
				},
			},
			"file_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for file_wo. Bump to re-send a changed file_wo (a write-only " +
					"value is absent from state, so its change cannot be detected automatically). file_wo is sent on " +
					"create and, after that, only when this integer changes; other updates (a rename) leave the stored " +
					"keytab alone. Required when file_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("file_wo")),
				},
			},
		},
	}
}
