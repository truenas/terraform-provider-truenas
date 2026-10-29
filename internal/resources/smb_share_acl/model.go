// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// SMBShareACLModel is the Terraform state/plan model for a
// truenas_smb_share_acl resource: a share_name-keyed declarative wrapper
// around the imperative sharing.smb.setacl/getacl methods (plain calls, not
// jobs). Entries are a typed nested list; ae_perm/ae_type are read from the
// server, and the principal fields keep the configured spelling when they
// name the same principal (see mergeEntries).
type SMBShareACLModel struct {
	ID        types.String `tfsdk:"id"`         // == share_name
	ShareName types.String `tfsdk:"share_name"` // keyed by share name, not a numeric id
	ShareACL  types.List   `tfsdk:"share_acl"`  // list of smbAclEntryModel
}

// SMBShareACLDataSourceModel is the read-only lookup model for the
// truenas_smb_share_acl datasource (looked up by share_name).
type SMBShareACLDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	ShareName types.String `tfsdk:"share_name"`
	ShareACL  types.List   `tfsdk:"share_acl"`
}

// smbAclEntryModel is one Terraform-side share ACL entry. A configured entry
// names its principal by exactly one of ae_who_sid or ae_who_id; the server
// resolves the others, and all three are stored (Optional+Computed, name
// Computed only), so an import - which cannot know which selector the
// configuration used - plans no change (B-123). See mergeEntries.
type smbAclEntryModel struct {
	AePerm   types.String              `tfsdk:"ae_perm"`
	AeType   types.String              `tfsdk:"ae_type"`
	AeWhoSID normalize.CaseInsensitive `tfsdk:"ae_who_sid"`
	AeWhoID  types.Object              `tfsdk:"ae_who_id"`
	AeWhoStr types.String              `tfsdk:"ae_who_str"`
}

// smbAclWhoIDAttrTypes is the attribute-type map for the ae_who_id nested
// object ({id_type: "USER"|"GROUP", id: <uid/gid>}).
var smbAclWhoIDAttrTypes = map[string]attr.Type{
	"id_type": types.StringType,
	"id":      types.Int64Type,
}

// smbAclEntryAttrTypes is the attribute-type map for one share_acl list element.
var smbAclEntryAttrTypes = map[string]attr.Type{
	"ae_perm":    types.StringType,
	"ae_type":    types.StringType,
	"ae_who_sid": normalize.CaseInsensitiveType{},
	"ae_who_id":  types.ObjectType{AttrTypes: smbAclWhoIDAttrTypes},
	"ae_who_str": types.StringType,
}

// smbAclEntryObjectType is the element type of the share_acl list.
func smbAclEntryObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: smbAclEntryAttrTypes}
}

// --- wire (API) shapes for sharing.smb.getacl / setacl ---

// smbAclWhoIDAPI is the ae_who_id object on the wire.
type smbAclWhoIDAPI struct {
	IDType string `json:"id_type"`
	ID     int64  `json:"id"`
}

// smbAclEntryAPI is one entry on the wire. The ae_who_* fields are pointers
// with omitempty so absent selectors are omitted from setacl payloads (the
// API rejects an entry with no principal) and distinguished from empty on read.
type smbAclEntryAPI struct {
	AePerm   string          `json:"ae_perm"`
	AeType   string          `json:"ae_type"`
	AeWhoSID *string         `json:"ae_who_sid,omitempty"`
	AeWhoID  *smbAclWhoIDAPI `json:"ae_who_id,omitempty"`
	AeWhoStr *string         `json:"ae_who_str,omitempty"`
}

// smbGetAclAPI is the full sharing.smb.getacl / setacl response shape
// (setacl echoes the resolved ACL, so Create/Update parse its return directly
// instead of a follow-up getacl round trip).
type smbGetAclAPI struct {
	ShareName string           `json:"share_name"`
	ShareACL  []smbAclEntryAPI `json:"share_acl"`
}

