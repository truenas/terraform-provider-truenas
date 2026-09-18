// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_image

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// containerImageVersionFloorMajor/Minor is the minimum TrueNAS
// release that exposes the container namespace (and therefore
// container.image.query_registry) at all. Probed live: TrueNAS 25.10 returns
// 0 container.* methods from core.get_methods (namespace genuinely absent).
// Mirrors the container package's own floor (see
// internal/resources/container/model.go) — kept as an independent copy
// rather than a shared import, matching this codebase's per-package
// version-gate convention (e.g. lxc_config does not import from
// docker_config even though both gate on the same kind of check).
const (
	containerImageVersionFloorMajor = 26
	containerImageVersionFloorMinor = 0
)

// versionGateDiagnostics reports whether the given TrueNAS release string
// is at or above the TrueNAS 26.0 floor container.image.query_registry
// requires, returning a single clean error diagnostic when it is not. Pure
// function of an already-probed version string (no live client access).
func versionGateDiagnostics(version string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !client.VersionAtLeastString(version, containerImageVersionFloorMajor, containerImageVersionFloorMinor) {
		diags.AddError(
			"TrueNAS version too old",
			"truenas_container_image requires TrueNAS 26.0 or later",
		)
	}
	return diags
}

// ContainerImageDataSourceModel is the read-only model for the
// truenas_container_image datasource, looked up by registry image name.
type ContainerImageDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Versions      types.List   `tfsdk:"versions"`
	LatestVersion types.String `tfsdk:"latest_version"`
}

// registryVersionAPI is the JSON wire format of one entry in a
// registryImageAPI's "versions" array.
type registryVersionAPI struct {
	Version string `json:"version"`
}

// registryImageAPI is the JSON wire format of one entry in
// container.image.query_registry's result array. Probed live: the method
// accepts NO arguments at all (a query-filter argument is rejected with
// "[EINVAL] : Too many arguments (expected 0, found 1)") and always returns
// every image name in the registry (62 observed live) — so the datasource
// must fetch the full list and filter client-side by Name.
type registryImageAPI struct {
	Name     string               `json:"name"`
	Versions []registryVersionAPI `json:"versions"`
}

// findRegistryImage returns the entry in results matching name, and
// whether it was found. Client-side filter, since query_registry itself
// takes no filter arguments (see registryImageAPI's doc comment).
func findRegistryImage(results []registryImageAPI, name string) (registryImageAPI, bool) {
	for _, entry := range results {
		if entry.Name == name {
			return entry, true
		}
	}
	return registryImageAPI{}, false
}

// versionStrings extracts the "version" string from each entry, preserving
// the registry's own order.
func versionStrings(versions []registryVersionAPI) []string {
	out := make([]string, len(versions))
	for i, v := range versions {
		out[i] = v.Version
	}
	return out
}

// latestVersion returns the last entry of versions (registry order),
// matching the versions actually observed live for
// "alpine:3.22:amd64:default" (3 entries, oldest first,
// e.g. "20260718_13:00" < "20260719_13:00" < "20260722_17:16" by build
// timestamp) — or "", false when versions is empty.
func latestVersion(versions []string) (string, bool) {
	if len(versions) == 0 {
		return "", false
	}
	return versions[len(versions)-1], true
}
