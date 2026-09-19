// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ZvolModel is the Terraform state/plan model for a TrueNAS zvol.
type ZvolModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	VolSize      types.Int64  `tfsdk:"volsize"`
	VolBlockSize types.Int64  `tfsdk:"volblocksize"`
	Compression  types.String `tfsdk:"compression"`
	Sync         types.String `tfsdk:"sync"`
	Dedup        types.String `tfsdk:"dedup"`
	Sparse       types.Bool   `tfsdk:"sparse"`
	Comments     types.String `tfsdk:"comments"`
	Pool         types.String `tfsdk:"pool"`
	Encrypted    types.Bool   `tfsdk:"encrypted"`
}

// zvolAPI matches the flat JSON structure returned by pool.dataset.get_instance for zvols.
type zvolAPI struct {
	Name      string `json:"name"`
	Pool      string `json:"pool"`
	Encrypted bool   `json:"encrypted"`

	Compression struct {
		Parsed string `json:"parsed"`
	} `json:"compression"`

	Sync struct {
		Parsed string `json:"parsed"`
	} `json:"sync"`

	Dedup struct {
		Parsed string `json:"parsed"`
	} `json:"deduplication"` // note: API key is "deduplication", not "dedup"

	VolSize struct {
		Parsed int64 `json:"parsed"`
	} `json:"volsize"`

	VolBlockSize struct {
		Parsed int64 `json:"parsed"`
	} `json:"volblocksize"`

	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// apiPayload converts the model to the create/update JSON payload.
func (m *ZvolModel) apiPayload() map[string]any {
	p := map[string]any{
		"name":    m.Name.ValueString(),
		"type":    "VOLUME",
		"volsize": m.VolSize.ValueInt64(),
	}
	if !m.VolBlockSize.IsNull() && !m.VolBlockSize.IsUnknown() && m.VolBlockSize.ValueInt64() != 0 {
		p["volblocksize"] = m.VolBlockSize.ValueInt64()
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = strings.ToUpper(m.Compression.ValueString())
	}
	if !m.Sync.IsNull() && !m.Sync.IsUnknown() {
		p["sync"] = strings.ToUpper(m.Sync.ValueString())
	}
	if !m.Dedup.IsNull() && !m.Dedup.IsUnknown() {
		p["deduplication"] = strings.ToUpper(m.Dedup.ValueString())
	}
	if !m.Sparse.IsNull() && !m.Sparse.IsUnknown() {
		p["sparse"] = m.Sparse.ValueBool()
	}
	if !m.Comments.IsNull() && !m.Comments.IsUnknown() {
		p["comments"] = m.Comments.ValueString()
	}
	return p
}

// responseToModel populates m from the API response. Sparse is write-only
// (not returned by the API) so the plan/state value is preserved as-is.
func responseToModel(api *zvolAPI, m *ZvolModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Pool = types.StringValue(api.Pool)
	m.Encrypted = types.BoolValue(api.Encrypted)
	m.VolSize = types.Int64Value(api.VolSize.Parsed)
	m.VolBlockSize = types.Int64Value(api.VolBlockSize.Parsed)
	m.Compression = preserveCase(m.Compression, api.Compression.Parsed)
	m.Sync = preserveCase(m.Sync, api.Sync.Parsed)
	m.Dedup = preserveCase(m.Dedup, api.Dedup.Parsed)
	m.Comments = types.StringValue(api.UserProperties.Comments.Value)
	// Sparse is write-only (not in API response); preserve plan/state value.
}

// preserveCase returns current if it matches apiVal case-insensitively (preserving
// the user's chosen casing), or a lowercased apiVal otherwise (drift or first read).
func preserveCase(current types.String, apiVal string) types.String {
	if current.IsNull() || current.IsUnknown() {
		return types.StringValue(strings.ToLower(apiVal))
	}
	if strings.EqualFold(current.ValueString(), apiVal) {
		return current
	}
	return types.StringValue(strings.ToLower(apiVal))
}