// entriesFromList converts the model's share_acl list into a Go slice.
func entriesFromList(ctx context.Context, l types.List) ([]smbAclEntryModel, diag.Diagnostics) {
	var entries []smbAclEntryModel
	if l.IsNull() || l.IsUnknown() {
		return entries, nil
	}
	diags := l.ElementsAs(ctx, &entries, false)
	return entries, diags
}

// setaclPayload builds the sharing.smb.setacl argument from the configured
// entries. Only the principal selector set in the configuration is sent:
// ae_who_sid or ae_who_id (the server resolves the other; ae_who_str is
// read-only because sharing.smb.setacl cannot resolve a name, B-073). Callers
// pass the configuration, not the plan: the plan also carries the computed
// selectors from state.
func setaclPayload(ctx context.Context, m *SMBShareACLModel) (map[string]any, diag.Diagnostics) {
	entries, diags := entriesFromList(ctx, m.ShareACL)
	if diags.HasError() {
		return nil, diags
	}

	acl := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entry := map[string]any{
			"ae_perm": e.AePerm.ValueString(),
			"ae_type": e.AeType.ValueString(),
		}
		if !e.AeWhoSID.IsNull() && !e.AeWhoSID.IsUnknown() {
			entry["ae_who_sid"] = e.AeWhoSID.ValueString()
		}
		if !e.AeWhoID.IsNull() && !e.AeWhoID.IsUnknown() {
			var who smbAclWhoIDModel
			d := e.AeWhoID.As(ctx, &who, basetypes.ObjectAsOptions{})
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}
			entry["ae_who_id"] = map[string]any{
				"id_type": who.IDType.ValueString(),
				"id":      who.ID.ValueInt64(),
			}
		}
		acl = append(acl, entry)
	}

	return map[string]any{
		"share_name": m.ShareName.ValueString(),
		"share_acl":  acl,
	}, diags
}

// smbAclWhoIDModel decodes the ae_who_id object for payload building.
type smbAclWhoIDModel struct {
	IDType types.String `tfsdk:"id_type"`
	ID     types.Int64  `tfsdk:"id"`
}

// defaultShareACLPayload builds the setacl argument used by Delete: it resets
// the share to the server default of a single "everyone@ FULL ALLOWED" entry
// (probed live as sharing.smb.setacl's documented default share_acl), the SMB
// equivalent of filesystem_acl's strip-to-trivial Delete.
func defaultShareACLPayload(shareName string) map[string]any {
	return map[string]any{
		"share_name": shareName,
		"share_acl": []map[string]any{
			{"ae_perm": "FULL", "ae_type": "ALLOWED", "ae_who_sid": "S-1-1-0"},
		},
	}
}

// whoIDOf decodes a known, non-null ae_who_id object. ok is false for a null
// or unknown object.
func whoIDOf(o types.Object) (idType string, id int64, ok bool) {
	if o.IsNull() || o.IsUnknown() {
		return "", 0, false
	}
	attrs := o.Attributes()
	t, _ := attrs["id_type"].(types.String)
	n, _ := attrs["id"].(types.Int64)
	if t.IsUnknown() || n.IsUnknown() {
		return "", 0, false
	}
	return t.ValueString(), n.ValueInt64(), true
}

// knownString reports whether a value is known and non-null.
func knownString(v attr.Value) bool { return !v.IsNull() && !v.IsUnknown() }

// samePrincipal reports whether a prior (state or plan) entry and an API entry
// name the same principal. The server stores only the SID (sharing.smb.setacl
// resolves ae_who_id to a SID) and reports SIDs upper-case, so SIDs compare
// case-insensitively (B-074; ae_who_sid is a normalize.CaseInsensitive). Without a prior SID, a matching Unix ID counts;
// trustIndex additionally accepts an unresolved API ID (winbindd down) - used
// right after setacl, where entry i is the one just written.
func samePrincipal(p smbAclEntryModel, a smbAclEntryAPI, trustIndex bool) bool {
	if knownString(p.AeWhoSID) {
		return a.AeWhoSID != nil && strings.EqualFold(*a.AeWhoSID, p.AeWhoSID.ValueString())
	}
	_, pid, ok := whoIDOf(p.AeWhoID)
	if !ok {
		return false
	}
	if a.AeWhoID == nil {
		return trustIndex
	}
	return a.AeWhoID.ID == pid
}

