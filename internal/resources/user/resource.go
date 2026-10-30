// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}
var _ resource.ResourceWithIdentity = &UserResource{}
var _ resource.ResourceWithModifyPlan = &UserResource{}

// UserResource implements the truenas_user resource.
type UserResource struct{ client *client.Client }

// NewResource returns a new UserResource.
func NewResource() resource.Resource { return &UserResource{} }

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *UserResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes in req.Plan, so
	// the actual password value (if any) is only available via req.Config.
	var cfg UserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Password = cfg.Password

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.gateVersionedFields(ctx, payload)

	// user.create is job:false (sync); it returns either the created user
	// object or a bare integer id, depending on middleware version.
	raw, err := r.client.Call(ctx, "user.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create user failed", err.Error())
		return
	}

	apiResp, userID, err := decodeCreateResult(raw)
	if err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}
	if apiResp == nil {
		// Bare-int shape — read back full state via get_instance.
		raw2, err := r.client.CallRead(ctx, "user.get_instance", userID)
		if err != nil {
			resp.Diagnostics.AddError("Read-back after create failed", err.Error())
			return
		}
		apiResp = &userAPI{}
		if err := json.Unmarshal(raw2, apiResp); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "user.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read user failed", err.Error())
		return
	}

	var apiResp userAPI
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

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	// write-only: value lives in config, not plan.
	var cfg UserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if passwordResendNeeded(plan, state) {
		plan.Password = cfg.Password
	}

	payload, diags := plan.updatePayload(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.gateVersionedFields(ctx, payload)

	_, err := r.client.Call(ctx, "user.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update user failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "user.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp userAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "user.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete user failed", err.Error())
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "user.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import user failed", err.Error())
		return
	}

	var apiResp userAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state UserModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.importAdjust(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// gateVersionedFields removes payload fields that older TrueNAS releases do not
// accept. `webshare` was added in TrueNAS 26.0; sending it to 25.10 fails the
// whole user.create/update with "Extra inputs are not permitted". Below 26.0 it
// is dropped from the payload so the rest of the user still applies; that only
// drops a false value, because ModifyPlan rejects webshare = true there. If the
// version cannot be determined, the payload is left unchanged rather than
// silently dropping fields.
func (r *UserResource) gateVersionedFields(ctx context.Context, payload map[string]any) {
	if ok, err := r.client.VersionAtLeast(ctx, 26, 0); err == nil && !ok {
		delete(payload, "webshare")
	}
}

// ModifyPlan adds the plan-time checks that need the connected server or the
// prior state:
//   - webshare = true below TrueNAS 26.0 is an error. The API there has no
//     webshare field, so the value would be dropped and read back as false,
//     failing the apply with an inconsistent result. (B-055)
//   - a change to group_create on an existing user is a warning: the flag only
//     picks the primary group when the user is created, so the update records
//     the new value without changing the server. (B-142)
func (r *UserResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var cfgWebshare types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("webshare"), &cfgWebshare)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if r.client != nil && !cfgWebshare.IsNull() && !cfgWebshare.IsUnknown() && cfgWebshare.ValueBool() {
		if ok, err := r.client.VersionAtLeast(ctx, 26, 0); err == nil && !ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("webshare"),
				"webshare requires TrueNAS 26.0 or later",
				"The user webshare field does not exist on this TrueNAS release. Remove the attribute (or set it to false) or upgrade the server.",
			)
		}
	}

	if req.State.Raw.IsNull() {
		// Create (hunt F2): the API needs either group or group_create, and
		// either a password or password_disabled; the minimal configuration a
		// newcomer writes had neither and failed at apply with a raw API
		// message. Default the primary group the way the UI does (create one
		// named after the user) and make the password choice a plan-time error.
		var cfgGroup types.Int64
		var cfgGroupCreate, cfgPwDisabled types.Bool
		var cfgPassword types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("group"), &cfgGroup)...)
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("group_create"), &cfgGroupCreate)...)
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("password_disabled"), &cfgPwDisabled)...)
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("password"), &cfgPassword)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if cfgGroup.IsNull() && cfgGroupCreate.IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("group_create"), types.BoolValue(true))...)
		}
		if cfgPassword.IsNull() && (cfgPwDisabled.IsNull() || (!cfgPwDisabled.IsUnknown() && !cfgPwDisabled.ValueBool())) {
			resp.Diagnostics.AddAttributeError(
				path.Root("password"),
				"A new user needs a password or password_disabled = true",
				"TrueNAS requires one of the two when a user is created. Set password (write-only, Terraform >= 1.11) or password_disabled = true.",
			)
		}
		return
	}
	var stateGC, planGC types.Bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("group_create"), &stateGC)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("group_create"), &planGC)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !stateGC.IsNull() && !planGC.IsNull() && !planGC.IsUnknown() && !stateGC.Equal(planGC) {
		resp.Diagnostics.AddAttributeWarning(
			path.Root("group_create"),
			"group_create only applies when the user is created",
			"Changing group_create on an existing user does not change its primary group; the new value is only recorded in state. Set 'group' to change the primary group.",
		)
	}
}

// importAdjust fills in what an import cannot get from the user record alone:
//   - groups: with smb = true, user.create adds the builtin_users group, which
//     the configuration does not list. Create/Update/Read filter it out against
//     the configured list, but an import has no configured list, so it is
//     dropped here. (B-051)
//   - home_mode: user.query does not return it, so it is read from the mode of
//     the home directory when that is under /mnt (other homes, such as
//     /var/empty, are not created or chmod-ed by the server).
func (r *UserResource) importAdjust(ctx context.Context, api *userAPI, m *UserModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if api.SMB && len(api.Groups) > 0 {
		raw, err := r.client.CallRead(ctx, "group.query",
			[]any{[]any{"group", "=", "builtin_users"}, []any{"local", "=", true}},
			map[string]any{"select": []string{"id"}})
		if err != nil {
			diags.AddError("Look up builtin_users group failed", err.Error())
			return diags
		}
		var found []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &found); err != nil {
			diags.AddError("Parse group.query response", err.Error())
			return diags
		}
		if len(found) == 1 {
			kept := make([]int64, 0, len(api.Groups))
			for _, g := range api.Groups {
				if g != found[0].ID {
					kept = append(kept, g)
				}
			}
			gl, d := types.ListValueFrom(ctx, types.Int64Type, kept)
			diags.Append(d...)
			m.Groups = gl
		}
	}

	if strings.HasPrefix(api.Home, "/mnt/") {
		raw, err := r.client.CallRead(ctx, "filesystem.stat", api.Home)
		if err != nil {
			diags.AddWarning("Could not read home directory mode",
				fmt.Sprintf("filesystem.stat %s failed, so home_mode is left unset in state: %s", api.Home, err))
			return diags
		}
		var st struct {
			Mode int64 `json:"mode"`
		}
		if err := json.Unmarshal(raw, &st); err != nil {
			diags.AddError("Parse filesystem.stat response", err.Error())
			return diags
		}
		m.HomeMode = normalize.NewOctalModeValue(strconv.FormatInt(st.Mode&0o777, 8))
	}

	return diags
}
