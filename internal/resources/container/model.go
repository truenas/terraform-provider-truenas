// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// containerVersionFloorMajor/Minor is the minimum TrueNAS release
// that exposes the container.* namespace at all. Probed live: TrueNAS 25.10
// returns 0 container.* methods from core.get_methods (namespace genuinely
// absent, not just empty); TrueNAS 26.0 supports the full namespace
// (container.create/update/start/stop/delete/get_instance/query, all
// confirmed live this session — see resource.go's checkVersion for where
// this gates every entry point before any container.* call reaches the
// wire).
const (
	containerVersionFloorMajor = 26
	containerVersionFloorMinor = 0
)

// versionGateDiagnostics reports whether the given TrueNAS release string
// (as returned by system.version_short via client.ServerVersion) is at or
// above the TrueNAS 26.0 floor the container namespace requires, returning a
// single clean error diagnostic when it is not. Pure function of an
// already-probed version string (no live client access), matching the
// lxc_config precedent so it stays independently unit-testable.
func versionGateDiagnostics(version string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !client.VersionAtLeastString(version, containerVersionFloorMajor, containerVersionFloorMinor) {
		diags.AddError(
			"TrueNAS version too old",
			"truenas_container requires TrueNAS 26.0 or later",
		)
	}
	return diags
}

// imageAttrTypes describes the attribute types of the nested "image" object.
var imageAttrTypes = map[string]attr.Type{
	"name":    types.StringType,
	"version": types.StringType,
}

// ImageModel maps to the nested "image" attribute: the LXC image registry
// name (e.g. "alpine:3.22:amd64:default") and a specific version string
// from container.image.query_registry (e.g. "20260722_17:16"). Probed
// live: container.create rejects a bare image string with a clean EINVAL —
// the API genuinely requires this {name, version} object shape.
type ImageModel struct {
	Name    types.String `tfsdk:"name"`
	Version types.String `tfsdk:"version"`
}

// ContainerModel is the Terraform state/plan model for truenas_container.
//
// Pool and Image are write-only from the API's perspective: probed live,
// neither container.create's job result nor container.get_instance/query
// ever echoes back "pool" or "image" (confirmed against the exact same
// container across create, two updates, and a final get_instance — the
// field is simply absent from every response). Pool is nonetheless
// self-healing on every Read: the container's root dataset path
// ("tank/.truenas_containers/containers/<name>") always starts with the
// pool name, so poolFromDataset recovers it from "dataset" rather than
// trusting stale plan/state. Image has no such recovery path — it is
// genuinely unrecoverable after creation, which is why ImportState leaves
// it null (see resource.go) and the acceptance test's ImportStateVerify
// must ignore it.
//
// Idmap is carried opaquely (as JSON text) rather than modeled field by
// field: its wire shape is a discriminated union (DefaultIdmapConfiguration
// vs IsolatedIdmapConfiguration, probed live via core.get_methods) that
// would add real schema complexity for a field this resource does not need
// to validate or diff structurally. It is also container.update-immutable
// (probed: container.update's accepts list omits "idmap" entirely, unlike
// "description"/"capabilities_policy"/etc which it does accept) hence
// RequiresReplace in schema.go. CapabilitiesState is NOT treated this way
// because its shape is flat (map[string]bool, probed
// additionalProperties:{type:boolean}) and container.update DOES accept
// it, so a real types.Map models it precisely with no opaque-JSON caveat
// needed.
type ContainerModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	UUID               types.String `tfsdk:"uuid"`
	Name               types.String `tfsdk:"name"`
	Pool               types.String `tfsdk:"pool"`
	Image              types.Object `tfsdk:"image"`
	Description        types.String `tfsdk:"description"`
	Autostart          types.Bool   `tfsdk:"autostart"`
	Cpuset             types.String `tfsdk:"cpuset"`
	Time               types.String `tfsdk:"time"`
	ShutdownTimeout    types.Int64  `tfsdk:"shutdown_timeout"`
	Init               types.String `tfsdk:"init"`
	InitDir            types.String `tfsdk:"initdir"`
	InitEnv            types.Map    `tfsdk:"initenv"`
	InitUser           types.String `tfsdk:"inituser"`
	InitGroup          types.String `tfsdk:"initgroup"`
	Idmap              types.String `tfsdk:"idmap"` // opaque JSON, see doc comment above
	CapabilitiesPolicy types.String `tfsdk:"capabilities_policy"`
	CapabilitiesState  types.Map    `tfsdk:"capabilities_state"` // map[string]bool
	Running            types.Bool   `tfsdk:"running"`
	Dataset            types.String `tfsdk:"dataset"`
	DefaultNetwork     types.String `tfsdk:"default_network"`
	Status             types.String `tfsdk:"status"`
}

