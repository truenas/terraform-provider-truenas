// Copyright (c) iXsystems, Inc.
// SPDX-License-Identifier: MPL-2.0

package system_general_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSystemGeneralDataSource_basic reads the current TrueNAS system
// general configuration through the truenas_system_general datasource only.
// It never writes: this configuration controls the management UI (bind
// addresses, ports, certificate), and the box's real configuration must not
// be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSystemGeneralDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_system_general" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_general.test", "id", "system_general"),
					resource.TestCheckResourceAttrSet("data.truenas_system_general.test", "timezone"),
				),
			},
		},
	})
}

// TestAccSystemGeneral_basic is intentionally skipped by default.
// truenas_system_general is a SINGLETON resource that controls the
// management UI: the box this suite runs against is managed live through
// that UI, and bringing this configuration under Terraform management
// mutates the box's actual system general configuration
// (system.general.update). A naive create/update/destroy acceptance test
// risks changing ui_address, ui_port, ui_httpsport, ui_allowlist, or
// ui_certificate on a live system and cutting off management access
// entirely.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk field (e.g. "ui_consolemsg") in the resource
//     config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch ui_address, ui_allowlist, ui_port, ui_httpsport,
//     ui_v6address, or ui_certificate in an automated test: changing those
//     on a live system can cut off management access.
//  3. Use ImportState with ImportStateId "system_general" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccSystemGeneral_basic(t *testing.T) {
	t.Skip("truenas_system_general controls the LIVE management UI; skipped to avoid cutting off management access to the target box. See comment on TestAccSystemGeneral_basic for how to safely enable this against a disposable instance.")
}
