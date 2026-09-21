// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// VdevModel represents a single vdev (virtual device) in Terraform state.
type VdevModel struct {
	Type  types.String `tfsdk:"type"`
	Disks types.List   `tfsdk:"disks"` // List[String]
}

// vdevAttrTypes describes the attribute types of a single vdev object, used
// to build types.Object/types.List values for the datasource model (see
// topologyAttrTypes and PoolDataSourceModel).
var vdevAttrTypes = map[string]attr.Type{
	"type":  types.StringType,
	"disks": types.ListType{ElemType: types.StringType},
}

// topologyAttrTypes describes the attribute types of the nested "topology"
// object. It is used to marshal a TopologyModel into a types.Object for the
// datasource model, which must be able to represent topology as null (the
// datasource's config never sets it - it's Computed-only).
var topologyAttrTypes = map[string]attr.Type{
	"data":  types.ListType{ElemType: types.ObjectType{AttrTypes: vdevAttrTypes}},
	"log":   types.ListType{ElemType: types.ObjectType{AttrTypes: vdevAttrTypes}},
	"cache": types.ListType{ElemType: types.StringType},
	"spare": types.ListType{ElemType: types.StringType},
}

// TopologyModel represents the pool topology in Terraform state. Every field
// is a types.List (not a bare []VdevModel) so it can hold null/unknown: the
// data/log/cache/spare attributes are Optional+Computed, so a config that
// omits one (e.g. no log vdev) hands the framework an *unknown* value for it,
// which a plain Go slice cannot represent (issue #7).
type TopologyModel struct {
	Data  types.List `tfsdk:"data"`  // List[Object{type,disks}]
	Log   types.List `tfsdk:"log"`   // List[Object{type,disks}]
	Cache types.List `tfsdk:"cache"` // List[String] - flat disk names
	Spare types.List `tfsdk:"spare"` // List[String] - flat disk names
}

// vdevObjectType is the element type of the data/log vdev lists.
var vdevObjectType = types.ObjectType{AttrTypes: vdevAttrTypes}

// PoolModel is the Terraform state model for a ZFS pool resource. Topology
// is Required in the resource schema, so it is never null in practice and
// can be modeled as a bare struct.
type PoolModel struct {
	ID        types.Int64   `tfsdk:"id"`
	Name      types.String  `tfsdk:"name"`
	Topology  TopologyModel `tfsdk:"topology"`
	AutoTrim  types.Bool    `tfsdk:"autotrim"`
	GUID      types.String  `tfsdk:"guid"`
	Status    types.String  `tfsdk:"status"`
	Healthy   types.Bool    `tfsdk:"healthy"`
	Path      types.String  `tfsdk:"path"`
	Size      types.Int64   `tfsdk:"size"`
	Free      types.Int64   `tfsdk:"free"`
	Allocated types.Int64   `tfsdk:"allocated"`
}

// PoolDataSourceModel is the Terraform state model for the truenas_pool
// datasource. Topology is Computed-only in the datasource schema (never set
// by the user in config), so req.Config.Get sees it as null; unlike the
// resource, this model must represent that with a nullable types.Object
// rather than a bare struct.
type PoolDataSourceModel struct {
	ID        types.Int64  `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Topology  types.Object `tfsdk:"topology"`
	AutoTrim  types.Bool   `tfsdk:"autotrim"`
	GUID      types.String `tfsdk:"guid"`
	Status    types.String `tfsdk:"status"`
	Healthy   types.Bool   `tfsdk:"healthy"`
	Path      types.String `tfsdk:"path"`
	Size      types.Int64  `tfsdk:"size"`
	Free      types.Int64  `tfsdk:"free"`
	Allocated types.Int64  `tfsdk:"allocated"`
}

// poolAPI is the wire-format response from TrueNAS pool methods.
type poolAPI struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	GUID      string `json:"guid"`
	Status    string `json:"status"`
	Healthy   bool   `json:"healthy"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Free      int64  `json:"free"`
	Allocated int64  `json:"allocated"`
	AutoTrim  struct {
		Parsed autotrimParsed `json:"parsed"`
	} `json:"autotrim"`
	Topology struct {
		Data  []poolVdev `json:"data"`
		Log   []poolVdev `json:"log"`
		Cache []poolVdev `json:"cache"`
		Spare []poolVdev `json:"spare"`
	} `json:"topology"`
}

type poolVdev struct {
	Type     string     `json:"type"`
	Children []poolDisk `json:"children"`
	// Disk is set on a single-disk vdev (a lone cache/log/spare device, or a
	// single-disk data vdev), where the device is reported at the vdev's top
	// level with an empty children array and type "DISK".
	Disk string `json:"disk"`
}

