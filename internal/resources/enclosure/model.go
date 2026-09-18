// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// enclosureAPI mirrors the top-level shape of one element of the JSON array
// enclosure2.query returns. Probed live (TrueNAS 25.10.4 Enterprise HA,
// wss://10.220.16.188, the disposable Plan 20 Task 3 test box): a single
// shared H-series chassis (both HA controllers see the same enclosure,
// "controller": true):
//
//	{"id": "3b0ad6d1c00006e0", "name": "BROADCOM VirtualSES 03",
//	 "label": "BROADCOM VirtualSES 03", "model": "H10", "controller": true,
//	 "vendor": "BROADCOM", "product": "VirtualSES", "revision": "03",
//	 "dmi": "TRUENAS-H10-HA", "bsg": "/dev/bsg/0:0:12:0", "sg": "/dev/sg13",
//	 "pci": "0:0:12:0", "rackmount": true, "front_loaded": true,
//	 "front_slots": 12, "rear_slots": 0, "top_loaded": false, "top_slots": 0,
//	 "internal_slots": 0, "status": ["OK"], "elements": {...}}
//
// NOTE: the known enclosure id documented in this task's brief
// ("3b0ad6d1c00006c0") did NOT match the id actually present on the box at
// probe time ("3b0ad6d1c00006e0", confirmed live, a single result) — VirtualSES
// enclosure ids on this box are not a fixed constant across boots/probes, so
// this provider never hardcodes one; the datasource and resource both take
// "id" as a Required lookup key supplied by the caller.
//
// DECISIVE "label" vs "name" evidence: enclosure.label.set changes ONLY the
// top-level "label" key — a live round trip (set a throwaway label, re-query,
// restore) left "name" ("BROADCOM VirtualSES 03") completely unchanged while
// "label" changed to the throwaway value and back. Before any customization,
// "label" and "name" happen to hold the identical string (the box's factory
// default), which is why they can look interchangeable on an unmodified
// system, but they are genuinely two different fields.
//
// "elements" (the nested per-component/per-slot health map, keyed by
// element-type name — e.g. "Array Device Slot", "SAS Connector" — then by a
// numeric slot/component index encoded as a JSON object key, e.g. "1") is
// DELIBERATELY NOT decoded or exposed by this datasource: (a) it is entirely
// out of scope per the binding design doc ("enclosure slot-level operations"
// is explicitly excluded), and (b) its dynamic, doubly-nested string-keyed
// map shape does not fit Terraform's static attribute schema without an
// awkward, ad hoc conversion (unlike docker_network's small, fixed-shape
// "ipam" object) that nothing in this task's scope needs.
//
// enclosure.get_instance does NOT exist (confirmed live: "[EINVAL] Method
// does not exist" — enclosure2.query is the only read path, id-filtered via
// its normal two-positional-args filters/options form; unlike ipmi.lan.query,
// there is no single-"data"-object quirk here). enclosure.query (v1, no "2")
// also does not exist on either probed release.
//
// Cross-release probe (TrueNAS 26.0, wss://192.168.1.68): core.get_methods
// lists enclosure2.query/enclosure2.set_slot_status/enclosure.label.set in
// full there (confirming the 25.10.4 box's core.get_methods, which lists
// ONLY enclosure.label.set, under-reports this namespace the same way Task 1
// found for failover.* — every method here was direct-call-verified on
// 25.10.4 regardless of that listing). enclosure2.query itself returns an
// EMPTY array on the 26.0 box (no enclosure hardware/license there) — a
// clean empty result, not an error — so both packages in this task treat a
// zero-result id-filtered query as "not found" rather than assuming a
// non-empty result.
type enclosureAPI struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Label         string   `json:"label"`
	Model         string   `json:"model"`
	Controller    bool     `json:"controller"`
	Vendor        string   `json:"vendor"`
	Product       string   `json:"product"`
	Revision      string   `json:"revision"`
	DMI           string   `json:"dmi"`
	BSG           string   `json:"bsg"`
	SG            string   `json:"sg"`
	PCI           string   `json:"pci"`
	Rackmount     bool     `json:"rackmount"`
	FrontLoaded   bool     `json:"front_loaded"`
	FrontSlots    int64    `json:"front_slots"`
	RearSlots     int64    `json:"rear_slots"`
	TopLoaded     bool     `json:"top_loaded"`
	TopSlots      int64    `json:"top_slots"`
	InternalSlots int64    `json:"internal_slots"`
	Status        []string `json:"status"`
}

