// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware_test

// Adversarial bug-hunt test: create a truenas_vmware snapshot integration
// with every API-accepted attribute set (datastore, filesystem, hostname,
// username, password, password_wo_version), assert the read-back re-plans
// empty, then import and verify (ignoring the write-only password and its
// rotation trigger).
//
// INFRA GATE: vmware.create validates hostname/username/password against a
// real vCenter/ESXi endpoint before persisting anything (probed live on both
// TrueNAS 25.10.4 HA and 26.0 — a throwaway create with fabricated
// credentials against an unreachable RFC 5737 host was rejected with a
// connection error on each release). Fabricated values cannot drive this
// resource, which is why the committed acceptance test (TestAccVMware_basic)
// is a permanent documented skip. This test therefore requires a REAL,
// reachable VMware host supplied via env vars and self-skips otherwise.

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntVMware_allFields(t *testing.T) {
	// This is a write test that validates against (and would register with) a
	// real external VMware host, so it is doubly gated: DisruptiveCheck
	// requires TRUENAS_DISRUPTIVE=1, and a reachable vCenter/ESXi fixture must
	// be supplied through TRUENAS_TEST_VMWARE=1 plus its connection env vars.
	acctest.DisruptiveCheck(t)
	if os.Getenv("TRUENAS_TEST_VMWARE") != "1" {
		t.Skip("Set TRUENAS_TEST_VMWARE=1 and provide TRUENAS_VMWARE_HOSTNAME / _USERNAME / _PASSWORD / " +
			"_DATASTORE (and optionally _FILESYSTEM) for a real, reachable vCenter/ESXi host: vmware.create " +
			"validates these against the live endpoint, so fabricated values cannot drive this resource " +
			"(see TestAccVMware_basic for the decisive probe evidence)")
	}

	hostname := os.Getenv("TRUENAS_VMWARE_HOSTNAME")
	username := os.Getenv("TRUENAS_VMWARE_USERNAME")
	password := os.Getenv("TRUENAS_VMWARE_PASSWORD")
	datastore := os.Getenv("TRUENAS_VMWARE_DATASTORE")
	if hostname == "" || username == "" || password == "" || datastore == "" {
		t.Fatal("TRUENAS_TEST_VMWARE=1 requires TRUENAS_VMWARE_HOSTNAME, TRUENAS_VMWARE_USERNAME, " +
			"TRUENAS_VMWARE_PASSWORD, and TRUENAS_VMWARE_DATASTORE to be set to a real, reachable VMware host")
	}
	// filesystem is a local ZFS dataset; default to the test pool root when a
	// dedicated fixture dataset is not supplied.
	filesystem := os.Getenv("TRUENAS_VMWARE_FILESYSTEM")
	if filesystem == "" {
		filesystem = acctest.TestPool()
	}

	cfg := acctest.ProviderConfig() + testHuntVMwareConfig(datastore, filesystem, hostname, username, password)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vmware.test", "datastore", datastore),
					resource.TestCheckResourceAttr("truenas_vmware.test", "filesystem", filesystem),
					resource.TestCheckResourceAttr("truenas_vmware.test", "hostname", hostname),
					resource.TestCheckResourceAttr("truenas_vmware.test", "username", username),
					resource.TestCheckResourceAttrSet("truenas_vmware.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_vmware.test", "state.state"),
				),
			},
			// Adversarial re-plan: every value we set must read back unchanged.
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_vmware.test",
				ImportState:       true,
				ImportStateVerify: true,
				// password is Required+WriteOnly (never stored in state) and
				// password_wo_version is an optional rotation trigger the API
				// does not return, so neither round-trips through import.
				ImportStateVerifyIgnore: []string{"password", "password_wo_version"},
			},
		},
	})
}

func testHuntVMwareConfig(datastore, filesystem, hostname, username, password string) string {
	return `
resource "truenas_vmware" "test" {
  datastore           = "` + datastore + `"
  filesystem          = "` + filesystem + `"
  hostname            = "` + hostname + `"
  username            = "` + username + `"
  password            = "` + password + `"
  password_wo_version = 1
}
`
}
