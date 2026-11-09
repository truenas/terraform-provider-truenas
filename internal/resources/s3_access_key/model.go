// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package s3_access_key manages TrueNAS S3 access keys (s3.accesskey.*),
// available from TrueNAS 27.0.
package s3_access_key

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// S3AccessKeyModel is the Terraform state model for truenas_s3_access_key.
//
// Secret handling (secrets-in-state class): the API returns the secret to
// administrators on every read, but the provider never reads it back into
// state. A user-supplied secret goes through the write-only `secret_wo`
// (never stored). A generated secret is stored in `secret` (Sensitive) from
// the create or rotate response only, as the one place the user can get it;
// Read and Update carry the state value forward, import leaves it null.
type S3AccessKeyModel struct {
	ID              types.Int64       `tfsdk:"id"`
	Name            types.String      `tfsdk:"name"`
	Username        types.String      `tfsdk:"username"`
	AccessKey       types.String      `tfsdk:"access_key"`
	Secret          types.String      `tfsdk:"secret"`
	SecretWO        types.String      `tfsdk:"secret_wo"`
	SecretWOVersion types.Int64       `tfsdk:"secret_wo_version"`
	RotateVersion   types.Int64       `tfsdk:"rotate_version"`
	Enabled         types.Bool        `tfsdk:"enabled"`
	ExpiresAt       normalize.RFC3339 `tfsdk:"expires_at"`
	ManageBuckets   types.Bool        `tfsdk:"manage_buckets"`
	UserIdentifier  types.String      `tfsdk:"user_identifier"`
	Local           types.Bool        `tfsdk:"local"`
	CreatedAt       types.String      `tfsdk:"created_at"`
	LastUsedAt      types.String      `tfsdk:"last_used_at"`
	Status          types.String      `tfsdk:"status"`
}