// poolDisk is a member of a vdev. It is usually a leaf DISK carrying its own
// "disk" name, but it can itself be a nested vdev — a "SPARE" vdev once a hot
// spare has stepped in for a faulted member, or a "REPLACING" vdev mid-resilver
// — in which case it carries no "disk" of its own and its Children hold the
// real devices (Children[0] is the original member, Children[1] the
// spare/replacement). Hence Type/Children are decoded too. See childDisk.
type poolDisk struct {
	Disk     string     `json:"disk"`
	Type     string     `json:"type"`
	Children []poolDisk `json:"children"`
}

// autotrimParsed decodes the "parsed" field of the autotrim ZFS property
// object returned by pool.query / pool.get_instance. TrueNAS sends this as
// the string "ON"/"OFF" (case varies), not a JSON bool - a naive `bool`
// field fails with "json: cannot unmarshal string into Go struct field
// .autotrim.parsed of type bool". Accept a native JSON bool too, in case
// the API shape ever changes.
type autotrimParsed bool

func (a *autotrimParsed) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case bool:
		*a = autotrimParsed(v)
	case string:
		switch strings.ToLower(v) {
		case "on", "true":
			*a = true
		case "off", "false":
			*a = false
		default:
			return fmt.Errorf("autotrim.parsed: unrecognized value %q", v)
		}
	default:
		return fmt.Errorf("autotrim.parsed: unsupported JSON type %T", raw)
	}
	return nil
}

// buildTopology converts the wire-format topology in a poolAPI response into
// a TopologyModel. Shared by responseToModel (resource) and
// responseToDataSourceModel (datasource) so both stay in sync.
//
// Disk names are stored as pool.query reports them — the current kernel device
// name (sdX). State deliberately reflects the LIVE device rather than a
// canonical serial: Terraform forbids a provider from rewriting a Required
// attribute (topology disks) to a third value, so a create-by-sdX could not
// store a serial anyway. Renumber-proofing instead lives in the plan
// (reconcilePlanTopology): when a disk is named by its stable serial (or any
// other form) in config and by a different sdX in state but they are the SAME
// physical disk, the plan is suppressed rather than forcing a replacement.
// vdevType still normalizes a single-disk vdev's "DISK" to "STRIPE".
//
// res is accepted for signature symmetry with the rest of the read path (and
// future use); the raw names it would resolve are what state stores here.
func buildTopology(ctx context.Context, api *poolAPI, res *diskResolver) (TopologyModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var topo TopologyModel
	_ = res

	// Convert data vdevs
	data := make([]VdevModel, len(api.Topology.Data))
	for i, v := range api.Topology.Data {
		diskList, d := types.ListValueFrom(ctx, types.StringType, vdevDisks(v))
		diags.Append(d...)
		data[i] = VdevModel{Type: types.StringValue(vdevType(v)), Disks: diskList}
	}
	dataList, d := types.ListValueFrom(ctx, vdevObjectType, data)
	diags.Append(d...)
	topo.Data = dataList

	// Convert log vdevs
	logVdevs := make([]VdevModel, len(api.Topology.Log))
	for i, v := range api.Topology.Log {
		diskList, d := types.ListValueFrom(ctx, types.StringType, vdevDisks(v))
		diags.Append(d...)
		logVdevs[i] = VdevModel{Type: types.StringValue(vdevType(v)), Disks: diskList}
	}
	logList, dl := types.ListValueFrom(ctx, vdevObjectType, logVdevs)
	diags.Append(dl...)
	topo.Log = logList

	// Cache: each cache vdev is a single-disk DISK vdev; flatten to disk names
	cacheDisks := make([]string, len(api.Topology.Cache))
	for i, v := range api.Topology.Cache {
		disks := vdevDisks(v)
		if len(disks) > 0 {
			cacheDisks[i] = disks[0]
		}
	}
	cacheList, d := types.ListValueFrom(ctx, types.StringType, cacheDisks)
	diags.Append(d...)
	topo.Cache = cacheList

	// Spare: each spare vdev is a single-disk vdev; flatten to disk names
	spareDisks := make([]string, len(api.Topology.Spare))
	for i, v := range api.Topology.Spare {
		disks := vdevDisks(v)
		if len(disks) > 0 {
			spareDisks[i] = disks[0]
		}
	}
	spareList, d := types.ListValueFrom(ctx, types.StringType, spareDisks)
	diags.Append(d...)
	topo.Spare = spareList

	return topo, diags
}