// entryFromAPI converts one API entry, carrying every principal field the
// server resolved (SID, Unix ID and name).
func entryFromAPI(a smbAclEntryAPI) smbAclEntryModel {
	e := smbAclEntryModel{
		AePerm:   types.StringValue(a.AePerm),
		AeType:   types.StringValue(a.AeType),
		AeWhoSID: normalize.NewCaseInsensitivePointerValue(a.AeWhoSID),
		AeWhoID:  types.ObjectNull(smbAclWhoIDAttrTypes),
		AeWhoStr: types.StringNull(),
	}
	if a.AeWhoID != nil {
		e.AeWhoID = types.ObjectValueMust(smbAclWhoIDAttrTypes, map[string]attr.Value{
			"id_type": types.StringValue(a.AeWhoID.IDType),
			"id":      types.Int64Value(a.AeWhoID.ID),
		})
	}
	if a.AeWhoStr != nil {
		e.AeWhoStr = types.StringValue(*a.AeWhoStr)
	}
	return e
}

// mergeEntries builds state entries from the server's share ACL. ae_perm and
// ae_type always come from the server, so real drift shows in the plan. When
// entry i of prior names the same principal as server entry i, prior's spelling
// of the SID and its Unix ID are kept (the server upper-cases SIDs, may report
// a GROUP id as BOTH, and reports a null ID while winbindd is down), and a
// server name that did not resolve falls back to prior's. Otherwise the
// server's values are used as-is.
func mergeEntries(prior []smbAclEntryModel, api []smbAclEntryAPI, trustIndex bool) []smbAclEntryModel {
	out := make([]smbAclEntryModel, 0, len(api))
	for i, a := range api {
		e := entryFromAPI(a)
		if i < len(prior) && samePrincipal(prior[i], a, trustIndex) {
			p := prior[i]
			if knownString(p.AeWhoSID) {
				e.AeWhoSID = p.AeWhoSID
			}
			if _, _, ok := whoIDOf(p.AeWhoID); ok {
				e.AeWhoID = p.AeWhoID
			}
			if a.AeWhoStr == nil && knownString(p.AeWhoStr) {
				e.AeWhoStr = p.AeWhoStr
			}
			// After an apply (trustIndex), a planned null is kept as well:
			// the plan promised it, and an ID or name that resolves only now
			// is picked up by the next read.
			if trustIndex {
				if p.AeWhoID.IsNull() {
					e.AeWhoID = p.AeWhoID
				}
				if p.AeWhoStr.IsNull() {
					e.AeWhoStr = p.AeWhoStr
				}
			}
		}
		out = append(out, e)
	}
	return out
}

// apiEntriesToList converts API entries into a typed share_acl list carrying
// every resolved principal field. Used on import and by the data source.
func apiEntriesToList(ctx context.Context, api []smbAclEntryAPI) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(ctx, smbAclEntryObjectType(), mergeEntries(nil, api, false))
}

// idFor returns the resource id / identity for a share name.
func idFor(shareName string) string { return shareName }

// countSelectors counts how many principal selectors (ae_who_sid, ae_who_id)
// a configured entry sets. An unknown (interpolated) value counts as set so
// cross-resource references validate.
func countSelectors(e smbAclEntryModel) int {
	n := 0
	if !e.AeWhoSID.IsNull() {
		n++
	}
	if !e.AeWhoID.IsNull() {
		n++
	}
	return n
}

// diagText flattens diagnostics into a single string for wrapping in an error.
func diagText(diags diag.Diagnostics) string {
	if !diags.HasError() {
		return ""
	}
	msg := ""
	for _, d := range diags.Errors() {
		if msg != "" {
			msg += "; "
		}
		msg += d.Summary() + ": " + d.Detail()
	}
	return msg
}

// String is a tiny helper for readable test/debug output.
func (m *SMBShareACLModel) String() string {
	return fmt.Sprintf("smb_share_acl(share_name=%s)", m.ShareName.ValueString())
}
