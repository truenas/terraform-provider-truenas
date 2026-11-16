// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBShare_basic creates a dataset fixture and an SMB share on its
// mountpoint, checks its attributes, updates comment and abe in place,
// imports it by its numeric id, and verifies destruction of both the share
// and the dataset fixture.
func TestAccSMBShare_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-smb"))
	shareName := acctest.RandName("tf-acc-smb")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckSMBShareDestroyed(shareName),
			testAccCheckSMBDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBShareConfig(datasetName, shareName, "initial comment", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "name", shareName),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "comment", "initial comment"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "abe", "false"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "hostsallow.#", "2"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "hostsallow.0", "127.0.0.1"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "path"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "vuid"),
				),
			},
			// Update in place: change comment and enable ABE.
			{
				Config: acctest.ProviderConfig() + testAccSMBShareConfig(datasetName, shareName, "updated comment", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "comment", "updated comment"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "abe", "true"),
				),
			},
			// Import by the share's numeric id.
			{
				ResourceName:      "truenas_smb_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSMBShare_audit exercises the nested audit block: enable auditing with
// a watch_list, verify the values round trip, and import.
func TestAccSMBShare_audit(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-smb-audit-ds"))
	shareName := acctest.RandName("tfaccsmbaudit")
	groupName := acctest.RandName("tfaccsmbauditgrp")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSMBShareDestroyed(shareName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBShareAuditConfig(datasetName, shareName, groupName,
					"enable = true\n    watch_list = [truenas_group.audit.name]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.enable", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.watch_list.#", "1"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.watch_list.0", groupName),
				),
			},
			// In-place update: disable auditing (clears the watch_list requirement).
			{
				Config: acctest.ProviderConfig() + testAccSMBShareAuditConfig(datasetName, shareName, groupName,
					"enable = false"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.enable", "false"),
				),
			},
			{
				ResourceName:      "truenas_smb_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSMBShareAuditConfig(datasetName, shareName, groupName, auditBody string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

# Auditing requires a non-empty watch_list or ignore_list of real SMB groups,
# so provision a throwaway SMB-enabled group to audit.
resource "truenas_group" "audit" {
  name = %q
  smb  = true
}

resource "truenas_smb_share" "test" {
  path    = truenas_dataset.test.mountpoint
  name    = %q
  enabled = true
  audit = {
    %s
  }
}
`, datasetName, groupName, shareName, auditBody)
}

func testAccSMBShareConfig(datasetName, shareName, comment string, abe bool) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

# timemachine requires Apple SMB2/3 protocol extensions enabled server-side.
resource "truenas_smb_config" "aapl" {
  aapl_extensions = true
}

resource "truenas_smb_share" "test" {
  path       = truenas_dataset.test.mountpoint
  name       = %q
  comment    = %q
  enabled    = true
  abe        = %v
  hostsallow = ["127.0.0.1", "192.168.1.0/24"]

  ro                = false
  browsable         = true
  recyclebin        = true
  guestok           = false
  home              = false
  acl               = true
  durablehandle     = true
  streams           = true
  hostsdeny         = ["10.0.0.0/8"]
  timemachine       = true
  timemachine_quota = 10737418240

  depends_on = [truenas_smb_config.aapl]
}
`, datasetName, shareName, comment, abe)
}

func testAccCheckSMBShareDestroyed(shareName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "sharing.smb.query", [][]any{{"name", "=", shareName}})
		if err != nil {
			return fmt.Errorf("error checking SMB share %s: %v", shareName, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing sharing.smb.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("SMB share %s still exists", shareName)
		}
		return nil
	}
}

func testAccCheckSMBDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}

