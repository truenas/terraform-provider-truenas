// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccDataset_basic creates a ZFS dataset, checks its computed
// attributes, updates a mutable field (comments), imports it by name, and
// verifies destruction.
func TestAccDataset_basic(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "initial comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", name),
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "lz4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "comments", "initial comment"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "aclmode", "PASSTHROUGH"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "acltype", "nfsv4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "atime", "OFF"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "checksum", "SHA256"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "copies", "2"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "dedup", "ON"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "quota", "2147483648"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "recordsize", "128K"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "sync", "ALWAYS"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "mountpoint"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "id"),
				),
			},
			// Update a mutable field in place (comments); compression left
			// unchanged to keep this a pure in-place-update step.
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "comments", "updated comment"),
				),
			},
			// Import by dataset name.
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatasetConfig(name, compression, comments string) string {
	// Full-surface: every writable ZFS property this resource models is set to a
	// non-default value so the post-apply plan (and ImportStateVerify) prove each
	// one round-trips. share_type is write-only (not read back) and left out.
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  compression = %q
  comments    = %q

  aclmode                  = "PASSTHROUGH"
  acltype                  = "nfsv4"
  atime                    = "OFF"
  exec                     = "OFF"
  checksum                 = "SHA256"
  copies                   = 2
  dedup                    = "ON"
  quota                    = 2147483648
  refquota                 = 1073741824
  reservation              = 10485760
  refreservation           = 10485760
  recordsize               = "128K"
  snapdir                  = "VISIBLE"
  special_small_block_size = 0
  sync                     = "ALWAYS"
}
`, name, compression, comments)
}

// datasetSummary is the subset of pool.dataset.query fields this test
// package needs directly (outside of the provider's own resource code).
type datasetSummary struct {
	ID string `json:"id"`
}

func testAccCheckDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		// CallRead (not Call) so the long-idle shared acctest client reconnects
		// if it went stale during the Terraform steps — a read query is idempotent.
		raw, err := c.CallRead(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []datasetSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}

// TestAccDataset_identityAfterUpdate verifies the resource populates its
// identity after an in-place update (regression for issue #20): a resource that
// declares an identity schema but omits SetIdentity in Update fails every
// update with "no resource identity data after update". Gated to Terraform
// 1.12+, where resource identity exists.
func TestAccDataset_identityAfterUpdate(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-ident"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "identity initial"),
			},
			{
				// In-place update; assert the identity's id matches state id.
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "identity updated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValueMatchesState("truenas_dataset.test", tfjsonpath.New("id")),
				},
			},
		},
	})
}

// TestAccDataset_encryptedPassphrase creates a passphrase-encrypted dataset and
// verifies the encryption state reads back (issue #18). encryption_passphrase is
// write-only; inherit_encryption / encryption_generate_key are create-only inputs
// the API does not return, so they are ignored on import.
func TestAccDataset_encryptedPassphrase(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-enc"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "enc" {
  name                  = %q
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-256-GCM"
  encryption_passphrase = "test-passphrase-1234"
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.enc", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "encryption", "true"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "encryption_algorithm", "AES-256-GCM"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "key_format", "PASSPHRASE"),
					resource.TestCheckResourceAttr("truenas_dataset.enc", "locked", "false"),
				),
			},
			{
				ResourceName:            "truenas_dataset.enc",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"encryption_passphrase", "encryption_key", "encryption_generate_key"},
			},
		},
	})
}

// TestAccDataset_encryptedGeneratedKey creates a key-based encrypted dataset
// with a generated key (issue #18), covering encryption_generate_key.
func TestAccDataset_encryptedGeneratedKey(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-enckey"))
	config := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "enckey" {
  name                    = %q
  encryption              = true
  inherit_encryption      = false
  encryption_algorithm    = "AES-256-GCM"
  encryption_generate_key = true
}
`, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		// Create, then import + re-apply the same config expecting an empty plan.
		// Guards GH #29: inherit_encryption / encryption_generate_key are create-
		// only inputs the API does not return, so an imported dataset reads them
		// back null; a plan against the config that sets them must NOT force a
		// replace (they are replaceIfChangedFromKnown, not RequiresReplace). Both
		// are ignored by import-verify because they cannot round-trip (null in
		// imported state).
		Steps: append([]resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.enckey", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_dataset.enckey", "key_format", "HEX"),
					resource.TestCheckResourceAttr("truenas_dataset.enckey", "locked", "false"),
				),
			},
		}, acctest.ImportReapplyNoop("truenas_dataset.enckey", config, "encryption_generate_key")...),
	})
}