// responseToModel maps a TrueNAS API pool response into the Terraform state
// model. res canonicalizes topology disk names to their stable serials.
func responseToModel(ctx context.Context, api *poolAPI, m *PoolModel, res *diskResolver) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.GUID = types.StringValue(api.GUID)
	m.Status = types.StringValue(api.Status)
	m.Healthy = types.BoolValue(api.Healthy)
	m.Path = types.StringValue(api.Path)
	m.Size = types.Int64Value(api.Size)
	m.Free = types.Int64Value(api.Free)
	m.Allocated = types.Int64Value(api.Allocated)
	m.AutoTrim = types.BoolValue(bool(api.AutoTrim.Parsed))

	topo, d := buildTopology(ctx, api, res)
	diags.Append(d...)
	m.Topology = topo

	return diags
}

// responseToDataSourceModel maps a TrueNAS API pool response into the
// truenas_pool datasource's Terraform state model. It reuses buildTopology
// (the same conversion the resource uses) and then wraps the result as a
// types.Object, since the datasource model must be able to represent
// topology as null.
func responseToDataSourceModel(ctx context.Context, api *poolAPI, m *PoolDataSourceModel, res *diskResolver) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.GUID = types.StringValue(api.GUID)
	m.Status = types.StringValue(api.Status)
	m.Healthy = types.BoolValue(api.Healthy)
	m.Path = types.StringValue(api.Path)
	m.Size = types.Int64Value(api.Size)
	m.Free = types.Int64Value(api.Free)
	m.Allocated = types.Int64Value(api.Allocated)
	m.AutoTrim = types.BoolValue(bool(api.AutoTrim.Parsed))

	topo, d := buildTopology(ctx, api, res)
	diags.Append(d...)
	topoObj, d2 := types.ObjectValueFrom(ctx, topologyAttrTypes, topo)
	diags.Append(d2...)
	m.Topology = topoObj

	return diags
}

// applyPlannedTopology chooses, per vdev list, the value to store in state
// after a create/update. Terraform requires applied state to equal the plan
// for a configured (known) attribute, so a list the user configured keeps its
// PLANNED (config-form) value; an Optional+Computed list the user omitted is
// unknown in the plan and instead takes the value read back from the API,
// which must be fully known. "data" is Required, so it is always the planned
// value.
func applyPlannedTopology(planned, fromAPI TopologyModel) TopologyModel {
	pick := func(p, a types.List) types.List {
		if p.IsUnknown() {
			return a
		}
		return p
	}
	return TopologyModel{
		Data:  pick(planned.Data, fromAPI.Data),
		Log:   pick(planned.Log, fromAPI.Log),
		Cache: pick(planned.Cache, fromAPI.Cache),
		Spare: pick(planned.Spare, fromAPI.Spare),
	}
}

