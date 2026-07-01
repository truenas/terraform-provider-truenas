package dataset

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DatasetModel maps to the TrueNAS pool.dataset API fields.
type DatasetModel struct {
	// Terraform ID (set to dataset name)
	ID types.String `tfsdk:"id"`

	// Required
	Name types.String `tfsdk:"name"`

	// Optional with TrueNAS defaults
	Type        types.String `tfsdk:"type"`        // FILESYSTEM | VOLUME
	Compression types.String `tfsdk:"compression"`
	AClType     types.String `tfsdk:"acltype"`
	ShareType   types.String `tfsdk:"share_type"`
	Comments    types.String `tfsdk:"comments"`
	Quota       types.Int64  `tfsdk:"quota"`
	RefQuota    types.Int64  `tfsdk:"refquota"`
	Reservation types.Int64  `tfsdk:"reservation"`
	VolSize     types.Int64  `tfsdk:"volsize"` // VOLUME only

	// Computed
	MountPoint types.String `tfsdk:"mountpoint"`
	Encrypted  types.Bool   `tfsdk:"encrypted"`
	Pool       types.String `tfsdk:"pool"`
}

// apiPayload converts the model to the create/update JSON payload.
func (m *DatasetModel) apiPayload() map[string]any {
	p := map[string]any{"name": m.Name.ValueString()}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		p["type"] = strings.ToUpper(m.Type.ValueString())
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = strings.ToUpper(m.Compression.ValueString())
	}
	if !m.AClType.IsNull() && !m.AClType.IsUnknown() {
		p["acltype"] = strings.ToUpper(m.AClType.ValueString())
	}
	if !m.ShareType.IsNull() && !m.ShareType.IsUnknown() {
		p["share_type"] = strings.ToUpper(m.ShareType.ValueString())
	}
	if !m.Comments.IsNull() && !m.Comments.IsUnknown() {
		p["comments"] = m.Comments.ValueString()
	}
	if !m.Quota.IsNull() && !m.Quota.IsUnknown() {
		p["quota"] = m.Quota.ValueInt64()
	}
	if !m.RefQuota.IsNull() && !m.RefQuota.IsUnknown() {
		p["refquota"] = m.RefQuota.ValueInt64()
	}
	if !m.Reservation.IsNull() && !m.Reservation.IsUnknown() {
		p["reservation"] = m.Reservation.ValueInt64()
	}
	if !m.VolSize.IsNull() && !m.VolSize.IsUnknown() {
		p["volsize"] = m.VolSize.ValueInt64()
	}
	return p
}

// apiResponse holds the fields we read back from the TrueNAS API.
type apiResponse struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	MountPoint string `json:"mountpoint"`
	Encrypted  bool   `json:"encrypted"`
	Pool       string `json:"pool"`
	Properties struct {
		Compression struct{ Value string `json:"value"` } `json:"compression"`
		AClType     struct{ Value string `json:"value"` } `json:"acltype"`
		ShareType   struct{ Value string `json:"value"` } `json:"share_type"`
		Quota       struct{ Value string `json:"value"` } `json:"quota"`
		RefQuota    struct{ Value string `json:"value"` } `json:"refquota"`
		Reservation struct{ Value string `json:"value"` } `json:"reservation"`
		Comments    struct{ Value string `json:"value"` } `json:"org.freenas:description"`
		VolSize     struct {
			Parsed int64 `json:"parsed"`
		} `json:"volsize"`
	} `json:"properties"`
}
