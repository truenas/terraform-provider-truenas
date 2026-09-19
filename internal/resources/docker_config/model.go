// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// dockerConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one Docker configuration per TrueNAS system
// (docker.config always returns a single record), and it is never created
// or deleted on TrueNAS itself. The API's own numeric "id" (probed: always
// 1) is an internal implementation detail and is intentionally not
// surfaced in the model, mirroring the audit_config singleton pattern.
const dockerConfigResourceID = "docker_config"

// addressPoolAttrTypes describes the attribute types of one entry in the
// "address_pools" list. Identical shape on TrueNAS 25.10 and 26.0 (probed
// live, see task-1-report.md): no version gating needed here.
var addressPoolAttrTypes = map[string]attr.Type{
	"base": types.StringType,
	"size": types.Int64Type,
}

// AddressPoolModel maps to one entry of the "address_pools" list: a network
// address pool docker.update uses to allocate per-container-network subnets
// from.
type AddressPoolModel struct {
	Base types.String `tfsdk:"base"`
	Size types.Int64  `tfsdk:"size"`
}

// registryMirrorAttrTypes describes the attribute types of one entry in the
// unified "registry_mirrors" list — the Terraform-facing shape, modeled
// after TrueNAS 26.0+'s native wire shape (see dockerConfigAPI doc comment
// for why 25.10's two-flat-array shape is translated into this one).
var registryMirrorAttrTypes = map[string]attr.Type{
	"url":      types.StringType,
	"insecure": types.BoolType,
}