// autotrimStr converts a boolean autotrim value to the TrueNAS string format.
func autotrimStr(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

// vdevDisks extracts the flat list of disk names from a vdev. Multi-disk vdevs
// (MIRROR/RAIDZ) list their members under children; a single-disk vdev
// (lone cache/log/spare, or a single-disk data vdev) instead reports the device
// at the vdev's top level with empty children, so fall back to that.
func vdevDisks(v poolVdev) []string {
	if len(v.Children) == 0 {
		if v.Disk != "" {
			return []string{v.Disk}
		}
		return []string{}
	}
	disks := make([]string, len(v.Children))
	for i, c := range v.Children {
		disks[i] = childDisk(c)
	}
	return disks
}

// childDisk returns the representative device name of a vdev child, so a
// degraded pool reads back with its configured membership rather than a
// transient repair structure. A leaf DISK carries its own "disk". A nested
// vdev child — a "SPARE" that has activated for a faulted member, or a
// "REPLACING" vdev during a resilver — carries no "disk"; its first child is
// the ORIGINAL configured member (the second is the spare/replacement), so the
// slot is represented by that original. This is what keeps a hot-spare
// activation from looking like a topology change (which, since topology is
// RequiresReplace, would otherwise plan a destroy/recreate of a degraded pool
// — see model.go's issue #9 handling and TestVdevDisks_spareActive, plus the
// live import-of-a-degraded-pool verification recorded in the changelog).
func childDisk(c poolDisk) string {
	if c.Disk != "" {
		return c.Disk
	}
	if len(c.Children) > 0 {
		return childDisk(c.Children[0])
	}
	return ""
}

// vdevType normalizes a vdev's reported type for round-tripping with the create
// API: TrueNAS reports a single-disk stripe vdev as type "DISK", but
// pool.create accepts "STRIPE" for that shape. Multi-disk types (MIRROR,
// RAIDZ*) are returned unchanged.
func vdevType(v poolVdev) string {
	return normalizeVdevType(v.Type)
}

// normalizeVdevType maps a vdev type string to its canonical create-API form.
// "DISK" (what pool.query returns, and the TrueNAS UI shows, for a single-disk
// stripe) is an accepted alias for "STRIPE" (what pool.create wants). Applying
// this to BOTH the readback (vdevType) AND the user's configured value means a
// config written as `type = "DISK"` and the state read back as "STRIPE" agree,
// instead of perpetually diffing into a pool replacement (issue #9). Any other
// value (STRIPE, MIRROR, RAIDZ1/2/3, or something unrecognized) passes through
// unchanged, case-folded to upper so "mirror" and "MIRROR" also agree.
func normalizeVdevType(t string) string {
	up := strings.ToUpper(strings.TrimSpace(t))
	if up == "DISK" {
		return "STRIPE"
	}
	return up
}

// reconcilePlanTopology rewrites a planned topology so a disk named in a
// different form than state — but pointing at the SAME physical disk — does
// not read as a change (which, since topology is RequiresReplace, would
// destroy and recreate the pool). It runs only on update (state non-nil); see
// PoolResource.ModifyPlan for why Create is left alone.
//
// Terraform requires a Required attribute's planned value to equal either the
// config value or the prior-state value (the "normalization" exception) —
// never a third value — so this only ever chooses between those two:
//
//   - a disk whose planned (config) name and its positional state name resolve
//     to the SAME physical disk (res.sameDisk: an sdX renumber, or a serial
//     written where an sdX is stored, or any accepted-form difference) is set
//     to the STATE value, erasing the spurious diff. A disk that resolves to a
//     DIFFERENT physical disk is left as the config value, so a real swap still
//     forces replacement.
//   - vdev "type" is set to the STATE value when the config value normalizes to
//     it (e.g. config "DISK" vs state "STRIPE"); otherwise it is left as the
//     config value (a real type change).
//
// Positional comparison is used: topology vdev and disk order is stable across
// a read, and a length change is itself a real topology change that should
// force replacement, so mismatched lengths fall through to the config value.
func reconcilePlanTopology(ctx context.Context, plan TopologyModel, state *TopologyModel, res *diskResolver) (TopologyModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := plan

	var sData, sLog *types.List
	var sCache, sSpare *types.List
	if state != nil {
		sData, sLog, sCache, sSpare = &state.Data, &state.Log, &state.Cache, &state.Spare
	}

	d, dd := reconcileVdevList(ctx, plan.Data, sData, res)
	diags.Append(dd...)
	out.Data = d

	l, dl := reconcileVdevList(ctx, plan.Log, sLog, res)
	diags.Append(dl...)
	out.Log = l

	c, dc := reconcileDiskList(ctx, plan.Cache, sCache, res)
	diags.Append(dc...)
	out.Cache = c

	s, ds := reconcileDiskList(ctx, plan.Spare, sSpare, res)
	diags.Append(ds...)
	out.Spare = s

	return out, diags
}

// reconcileVdevList applies reconcilePlanTopology's rules to one data/log vdev
// list. A null/unknown plan list, or a plan the framework hasn't fully
// resolved, is returned unchanged.
func reconcileVdevList(ctx context.Context, planList types.List, stateList *types.List, res *diskResolver) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if planList.IsNull() || planList.IsUnknown() {
		return planList, diags
	}
	var planVdevs []VdevModel
	diags.Append(planList.ElementsAs(ctx, &planVdevs, false)...)

	var stateVdevs []VdevModel
	if stateList != nil && !stateList.IsNull() && !stateList.IsUnknown() {
		diags.Append(stateList.ElementsAs(ctx, &stateVdevs, false)...)
	}

	for i := range planVdevs {
		var stateType string
		var stateDisks []string
		if i < len(stateVdevs) {
			stateType = stateVdevs[i].Type.ValueString()
			diags.Append(stateVdevs[i].Disks.ElementsAs(ctx, &stateDisks, false)...)
		}

		// Type: adopt the state spelling when the config normalizes to it
		// (config "DISK" vs state "STRIPE"); otherwise keep the config value.
		if normalizeVdevType(planVdevs[i].Type.ValueString()) == stateType {
			planVdevs[i].Type = types.StringValue(stateType)
		}

		var planDisks []string
		diags.Append(planVdevs[i].Disks.ElementsAs(ctx, &planDisks, false)...)
		for j := range planDisks {
			// Same physical disk as state, just a different name form: keep the
			// state value (a normalization Terraform permits). Otherwise leave
			// the config value (a real disk change → replacement).
			if j < len(stateDisks) && res.sameDisk(planDisks[j], stateDisks[j]) {
				planDisks[j] = stateDisks[j]
			}
		}
		dl, dd := types.ListValueFrom(ctx, types.StringType, planDisks)
		diags.Append(dd...)
		planVdevs[i].Disks = dl
	}

	out, d := types.ListValueFrom(ctx, vdevObjectType, planVdevs)
	diags.Append(d...)
	return out, diags
}

