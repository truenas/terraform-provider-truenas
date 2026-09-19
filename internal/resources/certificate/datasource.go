// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &CertificateDataSource{}

// CertificateDataSource implements the truenas_certificate data source.
type CertificateDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of CertificateDataSource.
func NewDataSource() datasource.DataSource { return &CertificateDataSource{} }

func (d *CertificateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (d *CertificateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS certificate by name (certificate.create requires name to be unique).",
		Attributes: map[string]dschema.Attribute{
			"id":                   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the certificate."},
			"name":                 dschema.StringAttribute{Required: true, Description: "Name to look up."},
			"add_to_trusted_store": dschema.BoolAttribute{Computed: true, Description: "Whether this certificate is added to the trusted certificate store."},
			"renew_days":           dschema.Int64Attribute{Computed: true, Description: "Days before expiration to attempt renewal."},
			"certificate":          dschema.StringAttribute{Computed: true, Description: "PEM-encoded certificate, or null."},
			"privatekey":           dschema.StringAttribute{Computed: true, Sensitive: true, Description: "PEM-encoded private key, or null."},
			"csr":                  dschema.StringAttribute{Computed: true, Description: "PEM-encoded certificate signing request, or null."},
			"key_type":             dschema.StringAttribute{Computed: true, Description: "Cryptographic key algorithm, or null."},
			"key_length":           dschema.Int64Attribute{Computed: true, Description: "Key length in bits, or null."},
			"ec_curve":             dschema.StringAttribute{Computed: true, Description: "Elliptic curve, if applicable (not echoed back by the API, so always null via this datasource)."},
			"city":                 dschema.StringAttribute{Computed: true, Description: "City from the certificate subject, or null."},
			"common":               dschema.StringAttribute{Computed: true, Description: "Common name from the certificate subject, or null."},
			"country":              dschema.StringAttribute{Computed: true, Description: "Country from the certificate subject, or null."},
			"email":                dschema.StringAttribute{Computed: true, Description: "Email from the certificate subject, or null."},
			"organization":         dschema.StringAttribute{Computed: true, Description: "Organization from the certificate subject, or null."},
			"organizational_unit":  dschema.StringAttribute{Computed: true, Description: "Organizational unit from the certificate subject, or null."},
			"state":                dschema.StringAttribute{Computed: true, Description: "State from the certificate subject, or null."},
			"san": dschema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Subject alternative names.",
			},
			"certificate_path": dschema.StringAttribute{Computed: true, Description: "Filesystem path to the certificate file, or null."},
			"privatekey_path":  dschema.StringAttribute{Computed: true, Description: "Filesystem path to the private key file, or null."},
			"csr_path":         dschema.StringAttribute{Computed: true, Description: "Filesystem path to the CSR file, or null."},
			"root_path":        dschema.StringAttribute{Computed: true, Description: "Filesystem directory where certificate-related files are stored."},
			"cert_type":        dschema.StringAttribute{Computed: true, Description: "Human-readable certificate type."},
			"fingerprint":      dschema.StringAttribute{Computed: true, Description: "SHA-256 fingerprint, or null."},
			"expired":          dschema.BoolAttribute{Computed: true, Description: "Whether the certificate has expired, or null."},
		},
	}
}

func (d *CertificateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *CertificateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CertificateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "certificate.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query certificates failed", err.Error())
		return
	}

	var results []certificateAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Certificate not found",
			fmt.Sprintf("no certificate found with name %q", state.Name.ValueString()),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(ctx, &results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
