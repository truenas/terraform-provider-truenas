// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &CertificateResource{}
var _ resource.ResourceWithValidateConfig = &CertificateResource{}
var _ resource.ResourceWithImportState = &CertificateResource{}
var _ resource.ResourceWithIdentity = &CertificateResource{}

// CertificateResource implements the truenas_certificate resource.
type CertificateResource struct{ client *client.Client }

// NewResource returns a new instance of CertificateResource.
func NewResource() resource.Resource { return &CertificateResource{} }

func (r *CertificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (r *CertificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *CertificateResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *CertificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// certificate.create/update/delete are all job:true (probed live and
// confirmed identical on TrueNAS 25.10/26.0); certificate.get_instance
// and certificate.query are plain (job:false) reads. See model.go's
// certificateAPI doc comment for why Create/Update always re-read via
// certificate.get_instance rather than trusting the job result directly
// (the job result masks "privatekey").

// ValidateConfig rejects store_privatekey = false for the imported create
// types at plan time. store_privatekey only governs whether a server-generated
// CSR key is read into state; for CERTIFICATE_CREATE_IMPORTED and
// CERTIFICATE_CREATE_IMPORTED_CSR the private key is the user's own
// configuration input, so keeping it out of state is meaningless and would hide
// a value the config already carries. (#49)
func (r *CertificateResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg CertificateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if storePrivatekeyFalseRejected(cfg.CreateType.ValueString(), cfg.StorePrivatekey) {
		resp.Diagnostics.AddAttributeError(
			path.Root("store_privatekey"),
			"store_privatekey = false is not valid for this create_type",
			"store_privatekey only applies to a server-generated CSR (create_type "+CreateTypeCSR+"). "+
				"For "+cfg.CreateType.ValueString()+" the private key is supplied in configuration, so it cannot "+
				"be kept out of state; remove store_privatekey or set it to true.",
		)
	}
}

func (r *CertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CertificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config CertificateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(plan.preflight()...)
	resp.Diagnostics.Append(validateRenewDays(plan.CreateType.ValueString(), config.RenewDays)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallJob(ctx, "certificate.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create certificate failed", err.Error())
		return
	}

	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// Re-read via get_instance: the job result above masks "privatekey" as
	// "********" (probed live), so it must never be used to populate state.
	raw2, err := r.client.CallRead(ctx, "certificate.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read after create failed", err.Error())
		return
	}

	var apiResp certificateAPI
	if err := json.Unmarshal(raw2, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "certificate.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read certificate failed", err.Error())
		return
	}

	var apiResp certificateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CertificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state CertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config CertificateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// create_type cannot have changed from a known value (it replaces); only the
	// renew_days-for-non-ACME check is re-run here, against Config (see
	// validateRenewDays's doc comment for why it must be Config, not Plan).
	resp.Diagnostics.Append(validateRenewDays(plan.CreateType.ValueString(), config.RenewDays)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.updatePayload()

	_, err := r.client.CallJob(ctx, "certificate.update", state.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update certificate failed", err.Error())
		return
	}

	// Re-read via get_instance for the same masking reason as Create.
	raw, err := r.client.CallRead(ctx, "certificate.get_instance", state.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp certificateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CertificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "certificate.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete certificate failed", err.Error())
	}
}

func (r *CertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "certificate.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import certificate failed", err.Error())
		return
	}

	var apiResp certificateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state CertificateModel
	// create_type is derived from the certificate (null for a CSR, see
	// importedCreateType). passphrase and the ACME-only input fields are
	// never returned by the API and are left null; their ReplaceFromKnown
	// plan modifiers record the configured value in place on the next apply
	// instead of replacing the certificate. "dns_mapping" needs its null value explicitly
	// typed (MapNull(Int64Type)): a bare zero-value types.Map — what an
	// uninitialized CertificateModel{} carries — has no element type
	// attached and fails to convert against the schema's typed map
	// attribute ("Received framework type ... MapType[!!! MISSING TYPE
	// !!!]"), confirmed live via `terraform import`.
	state.DNSMapping = types.MapNull(types.Int64Type)
	state.CreateType = importedCreateType(&apiResp)
	// store_privatekey has no API counterpart; a null value already reads as
	// the default true (responseToModel reads the key), so record that
	// explicitly. Left null, the Default made every import plan an update.
	state.StorePrivatekey = types.BoolValue(true)
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