// reconcileDiskList applies reconcilePlanTopology's rules to one flat
// disk-name list (cache/spare).
func reconcileDiskList(ctx context.Context, planList types.List, stateList *types.List, res *diskResolver) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	if planList.IsNull() || planList.IsUnknown() {
		return planList, diags
	}
	var planDisks []string
	diags.Append(planList.ElementsAs(ctx, &planDisks, false)...)

	var stateDisks []string
	if stateList != nil && !stateList.IsNull() && !stateList.IsUnknown() {
		diags.Append(stateList.ElementsAs(ctx, &stateDisks, false)...)
	}

	for i := range planDisks {
		// Keep the state value only when it is the same physical disk (a
		// permitted normalization); otherwise leave the config value.
		if i < len(stateDisks) && res.sameDisk(planDisks[i], stateDisks[i]) {
			planDisks[i] = stateDisks[i]
		}
	}
	out, d := types.ListValueFrom(ctx, types.StringType, planDisks)
	diags.Append(d...)
	return out, diags
}

// apiPayload builds the JSON payload for pool.create. res translates each
// config disk (given in any accepted form — serial, kernel name, TrueNAS
// identifier, or /dev/disk/by-id path) to the current kernel device name that
// pool.create consumes; an unresolvable name is passed through unchanged.
func (m *PoolModel) apiPayload(ctx context.Context, res *diskResolver) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	devices := func(disks []string) []string {
		out := make([]string, len(disks))
		for i, d := range disks {
			out[i] = res.deviceFor(d)
		}
		return out
	}

	// vdevPayload reads a data/log vdev list (which may be null or unknown when
	// the config omits it) into the wire shape [{type, disks}]. type is
	// normalized (DISK->STRIPE) to the form pool.create accepts.
	vdevPayload := func(l types.List) []map[string]any {
		out := []map[string]any{}
		if l.IsNull() || l.IsUnknown() {
			return out
		}
		var items []VdevModel
		diags.Append(l.ElementsAs(ctx, &items, false)...)
		for _, v := range items {
			var disks []string
			diags.Append(v.Disks.ElementsAs(ctx, &disks, false)...)
			out = append(out, map[string]any{"type": normalizeVdevType(v.Type.ValueString()), "disks": devices(disks)})
		}
		return out
	}
	dataVdevs := vdevPayload(m.Topology.Data)
	logVdevs := vdevPayload(m.Topology.Log)

	// diskList reads a flat disk-name list, tolerating null/unknown.
	diskList := func(l types.List) []string {
		out := []string{}
		if l.IsNull() || l.IsUnknown() {
			return out
		}
		diags.Append(l.ElementsAs(ctx, &out, false)...)
		return out
	}
	cacheDisks := devices(diskList(m.Topology.Cache))
	// Each L2ARC cache vdev is type "STRIPE" (a const in the pool.create
	// schema); "DISK" is rejected.
	cacheVdevs := make([]map[string]any, len(cacheDisks))
	for i, d := range cacheDisks {
		cacheVdevs[i] = map[string]any{"type": "STRIPE", "disks": []string{d}}
	}

	spareDisks := devices(diskList(m.Topology.Spare))

	// pool.create's topology key for spares is "spares" (plural) and takes a
	// flat array of disk names; note pool.query returns them under "spare"
	// (singular) — see responseToModel. autotrim is NOT a pool.create input
	// (it is rejected as "Extra inputs are not permitted"); the resource sets
	// it via a follow-up pool.update in Create instead.
	p := map[string]any{
		"name": m.Name.ValueString(),
		"topology": map[string]any{
			"data":   dataVdevs,
			"log":    logVdevs,
			"cache":  cacheVdevs,
			"spares": spareDisks,
		},
	}
	return p, diags
}