// RegistryMirrorModel maps to one entry of the unified "registry_mirrors"
// list.
type RegistryMirrorModel struct {
	URL      types.String `tfsdk:"url"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

// DockerConfigModel is the Terraform state model for truenas_docker_config.
//
// Nvidia is readable on both probed releases (docker.config genuinely
// returns a real boolean for it on TrueNAS 26.0+, not null — see
// dockerConfigAPI's doc comment) but is write-restricted: TrueNAS 26.0+
// removed "nvidia" from docker.update's accepted fields (probed live, see
// task-1-report.md), so this resource can display but no longer change it
// there. Explicitly setting it in HCL against a 26.0+ target is a
// apply-time error (see resource.go's applyNvidiaSupport, which uses the
// raw practitioner Config — not the resolved Plan — to distinguish "the
// user wrote this" from "UseStateForUnknown carried the previous known
// value forward").
//
// migrate_applications is intentionally NOT modeled: it is an apply-time
// action flag docker.update accepts to control whether existing
// applications are migrated when "pool" changes, not persisted
// configuration state — there is nothing for docker.config to read back,
// so it has no place in a Terraform resource model.
type DockerConfigModel struct {
	ID                 types.String `tfsdk:"id"`
	EnableImageUpdates types.Bool   `tfsdk:"enable_image_updates"`
	Pool               types.String `tfsdk:"pool"` // nullable
	Dataset            types.String `tfsdk:"dataset"`
	Nvidia             types.Bool   `tfsdk:"nvidia"` // read-only on TrueNAS 26.0+ (see doc comment above)
	AddressPools       types.List   `tfsdk:"address_pools"`
	CIDRv6             types.String `tfsdk:"cidr_v6"`
	RegistryMirrors    types.List   `tfsdk:"registry_mirrors"`
}

// DockerConfigDataSourceModel is the read-only model for the
// truenas_docker_config datasource.
type DockerConfigDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	EnableImageUpdates types.Bool   `tfsdk:"enable_image_updates"`
	Pool               types.String `tfsdk:"pool"`
	Dataset            types.String `tfsdk:"dataset"`
	Nvidia             types.Bool   `tfsdk:"nvidia"`
	AddressPools       types.List   `tfsdk:"address_pools"`
	CIDRv6             types.String `tfsdk:"cidr_v6"`
	RegistryMirrors    types.List   `tfsdk:"registry_mirrors"`
}

// addressPoolAPI mirrors one entry of docker.config/docker.update's
// "address_pools" array. Identical shape on both probed releases.
type addressPoolAPI struct {
	Base string `json:"base"`
	Size int64  `json:"size"`
}

// registryMirrorAPI mirrors one entry of TrueNAS 26.0+'s unified
// "registry_mirrors" array.
type registryMirrorAPI struct {
	URL      string `json:"url"`
	Insecure bool   `json:"insecure"`
}

// dockerConfigAPI mirrors the JSON object returned by docker.config and
// docker.update. Probed live against TrueNAS 25.10 and 26.0 (see
// task-1-report.md) — the two releases genuinely disagree on two fields:
//
//   - "nvidia" (bool): docker.config's response includes a real value for
//     this on BOTH releases (confirmed live: 26.0 returned "nvidia": false
//     even though its core.get_methods schema metadata no longer *declares*
//     the field). What actually differs is the write side — docker.update's
//     accepted-fields schema dropped "nvidia" entirely on 26.0+, so it can
//     be read but not changed there; see resource.go's applyNvidiaSupport.
//     Nvidia is still a pointer, defensively: if some future response ever
//     did omit the key outright, nil distinguishes that ("field truly
//     absent") from "field present and false" rather than silently
//     misreporting false.
//   - registry mirrors: 25.10 returns two flat string arrays,
//     "secure_registry_mirrors" and "insecure_registry_mirrors"; 26.0+
//     replaced both with a single "registry_mirrors" array of
//     {url, insecure} objects. Both shapes are decoded into this struct;
//     exactly one of RegistryMirrors vs. Secure/InsecureRegistryMirrors is
//     populated (non-nil) per response, since a JSON key that's absent
//     from the wire response leaves its Go field nil (an empty JSON array
//     "[]", by contrast, decodes to a non-nil empty slice) — see
//     hasUnifiedRegistryMirrors below, which keys shape detection in
//     responseToModel off exactly this nil-vs-non-nil distinction rather
//     than a live version probe.
//
// All other fields (dataset, pool, address_pools, cidr_v6) are identical
// between releases.
type dockerConfigAPI struct {
	ID                      int64               `json:"id"`
	EnableImageUpdates      bool                `json:"enable_image_updates"`
	Dataset                 *string             `json:"dataset"`
	Pool                    *string             `json:"pool"`
	Nvidia                  *bool               `json:"nvidia"`
	AddressPools            []addressPoolAPI    `json:"address_pools"`
	CIDRv6                  string              `json:"cidr_v6"`
	RegistryMirrors         []registryMirrorAPI `json:"registry_mirrors"`
	SecureRegistryMirrors   []string            `json:"secure_registry_mirrors"`
	InsecureRegistryMirrors []string            `json:"insecure_registry_mirrors"`
}

// hasUnifiedRegistryMirrors reports whether this response used TrueNAS
// 26.0+'s single "registry_mirrors" key (present, even if empty).
func (api *dockerConfigAPI) hasUnifiedRegistryMirrors() bool {
	return api.RegistryMirrors != nil
}

// addressPoolsListValue builds a types.List for the "address_pools"
// attribute from an API response.
func addressPoolsListValue(ctx context.Context, api []addressPoolAPI) (types.List, diag.Diagnostics) {
	pools := make([]AddressPoolModel, 0, len(api))
	for _, p := range api {
		pools = append(pools, AddressPoolModel{
			Base: types.StringValue(p.Base),
			Size: types.Int64Value(p.Size),
		})
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: addressPoolAttrTypes}, pools)
}

// registryMirrorsListValue builds the unified "registry_mirrors" list from
// an API response, translating TrueNAS 25.10's split string-array shape into
// the same {url, insecure} shape TrueNAS 26.0+ returns natively. Secure
// entries (insecure=false) are ordered first, matching the order
// docker.config itself returns the two source arrays in.
func registryMirrorsListValue(ctx context.Context, api *dockerConfigAPI) (types.List, diag.Diagnostics) {
	var mirrors []RegistryMirrorModel

	if api.hasUnifiedRegistryMirrors() {
		mirrors = make([]RegistryMirrorModel, 0, len(api.RegistryMirrors))
		for _, m := range api.RegistryMirrors {
			mirrors = append(mirrors, RegistryMirrorModel{
				URL:      types.StringValue(m.URL),
				Insecure: types.BoolValue(m.Insecure),
			})
		}
	} else {
		mirrors = make([]RegistryMirrorModel, 0, len(api.SecureRegistryMirrors)+len(api.InsecureRegistryMirrors))
		for _, u := range api.SecureRegistryMirrors {
			mirrors = append(mirrors, RegistryMirrorModel{URL: types.StringValue(u), Insecure: types.BoolValue(false)})
		}
		for _, u := range api.InsecureRegistryMirrors {
			mirrors = append(mirrors, RegistryMirrorModel{URL: types.StringValue(u), Insecure: types.BoolValue(true)})
		}
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: registryMirrorAttrTypes}, mirrors)
}

// responseToModel maps a dockerConfigAPI response onto a DockerConfigModel.
func responseToModel(ctx context.Context, api *dockerConfigAPI, m *DockerConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(dockerConfigResourceID)
	m.EnableImageUpdates = types.BoolValue(api.EnableImageUpdates)
	m.Pool = types.StringPointerValue(api.Pool)
	m.Dataset = types.StringPointerValue(api.Dataset)
	if api.Nvidia != nil {
		m.Nvidia = types.BoolValue(*api.Nvidia)
	} else {
		m.Nvidia = types.BoolNull()
	}
	m.CIDRv6 = types.StringValue(api.CIDRv6)

	pools, d := addressPoolsListValue(ctx, api.AddressPools)
	diags.Append(d...)
	m.AddressPools = pools

	mirrors, d := registryMirrorsListValue(ctx, api)
	diags.Append(d...)
	m.RegistryMirrors = mirrors

	return diags
}

// responseToDataSourceModel maps a dockerConfigAPI response onto a
// DockerConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *dockerConfigAPI, m *DockerConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(dockerConfigResourceID)
	m.EnableImageUpdates = types.BoolValue(api.EnableImageUpdates)
	m.Pool = types.StringPointerValue(api.Pool)
	m.Dataset = types.StringPointerValue(api.Dataset)
	if api.Nvidia != nil {
		m.Nvidia = types.BoolValue(*api.Nvidia)
	} else {
		m.Nvidia = types.BoolNull()
	}
	m.CIDRv6 = types.StringValue(api.CIDRv6)

	pools, d := addressPoolsListValue(ctx, api.AddressPools)
	diags.Append(d...)
	m.AddressPools = pools

	mirrors, d := registryMirrorsListValue(ctx, api)
	diags.Append(d...)
	m.RegistryMirrors = mirrors

	return diags
}

// addressPoolsPayload converts the plan's "address_pools" list into the
// []map[string]any docker.update expects.
func addressPoolsPayload(ctx context.Context, l types.List) ([]map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var pools []AddressPoolModel
	diags.Append(l.ElementsAs(ctx, &pools, false)...)
	if diags.HasError() {
		return nil, diags
	}

	out := make([]map[string]any, 0, len(pools))
	for _, p := range pools {
		out = append(out, map[string]any{
			"base": p.Base.ValueString(),
			"size": p.Size.ValueInt64(),
		})
	}
	return out, diags
}

// registryMirrorsPayload extracts the plan's unified "registry_mirrors"
// list into a plain []RegistryMirrorModel, for the caller to shape into
// whichever wire format the target release accepts (see
// unifiedRegistryMirrorsPayload/splitRegistryMirrorsPayload).
func registryMirrorsPayload(ctx context.Context, l types.List) ([]RegistryMirrorModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	var mirrors []RegistryMirrorModel
	diags.Append(l.ElementsAs(ctx, &mirrors, false)...)
	return mirrors, diags
}

// unifiedRegistryMirrorsPayload shapes a []RegistryMirrorModel into TrueNAS
// 26.0+'s native "registry_mirrors" wire value: a list of
// {url, insecure} maps.
func unifiedRegistryMirrorsPayload(mirrors []RegistryMirrorModel) []map[string]any {
	out := make([]map[string]any, 0, len(mirrors))
	for _, m := range mirrors {
		out = append(out, map[string]any{
			"url":      m.URL.ValueString(),
			"insecure": m.Insecure.ValueBool(),
		})
	}
	return out
}

// splitRegistryMirrorsPayload shapes a []RegistryMirrorModel into TrueNAS
// 25.10's two-flat-array wire value: "secure_registry_mirrors" (insecure
// == false entries) and "insecure_registry_mirrors" (insecure == true
// entries), each a plain list of URL strings.
func splitRegistryMirrorsPayload(mirrors []RegistryMirrorModel) (secure, insecure []string) {
	secure = []string{}
	insecure = []string{}
	for _, m := range mirrors {
		if m.Insecure.ValueBool() {
			insecure = append(insecure, m.URL.ValueString())
		} else {
			secure = append(secure, m.URL.ValueString())
		}
	}
	return secure, insecure
}

// updatePayload builds the docker.update argument.
//
// enable_image_updates, pool, cidr_v6, address_pools, and registry_mirrors
// are guarded: each is only included when known (Optional+Computed), so an
// unset optional is omitted entirely and the TrueNAS-side current value is
// left unchanged rather than overwritten with an explicit zero value. pool
// is nullable: an explicit null is sent through as nil (clearing the
// pool), distinct from "unset" (omitted, current value unchanged).
//
// registryMirrorsUnified selects the wire shape for registry_mirrors: true
// sends TrueNAS 26.0+'s native "registry_mirrors" key; false sends TrueNAS
// 25.10's "secure_registry_mirrors" + "insecure_registry_mirrors" split.
// This is a plain parameter (not a live client call) so the translation
// logic here stays a pure function, independently unit-testable for both
// wire shapes without a live TrueNAS connection — see resource.go for
// where the caller determines this via client.VersionAtLeast.
//
// nvidia is intentionally NOT handled here: it needs its own version-gated
// error path (TrueNAS 26.0+ rejects it outright rather than silently
// ignoring it), which requires diagnostics the resource layer owns — see
// resource.go's applyNvidiaSupport.
func (m *DockerConfigModel) updatePayload(ctx context.Context, registryMirrorsUnified bool) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.EnableImageUpdates.IsNull() && !m.EnableImageUpdates.IsUnknown() {
		p["enable_image_updates"] = m.EnableImageUpdates.ValueBool()
	}
	if !m.Pool.IsUnknown() {
		if m.Pool.IsNull() {
			p["pool"] = nil
		} else {
			p["pool"] = m.Pool.ValueString()
		}
	}
	if !m.CIDRv6.IsNull() && !m.CIDRv6.IsUnknown() {
		p["cidr_v6"] = m.CIDRv6.ValueString()
	}

	if !m.AddressPools.IsNull() && !m.AddressPools.IsUnknown() {
		pools, d := addressPoolsPayload(ctx, m.AddressPools)
		diags.Append(d...)
		if !diags.HasError() {
			p["address_pools"] = pools
		}
	}

	if !m.RegistryMirrors.IsNull() && !m.RegistryMirrors.IsUnknown() {
		mirrors, d := registryMirrorsPayload(ctx, m.RegistryMirrors)
		diags.Append(d...)
		if !diags.HasError() {
			if registryMirrorsUnified {
				p["registry_mirrors"] = unifiedRegistryMirrorsPayload(mirrors)
			} else {
				secure, insecure := splitRegistryMirrorsPayload(mirrors)
				p["secure_registry_mirrors"] = secure
				p["insecure_registry_mirrors"] = insecure
			}
		}
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the Docker pool/network configuration
// underpins every running application, so removing this resource from
// Terraform state must never reset or reconfigure the box's Docker setup.
// Splitting this into its own function keeps Delete's "no client calls"
// contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Docker configuration left in place",
		"Docker configuration left in place; removed from Terraform state only",
	)
	return diags
}