// S3AccessKeyDataSourceModel is the read-only model of the data source
// (looked up by name); it has no secret.
type S3AccessKeyDataSourceModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Username       types.String `tfsdk:"username"`
	AccessKey      types.String `tfsdk:"access_key"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	ExpiresAt      types.String `tfsdk:"expires_at"`
	ManageBuckets  types.Bool   `tfsdk:"manage_buckets"`
	UserIdentifier types.String `tfsdk:"user_identifier"`
	Local          types.Bool   `tfsdk:"local"`
	CreatedAt      types.String `tfsdk:"created_at"`
	LastUsedAt     types.String `tfsdk:"last_used_at"`
	Status         types.String `tfsdk:"status"`
}

// accessKeyAPI mirrors s3.accesskey.create/update/query rows. Timestamps are
// extended-JSON {"$date": ms}; user_identifier is an integer (local) or a
// SID string (directory services).
type accessKeyAPI struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Username       *string         `json:"username"`
	UserIdentifier json.RawMessage `json:"user_identifier"`
	Local          bool            `json:"local"`
	AccessKey      string          `json:"access_key"`
	Secret         *string         `json:"secret"`
	Enabled        bool            `json:"enabled"`
	ExpiresAt      json.RawMessage `json:"expires_at"`
	CreatedAt      json.RawMessage `json:"created_at"`
	LastUsedAt     json.RawMessage `json:"last_used_at"`
	ManageBuckets  bool            `json:"manage_buckets"`
	Status         string          `json:"status"`
}

// decodeDate decodes {"$date": ms} (or an RFC3339 string) to a UTC time;
// (nil, nil) for null.
func decodeDate(raw json.RawMessage) (*time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var dv struct {
		Date int64 `json:"$date"`
	}
	if err := json.Unmarshal(raw, &dv); err == nil {
		t := time.UnixMilli(dv.Date).UTC()
		return &t, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("datetime %q: %w", s, err)
		}
		t = t.UTC()
		return &t, nil
	}
	return nil, fmt.Errorf("cannot decode datetime %s", string(raw))
}

func dateString(t *time.Time) types.String {
	if t == nil {
		return types.StringNull()
	}
	return types.StringValue(t.Format(time.RFC3339))
}

// expiresAtPayload converts expires_at to the wire value: null when unset,
// {"$date": ms} otherwise.
func expiresAtPayload(v types.String) (any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v.IsNull() || v.IsUnknown() {
		return nil, diags
	}
	t, err := time.Parse(time.RFC3339, v.ValueString())
	if err != nil {
		diags.AddError("Invalid expires_at", fmt.Sprintf("expires_at %q is not a valid RFC3339 timestamp: %s", v.ValueString(), err))
		return nil, diags
	}
	return map[string]any{"$date": t.UnixMilli()}, diags
}

func userIdentifierString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String()
	}
	return string(raw)
}

// responseToModel maps an API row onto the model. Secret is never touched
// here (see S3AccessKeyModel).
func responseToModel(api *accessKeyAPI, m *S3AccessKeyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	if api.Username != nil {
		m.Username = types.StringValue(*api.Username)
	}
	m.AccessKey = types.StringValue(api.AccessKey)
	m.Enabled = types.BoolValue(api.Enabled)
	m.ManageBuckets = types.BoolValue(api.ManageBuckets)
	m.UserIdentifier = types.StringValue(userIdentifierString(api.UserIdentifier))
	m.Local = types.BoolValue(api.Local)
	m.Status = types.StringValue(api.Status)
	for _, f := range []struct {
		name string
		raw  json.RawMessage
		dst  *types.String
	}{{"created_at", api.CreatedAt, &m.CreatedAt}, {"last_used_at", api.LastUsedAt, &m.LastUsedAt}} {
		t, err := decodeDate(f.raw)
		if err != nil {
			diags.AddError("Invalid "+f.name+" in API response", err.Error())
			return diags
		}
		*f.dst = dateString(t)
	}
	exp, err := decodeDate(api.ExpiresAt)
	if err != nil {
		diags.AddError("Invalid expires_at in API response", err.Error())
		return diags
	}
	m.ExpiresAt = normalize.NewRFC3339PointerValue(dateString(exp).ValueStringPointer())
	return diags
}

func responseToDataSourceModel(api *accessKeyAPI, m *S3AccessKeyDataSourceModel) diag.Diagnostics {
	var full S3AccessKeyModel
	diags := responseToModel(api, &full)
	if diags.HasError() {
		return diags
	}
	m.ID, m.Name, m.Username, m.AccessKey = full.ID, full.Name, full.Username, full.AccessKey
	m.Enabled, m.ManageBuckets, m.UserIdentifier, m.Local = full.Enabled, full.ManageBuckets, full.UserIdentifier, full.Local
	m.CreatedAt, m.LastUsedAt, m.Status = full.CreatedAt, full.LastUsedAt, full.Status
	m.ExpiresAt = full.ExpiresAt.StringValue
	return diags
}

// createPayload builds the s3.accesskey.create argument. secretWO is the
// write-only secret from the configuration ("" when none: the server
// generates one).
func (m *S3AccessKeyModel) createPayload(secretWO string) (map[string]any, diag.Diagnostics) {
	exp, diags := expiresAtPayload(m.ExpiresAt.StringValue)
	if diags.HasError() {
		return nil, diags
	}
	p := map[string]any{
		"name":       m.Name.ValueString(),
		"username":   m.Username.ValueString(),
		"expires_at": exp,
	}
	if !m.AccessKey.IsNull() && !m.AccessKey.IsUnknown() && m.AccessKey.ValueString() != "" {
		p["access_key"] = m.AccessKey.ValueString()
	}
	if secretWO != "" {
		p["secret"] = secretWO
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.ManageBuckets.IsNull() && !m.ManageBuckets.IsUnknown() {
		p["manage_buckets"] = m.ManageBuckets.ValueBool()
	}
	return p, diags
}

// updatePayload builds the s3.accesskey.update argument. expires_at is
// always sent (null clears it); rotate is set when rotate_version changed.
func (m *S3AccessKeyModel) updatePayload(rotate bool) (map[string]any, diag.Diagnostics) {
	exp, diags := expiresAtPayload(m.ExpiresAt.StringValue)
	if diags.HasError() {
		return nil, diags
	}
	p := map[string]any{
		"name":       m.Name.ValueString(),
		"expires_at": exp,
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.ManageBuckets.IsNull() && !m.ManageBuckets.IsUnknown() {
		p["manage_buckets"] = m.ManageBuckets.ValueBool()
	}
	if rotate {
		p["rotate"] = true
	}
	return p, diags
}

// secretFollowsRotate keeps the stored secret on an ordinary update and
// plans it unknown when rotate_version changes (a rotation generates a new
// one, which the plan cannot know).
type secretFollowsRotate struct{}

func (secretFollowsRotate) Description(context.Context) string {
	return "Keeps the stored secret unless rotate_version changes."
}

func (m secretFollowsRotate) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (secretFollowsRotate) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || !req.PlanValue.IsUnknown() {
		return
	}
	var planned, prior types.Int64
	if d := req.Plan.GetAttribute(ctx, path.Root("rotate_version"), &planned); d.HasError() {
		return
	}
	if d := req.State.GetAttribute(ctx, path.Root("rotate_version"), &prior); d.HasError() {
		return
	}
	if planned.IsUnknown() || (!planned.IsNull() && !planned.Equal(prior)) {
		return // rotation: stays unknown
	}
	resp.PlanValue = req.StateValue
}