// ContainerDataSourceModel is the read-only model for the truenas_container
// datasource, looked up by name.
type ContainerDataSourceModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	UUID               types.String `tfsdk:"uuid"`
	Name               types.String `tfsdk:"name"`
	Pool               types.String `tfsdk:"pool"`
	Description        types.String `tfsdk:"description"`
	Autostart          types.Bool   `tfsdk:"autostart"`
	Cpuset             types.String `tfsdk:"cpuset"`
	Time               types.String `tfsdk:"time"`
	ShutdownTimeout    types.Int64  `tfsdk:"shutdown_timeout"`
	Init               types.String `tfsdk:"init"`
	InitDir            types.String `tfsdk:"initdir"`
	InitEnv            types.Map    `tfsdk:"initenv"`
	InitUser           types.String `tfsdk:"inituser"`
	InitGroup          types.String `tfsdk:"initgroup"`
	Idmap              types.String `tfsdk:"idmap"`
	CapabilitiesPolicy types.String `tfsdk:"capabilities_policy"`
	CapabilitiesState  types.Map    `tfsdk:"capabilities_state"`
	Running            types.Bool   `tfsdk:"running"`
	Dataset            types.String `tfsdk:"dataset"`
	DefaultNetwork     types.String `tfsdk:"default_network"`
	Status             types.String `tfsdk:"status"`
}

// containerStatusAPI is the JSON wire format for the nested "status" object
// returned by container.get_instance/query. state is "RUNNING"/"STOPPED";
// pid/domain_state are informational only (probed: both null on a stopped
// container) and are not surfaced as separate schema attributes (out of
// scope per the design spec's minimal Computed field list).
type containerStatusAPI struct {
	State       string  `json:"state"`
	PID         *int64  `json:"pid"`
	DomainState *string `json:"domain_state"`
}

