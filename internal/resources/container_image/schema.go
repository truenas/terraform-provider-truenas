// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_image

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// datasourceSchema returns the schema for the truenas_container_image data
// source. container_image is a DATASOURCE-ONLY namespace: the upstream LXC
// image registry (images.linuxcontainers.org, queried through
// container.image.query_registry) is not something this provider creates,
// updates, or deletes — there is nothing for a corresponding resource to
// manage, only a lookup (see provider.go's registration comment for how
// this is wired into the provider).
func datasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Looks up available versions of an LXC container image in the upstream registry " +
			"(container.image.query_registry). Purpose: let HCL reference a current image version for " +
			"truenas_container's \"image\" block without hardcoding one — the upstream registry " +
			"(images.linuxcontainers.org) PRUNES old builds; a version this datasource lists today can 404 on " +
			"download later once pruned (observed live: a version the registry still listed in " +
			"query_registry's response 404'd when container.create tried to download it). \"latest_version\" " +
			"(the registry's own last-listed entry, oldest-to-newest by build timestamp, confirmed live) is the " +
			"safest default to reference; a pinned older version carries this pruning risk." +
			"\n\n" +
			"Requires TrueNAS 26.0 or later (the container namespace does not exist on 25.10, confirmed " +
			"live — see truenas_container's schema description).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Same value as \"name\".",
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Registry image name to look up, e.g. \"alpine:3.22:amd64:default\". See " +
					"container.image.query_registry on the target box for the full set of available names.",
			},
			"versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Every version currently listed for this image, in registry order (oldest to " +
					"newest by build timestamp, confirmed live).",
			},
			"latest_version": schema.StringAttribute{
				Computed:    true,
				Description: "The newest version listed for this image (the last entry of \"versions\").",
			},
		},
	}
}
