// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// resourceSchema returns the schema for truenas_app_registry.
//
// Field shape (name, description, uri, username, password) probed live via
// core.get_methods("app.registry") against both TrueNAS 25.10.3.1
// (wss://192.168.1.249/api/current) and 26.0 (wss://192.168.1.68/api/current)
// on 2026-07-23: identical accepts/returns on both releases, all five
// app.registry methods (create/query/get_instance/update/delete) report
// "job": false (synchronous, no CallJob polling needed).
//
// DECISIVE PROBE (credential/uri validation), run against 26.0 (apps
// configured and running there; see below for why 25.10 could not be used
// for this probe):
//
//	app.registry.create({
//	  "name": "tf-probe-registry-decisive",
//	  "description": "tf probe decisive - safe to delete",
//	  "uri": "https://192.0.2.123:5000",       // RFC 5737 TEST-NET-1, unreachable
//	  "username": "tfprobeuser", "password": "tfprobepassword123",
//	})
//	=> truenas API error (code 22): [EINVAL] app_registry_create.uri:
//	   Invalid credentials for registry
//
// This confirms app.registry.create validates username/password/uri
// against the real registry endpoint synchronously before persisting
// anything — an unreachable TEST-NET-1 host and fabricated credentials are
// rejected outright, never stored. Per the task brief's decision rule ("if
// create DOES validate -> documented-skip acceptance + full units"), this
// resource's acceptance test is a permanent, documented skip (see
// acceptance_test.go), mirroring internal/resources/cloud_backup's
// TestAccCloudBackup_basic precedent.
//
// The identical probe against 25.10.3.1 could not reach this validation
// path at all: that box does not have Docker/apps configured, and
// app.registry.create failed earlier with `truenas API error (code 14):
// [EFAULT] No pool configured for Docker` — an apps-unconfigured error, not
// a credential/uri rejection. Per the task's guidance, 26.0 (apps
// configured and running) was therefore treated as the primary probe box
// for the decisive test; 25.10's error is recorded here as supplementary
// evidence that the schema/field shape itself is unaffected (both boxes
// agree on accepts/returns), just gated behind an earlier apps-config
// check when Docker isn't set up.
//
// Also probed: app.registry.query() (no filters) returned `[]` — an empty
// list — on BOTH boxes, including 26.0 where apps are configured and
// running. There is no default/pre-existing "docker.io" registry entry
// auto-created by TrueNAS, contrary to the task brief's expectation ("default
// docker.io likely"). Nothing pre-existing was found, so nothing was ever at
// risk of being touched by probing.
func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a TrueNAS app container registry (app.registry): credentials used by the Apps " +
			"(Docker) subsystem to authenticate against a container registry. NOTE: app.registry.create validates " +
			"the supplied username/password/uri against the real registry endpoint before persisting anything " +
			"(probed live on TrueNAS 26.0 with a throwaway create against an unreachable TEST-NET-1 host: " +
			"rejected with \"Invalid credentials for registry\"), so this resource's acceptance test is a " +
			"permanent, documented skip — see acceptance_test.go.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric app registry entry ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name for the container registry.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional description of the container registry.",
			},
			"uri": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Container registry URI endpoint. Defaults to \"https://index.docker.io/v1/\" (Docker Hub) when omitted.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username for registry authentication.",
			},
			"password": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Password or access token for registry authentication. Write-only: never stored in " +
					"Terraform state. Requires Terraform >= 1.11. Validated against the real registry endpoint at " +
					"apply time (see the resource-level description).",
			},
		},
	}
}
