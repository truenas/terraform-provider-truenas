// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// originalLabelPrivateKey is the private-state key this resource uses to
// stash the enclosure's label as observed immediately before this resource
// instance started managing it (via Create or ImportState — see
// resource.go's doc comments on both). Delete reads it back and restores it
// via enclosure.label.set before Terraform drops the resource from state.
//
// Private state (not a Computed schema attribute) is the deliberate choice
// here: it is never part of this resource's PUBLIC state surface — never
// shown by `terraform show`/`terraform state show`, never a plan-diff
// target, and never something a user could accidentally treat as durable
// API surface (e.g. reference from another resource/output, or hand-edit
// via `terraform state`). An "original_label" Computed attribute would
// invite exactly that: it looks like ordinary read-only resource data, but
// its entire purpose is bookkeeping for this resource's OWN Delete, not
// information about the enclosure a caller should consume. The framework's
// per-resource private state (persisted in the state file, scoped to this
// resource instance, opaque to Terraform core and to HCL) is built
// specifically for this "provider-internal bookkeeping that must survive to
// a later CRUD call" use case.
const originalLabelPrivateKey = "original_label"

// enclosureLabelAPI mirrors the subset of enclosure2.query's response shape
// this resource cares about. See internal/resources/enclosure's model.go
// for the datasource's full probed top-level shape (id, name, label, model,
// vendor, slot counts, etc.) — this resource intentionally decodes only the
// fields it manages ("label") or surfaces for read-only context ("name"),
// rather than importing that package: this provider has no cross-package
// resource imports (each internal/resources/* package is self-contained).
//
// "name" is decoded purely for context in state: probed live, it is the
// enclosure's fixed hardware/display name and is left completely unchanged
// by enclosure.label.set — see the truenas_enclosure_label resource's
// schema description for the full probe evidence.
type enclosureLabelAPI struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Name  string `json:"name"`
}

// EnclosureLabelModel is the Terraform state model for
// truenas_enclosure_label.
//
// "id" is Required + RequiresReplace (see schema.go): enclosure.label.set
// takes it as a positional argument identifying which pre-existing physical
// (or virtual, e.g. VirtualSES) enclosure to relabel — enclosures are never
// created or destroyed via this API, so "id" is this resource's true
// identity, not an assignable property, exactly mirroring the ipmi_lan
// package's "channel".
type EnclosureLabelModel struct {
	ID    types.String `tfsdk:"id"`
	Label types.String `tfsdk:"label"`
	Name  types.String `tfsdk:"name"`
}

// enclosureQueryArgs builds the two positional arguments (filters, options)
// enclosure2.query takes to look up a single enclosure by id. Mirrors
// internal/resources/enclosure's identically-named, independently-decided
// helper (see that package's model.go doc comment for the probe evidence
// that enclosure2.query follows the ordinary *.query filters/options
// convention, unlike ipmi.lan.query's single-"data"-object quirk).
func enclosureQueryArgs(id string) []any {
	return []any{[][]any{{"id", "=", id}}}
}

// responseToModel maps an enclosureLabelAPI response onto an
// EnclosureLabelModel.
func responseToModel(api *enclosureLabelAPI, m *EnclosureLabelModel) {
	m.ID = types.StringValue(api.ID)
	m.Label = types.StringValue(api.Label)
	m.Name = types.StringValue(api.Name)
}
