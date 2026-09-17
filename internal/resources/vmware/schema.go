// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// resourceSchema returns the schema for truenas_vmware.
//
// Field shape (id, datastore, filesystem, hostname, username, password,
// state) probed live via core.get_methods("vmware") against both TrueNAS
// TrueNAS 25.10.4 HA (wss://10.220.16.188/api/current) and 26.0
// (wss://192.168.1.68/api/current) on 2026-07-23: identical accepts/returns
// on both releases, all vmware.* methods (create/get_instance/query/update/
// delete/dataset_has_vms/get_datastores/match_datastores_with_datasets)
// report "job": false (synchronous, no CallJob polling needed).
//
// DECISIVE PROBE (vCenter/ESXi endpoint validation), run against BOTH
// probed releases:
//
//	vmware.create({
//	  "datastore": "tf-probe-datastore", "filesystem": "tank",
//	  "hostname": "192.0.2.123",              // RFC 5737 TEST-NET-1, unreachable
//	  "username": "tfprobeuser", "password": "tf-probe-fake-password-1234",
//	})
//	=> TrueNAS 25.10.4 HA: truenas API error (code 22): [EINVAL]
//	   vmware_create.datastore: Failed to connect: [ENETUNREACH]
//	   [Errno 101] Network is unreachable
//	=> TrueNAS 26.0: truenas API error (code 22): [EINVAL]
//	   vmware_create.datastore: Failed to connect: [ETIMEDOUT]
//	   [Errno 110] Connection timed out
//
// This confirms vmware.create validates hostname/username/password against
// the real vCenter/ESXi endpoint synchronously before persisting
// anything — an unreachable TEST-NET-1 host and fabricated credentials are
// rejected outright on both releases, never stored (vmware.query returned
// `[]` on both boxes both before and after the probe). Per the task brief's
// decision rule ("if create DOES validate -> documented-skip acceptance +
// full units"), this resource's acceptance test is a permanent, documented
// skip (see acceptance_test.go), mirroring internal/resources/app_registry's
// TestAccAppRegistry_basic / internal/resources/cloud_backup's
// TestAccCloudBackup_basic precedent.
func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a TrueNAS VMware snapshot integration (vmware.*): coordinates ZFS " +
			"snapshots of a local filesystem/dataset with VMware VM snapshots on a vCenter/ESXi host, so backups " +
			"of running VMs are consistent. NOTE: vmware.create validates the supplied hostname/username/password " +
			"against the real vCenter/ESXi endpoint before persisting anything (probed live on both TrueNAS " +
			"25.10.4 HA and 26.0 with a throwaway create against an unreachable RFC 5737 TEST-NET-1 host: " +
			"rejected with a connection error on both releases), so this resource's acceptance test is a " +
			"permanent, documented skip — see acceptance_test.go.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the VMware configuration.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"datastore": schema.StringAttribute{
				Required:    true,
				Description: "Valid datastore name which exists on the VMware host.",
			},
			"filesystem": schema.StringAttribute{
				Required:    true,
				Description: "ZFS filesystem or dataset to use for VMware storage.",
			},
			"hostname": schema.StringAttribute{
				Required: true,
				Description: "Valid IP address / hostname of a VMware host. When clustering, this is the " +
					"vCenter server for the cluster. Validated against the real endpoint at apply time (see the " +
					"resource-level description).",
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Credentials used to authorize access to the VMware host.",
			},
			"password": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Password for VMware host authentication. Write-only: never stored in Terraform " +
					"state. Requires Terraform >= 1.11. Validated against the real vCenter/ESXi endpoint at apply " +
					"time (see the resource-level description).",
			},
			"state": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Current connection and synchronization state with the VMware host.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"state": schema.StringAttribute{
						Computed: true,
						Description: "VMware host state: PENDING (no snapshot operation performed yet), " +
							"SUCCESS (last operation succeeded), ERROR (last operation failed), or BLOCKED " +
							"(network activity is blocked).",
					},
					"error": schema.StringAttribute{
						Computed:    true,
						Description: "Error text from the last snapshot operation, if any. Null when there is none.",
					},
					"datetime": schema.StringAttribute{
						Computed:    true,
						Description: "Timestamp of the last state update, if any. Null when there is none.",
					},
				},
			},
		},
	}
}
