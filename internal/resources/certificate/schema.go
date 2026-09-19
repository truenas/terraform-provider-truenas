// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a certificate on TrueNAS (certificate.*). \"create_type\" selects one of four " +
			"immutable creation modes (changing it, like changing any other input field below, replaces the " +
			"resource — certificate.update only ever accepts \"name\", \"renew_days\", and " +
			"\"add_to_trusted_store\", confirmed live): CERTIFICATE_CREATE_IMPORTED (import an existing " +
			"certificate+private key pair), CERTIFICATE_CREATE_CSR (generate a new key pair and certificate " +
			"signing request on the box), CERTIFICATE_CREATE_IMPORTED_CSR (import an externally-generated CSR " +
			"and its private key), and CERTIFICATE_CREATE_ACME (register an ACME-managed certificate). Live " +
			"end-to-end ACME issuance is tested by this provider (TestAccCertificate_acmeIssuance, gated on " +
			"TRUENAS_ACME=1): a full DNS-01 order is driven against a real ACME CA — a Pebble test server — " +
			"using a truenas_acme_dns_authenticator (shell variant) that publishes the challenge, and the issued " +
			"certificate is verified. Renewal polling (the renew_days-driven auto-renew cycle) is the one ACME " +
			"behavior still not exercised by the test suite. " +
			"Never point this resource at certificate id 1 (or any certificate currently serving the TrueNAS " +
			"UI/API) — certificate.create always creates a new certificate; there is no way to adopt an " +
			"existing one short of `terraform import`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the certificate.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Certificate name. Must be unique and contain only alphanumeric characters, " +
					"dashes, and underscores. Updatable in place.",
			},
			"create_type": schema.StringAttribute{
				Required: true,
				Description: "Certificate creation mode: CERTIFICATE_CREATE_IMPORTED, CERTIFICATE_CREATE_CSR, " +
					"CERTIFICATE_CREATE_IMPORTED_CSR, or CERTIFICATE_CREATE_ACME. Immutable.",
				Validators: []validator.String{stringvalidator.OneOf(
					CreateTypeImported, CreateTypeCSR, CreateTypeImportedCSR, CreateTypeACME,
				)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"add_to_trusted_store": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to add this certificate to the trusted certificate store. Defaults to false. Updatable in place.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"renew_days": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Days before expiration to attempt renewal (1-30). Only supported when create_type " +
					"is CERTIFICATE_CREATE_ACME — rejected client-side (a clear preflight error) if set for any " +
					"other create_type: probed live, certificate.update rejects it outright for non-ACME " +
					"certificates (\"Certificate renewal days is only supported for ACME certificates\"), and " +
					"certificate.create silently ignores it and forces the default (10) instead of honoring a " +
					"caller-supplied value, either of which would otherwise surface as a confusing " +
					"provider-inconsistency error rather than a clear one. Defaults to 10. Updatable in place.",
				Validators: []validator.Int64{int64validator.Between(1, 30)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"certificate": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "PEM-encoded certificate. Required (and the certificate to import) when " +
					"create_type is CERTIFICATE_CREATE_IMPORTED. Read-only output (parsed subject/validity data " +
					"lives in the other computed fields) for every other create_type — null until a CSR is " +
					"signed. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"privatekey": schema.StringAttribute{
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				Description: "PEM-encoded private key. Required when create_type is CERTIFICATE_CREATE_IMPORTED " +
					"or CERTIFICATE_CREATE_IMPORTED_CSR (the key being imported); server-generated (and read " +
					"back here) for CERTIFICATE_CREATE_CSR. Marked Sensitive (kept out of plan/apply output and " +
					"logs), but — unlike a WriteOnly attribute — it IS stored in state and IS read back on " +
					"refresh/import: certificate.get_instance returns it byte-for-byte intact (probed live). The " +
					"job result returned directly by certificate.create/update masks this field as \"********\" " +
					"— this provider always re-reads via certificate.get_instance before saving state, so that " +
					"masked value is never persisted. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"csr": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "PEM-encoded certificate signing request. Required (the CSR being imported) when " +
					"create_type is CERTIFICATE_CREATE_IMPORTED_CSR. Server-generated and read back here when " +
					"create_type is CERTIFICATE_CREATE_CSR. Null for CERTIFICATE_CREATE_IMPORTED. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Cryptographic key algorithm for CERTIFICATE_CREATE_CSR: RSA or EC. Defaults to " +
					"RSA. Echoed back (parsed from the actual key) for every create_type. Immutable.",
				Validators: []validator.String{stringvalidator.OneOf("RSA", "EC")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key_length": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "RSA key length in bits: 2048 or 4096. Required when create_type is " +
					"CERTIFICATE_CREATE_CSR and key_type is RSA (the default) — confirmed live: TrueNAS rejects " +
					"an RSA CSR request with no key_length (\"RSA-based keys require an entry in this field\"). " +
					"Not applicable to EC keys (echoed back as the curve's bit size instead, e.g. 384 for " +
					"SECP384R1). Immutable.",
				Validators: []validator.Int64{int64validator.OneOf(2048, 4096)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			"ec_curve": schema.StringAttribute{
				Optional: true,
				Description: "Elliptic curve for CERTIFICATE_CREATE_CSR when key_type is EC: SECP256R1, " +
					"SECP384R1, SECP521R1, or ed25519. The API defaults to SECP384R1 when unset — confirmed " +
					"live: an EC CSR with no ec_curve set succeeds using this default, unlike RSA/key_length — " +
					"but since the API never echoes \"ec_curve\" back in any response (no such field in " +
					"certificate.query's output), this attribute is deliberately NOT Computed: it is taken " +
					"verbatim from config (null if you never set it) rather than read back or defaulted " +
					"server-side into state, the same write-only-by-convention treatment as \"passphrase\" " +
					"below. Immutable.",
				Validators: []validator.String{stringvalidator.OneOf("SECP256R1", "SECP384R1", "SECP521R1", "ed25519")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"passphrase": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Passphrase to protect the generated/imported private key. Write-only by " +
					"convention: certificate.create accepts it but it never appears in any create/update/" +
					"get_instance/query response, so — unlike \"privatekey\" — it cannot be read back and is not " +
					"marked Computed; the value you set here is simply preserved in state as-is. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"city": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "City or locality name for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"common": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Common name (CN) for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"country": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Country code for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Email address for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Organization name for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organizational_unit": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Organizational unit for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "State or province name for the certificate subject (CSR generation) or parsed from an imported certificate/CSR. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"digest_algorithm": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Hash algorithm for CSR/certificate signing: SHA224, SHA256, SHA384, or SHA512. Defaults to SHA256. Immutable.",
				Validators:  []validator.String{stringvalidator.OneOf("SHA224", "SHA256", "SHA384", "SHA512")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"san": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Subject alternative names. Required (at least one entry) when create_type is " +
					"CERTIFICATE_CREATE_CSR — confirmed live: TrueNAS rejects a CSR request with an empty/unset " +
					"\"san\" (\"List should have at least 1 item\"), even when \"common\" is also set. Parsed " +
					"back from an imported certificate/CSR for the other create_types. Immutable.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
					listplanmodifier.RequiresReplace(),
				},
			},
			"acme_directory_uri": schema.StringAttribute{
				Optional: true,
				Description: "ACME directory URI. Required when create_type is CERTIFICATE_CREATE_ACME. Live " +
					"end-to-end issuance against this URI is exercised by the provider's ACME acceptance test " +
					"(see the resource-level description). Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"csr_id": schema.Int64Attribute{
				Optional: true,
				Description: "ID of a truenas_certificate (create_type CERTIFICATE_CREATE_CSR) to use for ACME " +
					"issuance. Required when create_type is CERTIFICATE_CREATE_ACME. Immutable.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"tos": schema.BoolAttribute{
				Optional: true,
				Description: "Set to true to accept the ACME service's terms of service. Required (must be " +
					"true) when create_type is CERTIFICATE_CREATE_ACME. Immutable.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"dns_mapping": schema.MapAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "Map of domain name to truenas_acme_dns_authenticator ID, one entry per domain " +
					"listed in \"san\"/\"common\" of the target CSR. Required (at least one entry) when " +
					"create_type is CERTIFICATE_CREATE_ACME. Immutable.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			// certificate_path/privatekey_path/csr_path are deliberately NOT
			// given a UseStateForUnknown plan modifier: probed live, they
			// embed "name" (e.g. "/etc/certificates/<name>.crt"), so a
			// rename changes them. UseStateForUnknown would lock the plan to
			// the pre-rename path and fail Terraform's plan-consistency
			// check ("provider produced an inconsistent result after
			// apply") the moment "name" changes. Leaving them fully
			// Computed (no modifier) means they always show as "(known
			// after apply)" until the update actually runs, which is
			// correct here since they are update-time-variable. root_path/
			// cert_type/fingerprint/expired don't depend on "name" and are
			// left the same way for consistency/robustness rather than
			// re-introducing the same class of bug piecemeal.
			"certificate_path": schema.StringAttribute{
				Computed:    true,
				Description: "Filesystem path to the certificate file (.crt), or null if no certificate is available. Changes when \"name\" changes.",
			},
			"privatekey_path": schema.StringAttribute{
				Computed:    true,
				Description: "Filesystem path to the private key file (.key), or null if no private key is available. Changes when \"name\" changes.",
			},
			"csr_path": schema.StringAttribute{
				Computed:    true,
				Description: "Filesystem path to the certificate signing request file (.csr), or null if no CSR is available. Changes when \"name\" changes.",
			},
			"root_path": schema.StringAttribute{
				Computed:    true,
				Description: "Filesystem directory where certificate-related files are stored.",
			},
			"cert_type": schema.StringAttribute{
				Computed:    true,
				Description: "Human-readable certificate type, typically \"CERTIFICATE\".",
			},
			"fingerprint": schema.StringAttribute{
				Computed:    true,
				Description: "SHA-256 fingerprint of the signed certificate, or null for a CSR that has not yet been signed.",
			},
			"expired": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the certificate has expired, or null for a CSR that has not yet been signed.",
			},
		},
	}
}
