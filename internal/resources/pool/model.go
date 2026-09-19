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
}

type poolDisk struct {
	Disk string `json:"disk"`
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
func buildTopology(ctx context.Context, api *poolAPI) (TopologyModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var topo TopologyModel

	// Convert data vdevs
	data := make([]VdevModel, len(api.Topology.Data))
	for i, v := range api.Topology.Data {
		disks := vdevDisks(v)
		diskList, d := types.ListValueFrom(ctx, types.StringType, disks)
		diags.Append(d...)
		data[i] = VdevModel{Type: types.StringValue(v.Type), Disks: diskList}
	}
	dataList, d := types.ListValueFrom(ctx, vdevObjectType, data)
	diags.Append(d...)
	topo.Data = dataList

	// Convert log vdevs
	logVdevs := make([]VdevModel, len(api.Topology.Log))
	for i, v := range api.Topology.Log {
		disks := vdevDisks(v)
		diskList, d := types.ListValueFrom(ctx, types.StringType, disks)
		diags.Append(d...)
		logVdevs[i] = VdevModel{Type: types.StringValue(v.Type), Disks: diskList}
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

// responseToModel maps a TrueNAS API pool response into the Terraform state model.
func responseToModel(ctx context.Context, api *poolAPI, m *PoolModel) diag.Diagnostics {
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

	topo, d := buildTopology(ctx, api)
	diags.Append(d...)
	m.Topology = topo

	return diags
}

// responseToDataSourceModel maps a TrueNAS API pool response into the
// truenas_pool datasource's Terraform state model. It reuses buildTopology
// (the same conversion the resource uses) and then wraps the result as a
// types.Object, since the datasource model must be able to represent
// topology as null.
func responseToDataSourceModel(ctx context.Context, api *poolAPI, m *PoolDataSourceModel) diag.Diagnostics {
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

	topo, d := buildTopology(ctx, api)
	diags.Append(d...)
	topoObj, d2 := types.ObjectValueFrom(ctx, topologyAttrTypes, topo)
	diags.Append(d2...)
	m.Topology = topoObj

	return diags
}

// autotrimStr converts a boolean autotrim value to the TrueNAS string format.
func autotrimStr(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

// vdevDisks extracts the flat list of disk names from a vdev's children.
func vdevDisks(v poolVdev) []string {
	disks := make([]string, len(v.Children))
	for i, c := range v.Children {
		disks[i] = c.Disk
	}
	return disks
}

// apiPayload builds the JSON payload for pool.create.
func (m *PoolModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	// vdevPayload reads a data/log vdev list (which may be null or unknown when
	// the config omits it) into the wire shape [{type, disks}].
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
			out = append(out, map[string]any{"type": v.Type.ValueString(), "disks": disks})
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
	cacheDisks := diskList(m.Topology.Cache)
	cacheVdevs := make([]map[string]any, len(cacheDisks))
	for i, d := range cacheDisks {
		cacheVdevs[i] = map[string]any{"type": "DISK", "disks": []string{d}}
	}

	spareDisks := diskList(m.Topology.Spare)

	p := map[string]any{
		"name": m.Name.ValueString(),
		"topology": map[string]any{
			"data":  dataVdevs,
			"log":   logVdevs,
			"cache": cacheVdevs,
			"spare": spareDisks,
		},
		"autotrim": autotrimStr(m.AutoTrim.ValueBool()),
	}
	return p, diags
}
