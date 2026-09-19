// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service

import "github.com/hashicorp/terraform-plugin-framework/types"

// ServiceModel is the Terraform state model for a TrueNAS service.
type ServiceModel struct {
	ID      types.String `tfsdk:"id"`      // service name used as Terraform ID
	Name    types.String `tfsdk:"name"`    // service key: nfs, cifs, ssh, etc.
	Enabled types.Bool   `tfsdk:"enabled"` // autostart on boot
	Running types.Bool   `tfsdk:"running"` // current running state
}

// serviceAPI mirrors the JSON object returned by service.query.
type serviceAPI struct {
	ID      int64  `json:"id"`      // numeric API ID (used for service.update)
	Service string `json:"service"` // service name
	State   string `json:"state"`   // RUNNING or STOPPED
	Enable  bool   `json:"enable"`
}