// TestAccDataset_aclTypeUpdate guards GH #26: an in-place update of a dataset
// created with acltype=posix must not resend acltype. acltype is RequiresReplace,
// so update only ever carries the current value — but pool.dataset.update also
// writes aclmode/aclinherit=DISCARD for POSIX/OFF acltype, turning an inherited
// aclmode into a local one and failing the update with an inconsistent-result
// error. The fix strips acltype from the update payload. Step 2 (a comment-only
// change) exercises the update; the framework's post-apply empty-plan check is
// the regression assertion.
//
// Version split: 26.0+ rejects creating a POSIX/OFF dataset unless aclmode is
// explicitly DISCARD ("[EINVAL] ...aclmode: Must be set to DISCARD when acltype
// is POSIX or OFF"), so there the config sets it and the test is a smoke test of
// the posix update path. On 25.10 the server allows POSIX with an inherited
// aclmode — the exact precondition for #26 — so the config omits aclmode and the
// update reproduces the inconsistent-result bug when the fix is absent.
func TestAccDataset_aclTypeUpdate(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-acl"))
	aclmodeLine := ""
	if acctest.ServerVersionAtLeast(t, 26, 0) {
		aclmodeLine = "  aclmode  = \"DISCARD\"\n"
	}
	cfg := func(comments string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "acl" {
  name     = %q
  acltype  = "posix"
%s  comments = %q
}
`, name, aclmodeLine, comments)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg("before"), Check: resource.TestCheckResourceAttr("truenas_dataset.acl", "acltype", "posix")},
			{Config: cfg("after"), Check: resource.TestCheckResourceAttr("truenas_dataset.acl", "comments", "after")},
		},
	})
}

// TestAccDataset_shareTypeUpdate guards GH #25: a dataset created with a
// share_type preset must survive an in-place update. share_type is write-only
// and RequiresReplace; pool.dataset.update rejects it outright, so the update
// payload must strip it. Step 2 (comment-only change) exercises the update.
func TestAccDataset_shareTypeUpdate(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-st"))
	cfg := func(comments string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "st" {
  name       = %q
  share_type = "SMB"
  comments   = %q
}
`, name, comments)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg("before")},
			{Config: cfg("after"), Check: resource.TestCheckResourceAttr("truenas_dataset.st", "comments", "after")},
		},
	})
}

// TestAccDataset_mountpointStableForConsumer guards GH #27: a resource that
// consumes a dataset's mountpoint as a RequiresReplace path (here
// truenas_filesystem_acl) must not be replaced when the dataset is updated in
// place. mountpoint is Computed+UseStateForUnknown so it stays known on update
// rather than planning "known after apply", which would churn the consumer. The
// PreApply plan check asserts the ACL resource is a no-op when only the
// dataset's comments change.
func TestAccDataset_mountpointStableForConsumer(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-mp"))
	cfg := func(comments string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "mp" {
  name     = %q
  acltype  = "nfsv4"
  aclmode  = "PASSTHROUGH"
  comments = %q
}
resource "truenas_filesystem_acl" "a" {
  path    = truenas_dataset.mp.mountpoint
  entries = jsonencode([{ tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } }])
}
`, name, comments)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg("before")},
			{
				Config: cfg("after"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_filesystem_acl.a", plancheck.ResourceActionNoop),
					},
				},
			},
		},
	})
}

// encParentAndChild builds an encrypted parent (owns its key) and a child that
// inherits. childExtra is extra HCL lines inside the child (e.g. an explicit
// encryption flag). Both are created in one config so the parent exists first.
func encParentAndChild(parent, childExtra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name                    = %q
  encryption              = true
  inherit_encryption      = false
  encryption_generate_key = true
}
resource "truenas_dataset" "child" {
  name               = "${truenas_dataset.parent.name}/child"
%s}
`, parent, childExtra)
}

// TestAccDataset_inheritEncryptionConflict guards GH #31: setting encryption
// together with inherit_encryption = true is contradictory (the parent
// determines encryption) and previously produced "inconsistent result after
// apply". It is now rejected at plan time with a clear message. The
// non-conflicting inherited path (inherit alone) is covered by
// TestAccDataset_inheritEncryptionRoundTrip.
func TestAccDataset_inheritEncryptionConflict(t *testing.T) {
	parent := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-encp"))
	cfg := encParentAndChild(parent, "  encryption         = false\n  inherit_encryption = true\n")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`encryption cannot be set when inherit_encryption`),
			},
		},
	})
}

// TestAccDataset_inheritEncryptionRoundTrip guards GH #32: inherit_encryption is
// reconciled from the API on read, so an inherited dataset carries the correct
// value in state even when the attribute is absent from configuration. Step 2
// drops inherit_encryption from the child's config and expects an empty plan —
// before the fix it planned a spurious in-place update.
func TestAccDataset_inheritEncryptionRoundTrip(t *testing.T) {
	parent := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-encr"))
	withInherit := encParentAndChild(parent, "  inherit_encryption = true\n")
	withoutInherit := encParentAndChild(parent, "")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(parent),
		Steps: []resource.TestStep{
			{
				Config: withInherit,
				Check:  resource.TestCheckResourceAttr("truenas_dataset.child", "inherit_encryption", "true"),
			},
			{
				Config: withoutInherit,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccDataset_compressionInherit guards GH #38 for datasets: compression =
// "inherit" round-trips (reported as inherit when not set locally, not the
// resolved algorithm).
func TestAccDataset_compressionInherit(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-dsinh"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "inh" {
  name        = %q
  compression = "inherit"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check:  resource.TestCheckResourceAttr("truenas_dataset.inh", "compression", "inherit"),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccDataset_syncInherit guards bucket-1 of the enum-conformance work: a
// ZFS enum (sync) set to the API's INHERIT member round-trips (reported as
// INHERIT when not set locally), rather than being rejected by the validator or
// read back as the resolved value.
func TestAccDataset_syncInherit(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-dssync"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "si" {
  name = %q
  sync = "INHERIT"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_dataset.si", "sync", "INHERIT")},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
