// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NVMe-oF namespace (the block device behind a subsystem) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NVMe-oF namespace ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"subsys_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the NVMe-oF subsystem this namespace belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"device_path": schema.StringAttribute{
				Required:    true,
				Description: "Path of the backing device, e.g. \"zvol/tank/vms/vm1\" or a file path on a dataset. Changing it (or device_type) on an enabled namespace briefly disables the namespace while the device is switched, because TrueNAS refuses the change on an active namespace; device_uuid and device_nguid are kept.",
			},
			"device_type": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("FILE", "ZVOL")},
				Optional:    true,
				Computed:    true,
				Description: "Backing device type: ZVOL or FILE. The API requires it; when omitted it is derived from device_path (ZVOL for a `zvol/` path, FILE otherwise).",
				PlanModifiers: []planmodifier.String{
					deviceTypeFromPath{},
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the namespace is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"filesize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Size in bytes of the backing file. Only applicable for FILE device_type.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"nsid": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Namespace ID within the subsystem. Auto-assigned by TrueNAS when unset. Immutable after creation.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			// Computed-only (server-generated)
			"device_nguid": schema.StringAttribute{
				Computed:    true,
				Description: "NVMe Globally Unique Identifier for the namespace's backing device.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"device_uuid": schema.StringAttribute{
				Computed:    true,
				Description: "UUID for the namespace's backing device.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"locked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the namespace's backing device is locked (e.g. an encrypted dataset). Not stabilized with UseStateForUnknown since the server can change this independently of Terraform.",
			},
		},
	}
}

// inferDeviceType returns the device_type implied by a device_path. TrueNAS
// requires ZVOL paths to start with "zvol/"; any other path is a file on a
// dataset. A "/dev/zvol/..." path maps to ZVOL so the server reports the
// path-form error ("must start with zvol/") rather than a FILE error.
func inferDeviceType(devicePath string) string {
	if strings.HasPrefix(devicePath, "zvol/") || strings.HasPrefix(devicePath, "/dev/zvol/") {
		return "ZVOL"
	}
	return "FILE"
}

// deviceTypeFromPath plans device_type from device_path when it is not set in
// config. nvmet.namespace.create requires device_type (no server default), so
// leaving it unknown and omitting it failed every create that did not set it.
// Unlike UseStateForUnknown, this follows a device_path change, so switching
// from a zvol to a file does not keep a stale ZVOL.
type deviceTypeFromPath struct{}

func (deviceTypeFromPath) Description(_ context.Context) string {
	return "When unset, device_type is derived from device_path: ZVOL for a zvol/ path, FILE otherwise."
}

func (m deviceTypeFromPath) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (deviceTypeFromPath) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.Plan.Raw.IsNull() || !req.ConfigValue.IsNull() {
		return
	}
	var devicePath types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("device_path"), &devicePath)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if devicePath.IsUnknown() || devicePath.IsNull() {
		resp.PlanValue = types.StringUnknown()
		return
	}
	resp.PlanValue = types.StringValue(inferDeviceType(devicePath.ValueString()))
}