// EnclosureDataSourceModel is the read-only model for the truenas_enclosure
// datasource. "id" is the required lookup key (Required, not Computed):
// enclosure2.query's own "id" field is this provider's chosen identity (see
// enclosureAPI's doc comment) — the same value enclosure.label.set's "id"
// positional argument takes, and the truenas_enclosure_label resource's own
// identity. "name"/"label" are NOT used as an alternate lookup key: both are
// user-customizable display strings (probed live: "label" is directly
// settable via enclosure.label.set) with no uniqueness guarantee on a
// system, unlike "id".
type EnclosureDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Label         types.String `tfsdk:"label"`
	Model         types.String `tfsdk:"model"`
	Controller    types.Bool   `tfsdk:"controller"`
	Vendor        types.String `tfsdk:"vendor"`
	Product       types.String `tfsdk:"product"`
	Revision      types.String `tfsdk:"revision"`
	DMI           types.String `tfsdk:"dmi"`
	BSG           types.String `tfsdk:"bsg"`
	SG            types.String `tfsdk:"sg"`
	PCI           types.String `tfsdk:"pci"`
	Rackmount     types.Bool   `tfsdk:"rackmount"`
	FrontLoaded   types.Bool   `tfsdk:"front_loaded"`
	FrontSlots    types.Int64  `tfsdk:"front_slots"`
	RearSlots     types.Int64  `tfsdk:"rear_slots"`
	TopLoaded     types.Bool   `tfsdk:"top_loaded"`
	TopSlots      types.Int64  `tfsdk:"top_slots"`
	InternalSlots types.Int64  `tfsdk:"internal_slots"`
	Status        types.List   `tfsdk:"status"`
}

// enclosureQueryArgs builds the two positional arguments (filters, options)
// enclosure2.query takes to look up a single enclosure by id. Unlike
// ipmi.lan.query, enclosure2.query follows the ordinary *.query convention
// (confirmed live: a plain [["id","=",id]] filters array works, including
// alongside a normal {"select": [...]} options object).
func enclosureQueryArgs(id string) []any {
	return []any{[][]any{{"id", "=", id}}}
}

// stringListOrEmpty converts a possibly-nil []string from the API into a
// non-null types.List, matching the nil-guard convention used elsewhere for
// API-returned lists (e.g. audit_config's stringListOrEmpty).
func stringListOrEmpty(ctx context.Context, vals []string) (types.List, diag.Diagnostics) {
	if vals == nil {
		vals = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, vals)
}

// responseToDataSourceModel maps an enclosureAPI response onto an
// EnclosureDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *enclosureAPI, m *EnclosureDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Label = types.StringValue(api.Label)
	m.Model = types.StringValue(api.Model)
	m.Controller = types.BoolValue(api.Controller)
	m.Vendor = types.StringValue(api.Vendor)
	m.Product = types.StringValue(api.Product)
	m.Revision = types.StringValue(api.Revision)
	m.DMI = types.StringValue(api.DMI)
	m.BSG = types.StringValue(api.BSG)
	m.SG = types.StringValue(api.SG)
	m.PCI = types.StringValue(api.PCI)
	m.Rackmount = types.BoolValue(api.Rackmount)
	m.FrontLoaded = types.BoolValue(api.FrontLoaded)
	m.FrontSlots = types.Int64Value(api.FrontSlots)
	m.RearSlots = types.Int64Value(api.RearSlots)
	m.TopLoaded = types.BoolValue(api.TopLoaded)
	m.TopSlots = types.Int64Value(api.TopSlots)
	m.InternalSlots = types.Int64Value(api.InternalSlots)

	status, d := stringListOrEmpty(ctx, api.Status)
	diags.Append(d...)
	m.Status = status

	return diags
}