// containerAPI mirrors the JSON object returned by container.create's job
// result, container.update, container.get_instance, and container.query.
// Probed live against TrueNAS 26.0 (the only release with this namespace —
// see versionGateDiagnostics): notably, "pool" and "image" are NEVER
// present in this shape (see ContainerModel's doc comment) even though
// both are required/accepted by container.create.
type containerAPI struct {
	ID                 int64              `json:"id"`
	UUID               *string            `json:"uuid"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	Cpuset             *string            `json:"cpuset"`
	Autostart          bool               `json:"autostart"`
	Time               string             `json:"time"`
	ShutdownTimeout    int64              `json:"shutdown_timeout"`
	Dataset            string             `json:"dataset"`
	Init               string             `json:"init"`
	InitDir            *string            `json:"initdir"`
	InitEnv            map[string]string  `json:"initenv"`
	InitUser           *string            `json:"inituser"`
	InitGroup          *string            `json:"initgroup"`
	Idmap              json.RawMessage    `json:"idmap"`
	CapabilitiesPolicy string             `json:"capabilities_policy"`
	CapabilitiesState  map[string]bool    `json:"capabilities_state"`
	DefaultNetwork     *string            `json:"default_network"`
	Status             containerStatusAPI `json:"status"`
}

// poolFromDataset recovers the ZFS pool name from a container's root
// dataset path (e.g. "tank/.truenas_containers/containers/my-ctr" ->
// "tank"), since container.get_instance/query never echoes back "pool"
// directly (see ContainerModel's doc comment).
func poolFromDataset(dataset string) string {
	if i := strings.IndexByte(dataset, '/'); i >= 0 {
		return dataset[:i]
	}
	return dataset
}

// idmapResponseValue decides the state value for the opaque "idmap" field
// after container.create: since container.update never accepts "idmap"
// (probed), the field is immutable after creation, so its stored text must
// exactly equal the planned value whenever the plan already had a known
// value — any reformatting by round-tripping through the API's response
// would trip Terraform's "provider produced inconsistent result after
// apply" check. Only when the plan value is unknown (the user left idmap
// unset, relying on the server default) does this fall back to the API's
// own response text, since Terraform requires a concrete (if arbitrary)
// value to resolve a Computed unknown.
func idmapResponseValue(planIdmap types.String, apiIdmap json.RawMessage) types.String {
	if !planIdmap.IsNull() && !planIdmap.IsUnknown() {
		return planIdmap
	}
	return idmapFromAPI(apiIdmap)
}

// idmapFromAPI converts the raw "idmap" JSON from an API response into a
// types.String, mapping a JSON null (or an empty/absent value) to a null
// types.String rather than the literal text "null".
func idmapFromAPI(apiIdmap json.RawMessage) types.String {
	if len(apiIdmap) == 0 || string(apiIdmap) == "null" {
		return types.StringNull()
	}
	return types.StringValue(string(apiIdmap))
}

// responseToModel maps a containerAPI response onto a ContainerModel.
// Deliberately does NOT touch Image or Idmap (see idmapResponseValue for
// idmap's own Create-time handling; Image is always carried forward as-is
// by the caller, see resource.go): both are immutable after creation and
// the API never round-trips "image" at all, so overwriting either here
// would either lose information (image) or risk plan-inconsistency
// (idmap). Pool IS overwritten, but derived from Dataset rather than the
// (absent) API "pool" field.
func responseToModel(ctx context.Context, api *containerAPI, m *ContainerModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.UUID = types.StringPointerValue(api.UUID)
	m.Name = types.StringValue(api.Name)
	m.Pool = types.StringValue(poolFromDataset(api.Dataset))
	m.Description = types.StringValue(api.Description)
	m.Autostart = types.BoolValue(api.Autostart)
	m.Cpuset = types.StringPointerValue(api.Cpuset)
	m.Time = types.StringValue(api.Time)
	m.ShutdownTimeout = types.Int64Value(api.ShutdownTimeout)
	m.Init = types.StringValue(api.Init)
	m.InitDir = types.StringPointerValue(api.InitDir)

	initEnv, d := types.MapValueFrom(ctx, types.StringType, nonNilStringMap(api.InitEnv))
	diags.Append(d...)
	m.InitEnv = initEnv

	m.InitUser = types.StringPointerValue(api.InitUser)
	m.InitGroup = types.StringPointerValue(api.InitGroup)
	m.CapabilitiesPolicy = types.StringValue(api.CapabilitiesPolicy)

	capState, d := types.MapValueFrom(ctx, types.BoolType, nonNilBoolMap(api.CapabilitiesState))
	diags.Append(d...)
	m.CapabilitiesState = capState

	m.Dataset = types.StringValue(api.Dataset)
	m.DefaultNetwork = types.StringPointerValue(api.DefaultNetwork)
	m.Status = types.StringValue(api.Status.State)
	m.Running = types.BoolValue(api.Status.State == "RUNNING")

	return diags
}

// responseToDataSourceModel maps a containerAPI response onto a
// ContainerDataSourceModel, plus the caller-supplied opaque idmap text
// (already read back live, so no plan-consistency concern for a
// datasource).
func responseToDataSourceModel(ctx context.Context, api *containerAPI, m *ContainerDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.UUID = types.StringPointerValue(api.UUID)
	m.Name = types.StringValue(api.Name)
	m.Pool = types.StringValue(poolFromDataset(api.Dataset))
	m.Description = types.StringValue(api.Description)
	m.Autostart = types.BoolValue(api.Autostart)
	m.Cpuset = types.StringPointerValue(api.Cpuset)
	m.Time = types.StringValue(api.Time)
	m.ShutdownTimeout = types.Int64Value(api.ShutdownTimeout)
	m.Init = types.StringValue(api.Init)
	m.InitDir = types.StringPointerValue(api.InitDir)

	initEnv, d := types.MapValueFrom(ctx, types.StringType, nonNilStringMap(api.InitEnv))
	diags.Append(d...)
	m.InitEnv = initEnv

	m.InitUser = types.StringPointerValue(api.InitUser)
	m.InitGroup = types.StringPointerValue(api.InitGroup)
	m.Idmap = idmapFromAPI(api.Idmap)
	m.CapabilitiesPolicy = types.StringValue(api.CapabilitiesPolicy)

	capState, d := types.MapValueFrom(ctx, types.BoolType, nonNilBoolMap(api.CapabilitiesState))
	diags.Append(d...)
	m.CapabilitiesState = capState

	m.Dataset = types.StringValue(api.Dataset)
	m.DefaultNetwork = types.StringPointerValue(api.DefaultNetwork)
	m.Status = types.StringValue(api.Status.State)
	m.Running = types.BoolValue(api.Status.State == "RUNNING")

	return diags
}

// nonNilStringMap returns m unchanged, or an empty (non-nil) map, so
// types.MapValueFrom always produces a known empty map rather than a null
// one when the API returns an empty/absent object (probed: initenv
// defaults to {}, never omitted).
func nonNilStringMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// nonNilBoolMap is nonNilStringMap's counterpart for capabilities_state
// (probed: also defaults to {}, never omitted).
func nonNilBoolMap(m map[string]bool) map[string]bool {
	if m == nil {
		return map[string]bool{}
	}
	return m
}

// createPayload builds the map[string]any payload for container.create.
// "name", "pool", and "image" are always included (Required in schema).
// Every other field is Optional+Computed: cpuset/initdir/inituser/initgroup
// use the three-way nullable convention (unknown omits, explicit null sends
// JSON nil, a value sends the string) since container.create's own schema
// treats them as anyOf{string,null} with a null default; every other
// Optional+Computed field uses a plain guard (included only when known and
// non-null) so an unset optional lets the TrueNAS-side default apply
// instead of an explicit zero value.
func (m *ContainerModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var img ImageModel
	diags.Append(m.Image.As(ctx, &img, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	p := map[string]any{
		"name": m.Name.ValueString(),
		"pool": m.Pool.ValueString(),
		"image": map[string]any{
			"name":    img.Name.ValueString(),
			"version": img.Version.ValueString(),
		},
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.Autostart.IsNull() && !m.Autostart.IsUnknown() {
		p["autostart"] = m.Autostart.ValueBool()
	}
	if !m.Time.IsNull() && !m.Time.IsUnknown() {
		p["time"] = m.Time.ValueString()
	}
	if !m.ShutdownTimeout.IsNull() && !m.ShutdownTimeout.IsUnknown() {
		p["shutdown_timeout"] = m.ShutdownTimeout.ValueInt64()
	}
	if !m.Init.IsNull() && !m.Init.IsUnknown() {
		p["init"] = m.Init.ValueString()
	}
	if !m.CapabilitiesPolicy.IsNull() && !m.CapabilitiesPolicy.IsUnknown() {
		p["capabilities_policy"] = m.CapabilitiesPolicy.ValueString()
	}

	threeWayString(p, "cpuset", m.Cpuset)
	threeWayString(p, "initdir", m.InitDir)
	threeWayString(p, "inituser", m.InitUser)
	threeWayString(p, "initgroup", m.InitGroup)

	if !m.InitEnv.IsNull() && !m.InitEnv.IsUnknown() {
		var env map[string]string
		diags.Append(m.InitEnv.ElementsAs(ctx, &env, false)...)
		p["initenv"] = env
	}
	if !m.CapabilitiesState.IsNull() && !m.CapabilitiesState.IsUnknown() {
		var capState map[string]bool
		diags.Append(m.CapabilitiesState.ElementsAs(ctx, &capState, false)...)
		p["capabilities_state"] = capState
	}
	if !m.Idmap.IsNull() && !m.Idmap.IsUnknown() {
		var idmap any
		if err := json.Unmarshal([]byte(m.Idmap.ValueString()), &idmap); err != nil {
			diags.AddError("Invalid idmap JSON", err.Error())
			return nil, diags
		}
		p["idmap"] = idmap
	}

	return p, diags
}

// updatePayload builds the map[string]any payload for container.update.
// Only fields container.update actually accepts are included (probed live
// via core.get_methods): "pool", "image", and "idmap" are permanently
// excluded (RequiresReplace in schema.go — container.update rejects "pool"
// with a clean EINVAL, confirmed live; "image"/"idmap" are simply absent
// from its accepted field list). "name" IS included: probed live,
// container.update accepts and applies it (renaming a container works),
// so unlike "pool"/"image" it is NOT ForceNew in schema.go.
func (m *ContainerModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"name": m.Name.ValueString(),
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.Autostart.IsNull() && !m.Autostart.IsUnknown() {
		p["autostart"] = m.Autostart.ValueBool()
	}
	if !m.Time.IsNull() && !m.Time.IsUnknown() {
		p["time"] = m.Time.ValueString()
	}
	if !m.ShutdownTimeout.IsNull() && !m.ShutdownTimeout.IsUnknown() {
		p["shutdown_timeout"] = m.ShutdownTimeout.ValueInt64()
	}
	if !m.Init.IsNull() && !m.Init.IsUnknown() {
		p["init"] = m.Init.ValueString()
	}
	if !m.CapabilitiesPolicy.IsNull() && !m.CapabilitiesPolicy.IsUnknown() {
		p["capabilities_policy"] = m.CapabilitiesPolicy.ValueString()
	}

	threeWayString(p, "cpuset", m.Cpuset)
	threeWayString(p, "initdir", m.InitDir)
	threeWayString(p, "inituser", m.InitUser)
	threeWayString(p, "initgroup", m.InitGroup)

	if !m.InitEnv.IsNull() && !m.InitEnv.IsUnknown() {
		var env map[string]string
		diags.Append(m.InitEnv.ElementsAs(ctx, &env, false)...)
		p["initenv"] = env
	}
	if !m.CapabilitiesState.IsNull() && !m.CapabilitiesState.IsUnknown() {
		var capState map[string]bool
		diags.Append(m.CapabilitiesState.ElementsAs(ctx, &capState, false)...)
		p["capabilities_state"] = capState
	}

	return p, diags
}

// threeWayString applies the three-way nullable convention to payload key
// key from field: unknown omits the key entirely (leaving the TrueNAS-side
// value/default unchanged), null sends an explicit JSON nil (clearing it),
// and a known value sends the string. Mirrors lxc_config's
// PreferredPool/Bridge handling.
func threeWayString(payload map[string]any, key string, field types.String) {
	if field.IsUnknown() {
		return
	}
	if field.IsNull() {
		payload[key] = nil
		return
	}
	payload[key] = field.ValueString()
}

// isContainerAlreadyStopped reports whether err is one of the two libvirt
// "already stopped" failures container.stop's job can fail with, rather
// than a genuine failure: "Domain '<name>' does not exist" (probed live —
// a container that was NEVER started) or "Domain '<name>' is not active"
// (probed live — a container that WAS started at least once but is
// already stopped, e.g. Delete running immediately after an apply that
// left running=false). Both must be tolerated by callers that stop
// best-effort before deleting or setting running=false.
//
// This must be checked against the error text directly rather than
// client.IsNotFound: CallJob wraps a FAILED job's error into a plain
// fmt.Errorf (job.go), not a *client.APIError, so the errname/reason-code
// checks IsNotFound relies on never match here.
func isContainerAlreadyStopped(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "domain") {
		return false
	}
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "is not active")
}