// TestAccSMBShare_options exercises the typed options object, which exists only
// on TrueNAS 26.0+ (the discriminated purpose/options model). It uses a
// DEFAULT_SHARE, whose options variant is {aapl_name_mangling, hostsallow,
// hostsdeny}.
func TestAccSMBShare_options(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("smb_share options requires TrueNAS 26.0+ (discriminated purpose/options model)")
	}
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-smbopt"))
	shareName := acctest.RandName("tfaccsmbopt")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckSMBShareDestroyed(shareName),
			testAccCheckSMBDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBShareOptionsConfig(datasetName, shareName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "purpose", "DEFAULT_SHARE"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "options.aapl_name_mangling", "true"),
				),
			},
			{
				ResourceName:      "truenas_smb_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSMBShare_hostsallowReorderNoDiff verifies the pool #66 bug class for
// SMB: hostsallow/hostsdeny are order-insensitive sets, so re-applying the
// same hosts in a different order than they were created (and than the server
// returns them) must produce an empty plan, not a perpetual update diff. The
// flat hostsallow attribute maps to a LEGACY_SHARE options.hostsallow on the
// wire.
func TestAccSMBShare_hostsallowReorderNoDiff(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffixreord"))
	name := acctest.RandName("tffixreord")
	// The top-level LEGACY_SHARE Optional+Computed bools lack UseStateForUnknown,
	// so leaving them unset makes every re-plan show them "known after apply" —
	// a pre-existing churn unrelated to this fix. Pin them (to their server
	// defaults) in both steps so the empty-plan assertion isolates the one thing
	// under test: that reordering hostsallow/hostsdeny is a no-op.
	pinned := `
  acl               = true
  durablehandle     = true
  guestok           = false
  home              = false
  recyclebin        = false
  streams           = true
  timemachine       = false
  timemachine_quota = 0
`
	c1 := smbShareConfig(ds, name, pinned+`
  hostsallow = ["10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"]
  hostsdeny  = ["203.0.113.0/24", "198.51.100.0/24", "192.0.2.0/24"]
`)
	// Same elements, different order: must be a no-op plan.
	c2 := smbShareConfig(ds, name, pinned+`
  hostsallow = ["172.16.0.0/12", "10.0.0.0/8", "192.168.1.0/24"]
  hostsdeny  = ["192.0.2.0/24", "203.0.113.0/24", "198.51.100.0/24"]
`)
	n := "truenas_smb_share.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c1,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(n, "hostsallow.#", "3"),
					resource.TestCheckResourceAttr(n, "hostsdeny.#", "3"),
				),
			},
			{
				Config:             c2,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccSMBShare_optionsHostsallowReorderNoDiff is the nested-options twin of
// TestAccSMBShare_hostsallowReorderNoDiff: options.hostsallow/hostsdeny on a
// DEFAULT_SHARE (26.0+) must also treat a reorder as a no-op.
//
// A whole-plan empty assertion cannot be used here: a DEFAULT_SHARE's many other
// Optional+Computed options.* fields lack per-field UseStateForUnknown, so they
// re-read as "known after apply" on any re-plan (a pre-existing SMB churn,
// unrelated to this fix). So we assert at the nested path instead — the reorder
// must not replace the share, and options.hostsallow/hostsdeny must stay known
// 3-element lists rather than flip to a change/unknown. The element-level
// order-insensitivity itself is proven by TestAccSMBShare_hostsallowReorderNoDiff
// (which passes empty-plan on 25.10 and 26.0) plus the normalize unit tests.
func TestAccSMBShare_optionsHostsallowReorderNoDiff(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("smb_share options requires TrueNAS 26.0+ (discriminated purpose/options model)")
	}
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffixoreord"))
	name := acctest.RandName("tffixoreord")
	c1 := smbShareConfig(ds, name, `
  purpose = "DEFAULT_SHARE"
  options = {
    hostsallow = ["10.0.0.0/8", "192.168.1.0/24", "172.16.0.0/12"]
    hostsdeny  = ["203.0.113.0/24", "198.51.100.0/24", "192.0.2.0/24"]
  }
`)
	c2 := smbShareConfig(ds, name, `
  purpose = "DEFAULT_SHARE"
  options = {
    hostsallow = ["172.16.0.0/12", "10.0.0.0/8", "192.168.1.0/24"]
    hostsdeny  = ["192.0.2.0/24", "203.0.113.0/24", "198.51.100.0/24"]
  }
`)
	n := "truenas_smb_share.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c1,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(n, "options.hostsallow.#", "3"),
					resource.TestCheckResourceAttr(n, "options.hostsdeny.#", "3"),
				),
			},
			{
				// Re-apply the reordered config. The plan is inspected before
				// apply: the reorder must not replace the share, and
				// options.hostsallow/hostsdeny must stay known 3-element lists
				// rather than flip to a change or to unknown. (A whole-plan empty
				// assertion can't be used — see the test doc comment.)
				Config: c2,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						acctest.ExpectNoReplace(n),
						plancheck.ExpectKnownValue(n, tfjsonpath.New("options").AtMapKey("hostsallow"), knownvalue.ListSizeExact(3)),
						plancheck.ExpectKnownValue(n, tfjsonpath.New("options").AtMapKey("hostsdeny"), knownvalue.ListSizeExact(3)),
					},
				},
				ExpectNonEmptyPlan: true, // unrelated options.* computed fields churn to unknown
			},
		},
	})
}

func testAccSMBShareOptionsConfig(datasetName, shareName string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

resource "truenas_smb_share" "test" {
  path    = truenas_dataset.test.mountpoint
  name    = %q
  enabled = true
  purpose = "DEFAULT_SHARE"
  options = {
    aapl_name_mangling = true
    hostsallow         = ["10.0.0.0/8"]
    hostsdeny          = ["192.168.99.0/24"]
  }
}
`, datasetName, shareName)
}
