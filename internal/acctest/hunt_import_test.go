// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt (group "api"), NOT for commit.
//
// What ImportReapplyNoop was meant to do, done correctly: create, import with
// ImportStatePersist: true (so the imported state REPLACES the created state),
// then plan the identical config and require an empty plan. One test per
// attribute that the repo hides behind ImportStateVerifyIgnore.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntImportPersist(t *testing.T, addr, cfg string, _ resource.ImportStateIdFunc) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	// Same config minus the target resource block, plus a removed{} block so
	// the object leaves state without being destroyed.
	hdr := "resource \"" + strings.SplitN(addr, ".", 2)[0] + "\" \"" + strings.SplitN(addr, ".", 2)[1] + "\" {"
	i := strings.Index(full, hdr)
	j := i + strings.Index(full[i:], "\n}\n") + 3
	without := full[:i] + full[j:] + fmt.Sprintf("\nremoved {\n  from = %s\n  lifecycle {\n    destroy = false\n  }\n}\n", addr)
	var id string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full, Check: func(s *terraform.State) error {
				id = s.RootModule().Resources[addr].Primary.ID
				return nil
			}},
			{Config: without},
			{
				Config:             full,
				ResourceName:       addr,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  func(*terraform.State) (string, error) { return id, nil },
			},
			{
				Config: full,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{logPlanDiff{t}, plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// huntImportC1 runs the reworked acctest.ImportReapplyNoop (fix C1/T-001):
// import must not replace, only the listed create-only inputs may be recorded
// in place, and the plan after that must be empty.
func huntImportC1(t *testing.T, addr, cfg string, allowed ...string) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    append([]resource.TestStep{{Config: full}}, acctest.ImportReapplyNoop(addr, full, allowed...)...),
	})
}

func huntDS(n string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fx" {
  name = %q
}
`, acctest.TestPool()+"/"+n)
}

// certificate.create_type + ec_curve (Required/RequiresReplace, import-ignored)
func TestHuntImport_certificateCSR_EC(t *testing.T) {
	n := acctest.RandName("tfhuntapicert")
	huntImportC1(t, "truenas_certificate.c", fmt.Sprintf(`
resource "truenas_certificate" "c" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_CSR"
  key_type    = "EC"
  ec_curve    = "SECP384R1"
  common      = "tfhunt-csr.example.com"
  san         = ["tfhunt-csr.example.com"]
}
`, n), "create_type", "ec_curve")
}

// certificate.digest_algorithm on an RSA CSR: today's hunt test hid it behind
// ImportStateVerifyIgnore because import did not return it. If the provider
// does not read it back, import → plan will want to re-set it (a diff), or
// worse, replace. Candidate bug, not normalization.
// F1: digest_algorithm must round-trip through import (derived from the CSR's
// signature algorithm). create_type is the one create-only input the API
// cannot return for a CSR; it is recorded in place, as for the EC case above.
func TestHuntImport_certificateCSR_RSA_digest(t *testing.T) {
	n := acctest.RandName("tfhuntapicertd")
	huntImportC1(t, "truenas_certificate.c", fmt.Sprintf(`
resource "truenas_certificate" "c" {
  name             = %q
  create_type      = "CERTIFICATE_CREATE_CSR"
  key_type         = "RSA"
  key_length       = 2048
  digest_algorithm = "SHA512"
  common           = "tfhunt-csr-rsa.example.com"
  san              = ["tfhunt-csr-rsa.example.com"]
}
`, n), "create_type")
}

// cloudsync.attributes: the server returns the rclone attributes expanded
// (region/storage_class/fast_list/encryption). Today's hunt test ignored the
// field on import; this asks the real question — does import → plan stay empty?
func TestHuntImport_cloudsyncAttributes(t *testing.T) {
	n := acctest.RandName("tfhuntapics")
	huntImportPersist(t, "truenas_cloudsync_task.t", huntDS(n)+fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "c" {
  name = %q
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIAHUNT"
    secret_access_key = "hunt-secret"
    endpoint          = "http://192.0.2.9:9000"
    region            = "us-east-1"
    skip_region       = false
    signatures_v2     = false
  })
}
resource "truenas_cloudsync_task" "t" {
  description   = %q
  path          = truenas_dataset.fx.mountpoint
  credentials   = truenas_cloudsync_credentials.c.id
  direction     = "PUSH"
  transfer_mode = "COPY"
  enabled       = false
  attributes    = jsonencode({ bucket = "b", folder = "x" })
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, n, n), nil)
}

// user.groups: on 26.0+ import listed an extra gid alongside the configured
// one (the primary group?). Today's hunt test ignored "groups". If import
// then plans a change (dropping that gid), that is a real diff users see.
func TestHuntImport_userGroupsExtra(t *testing.T) {
	n := acctest.RandName("tfhuntapiug")
	huntImportPersist(t, "truenas_user.u", fmt.Sprintf(`
resource "truenas_group" "g1" {
  name = %q
}
resource "truenas_group" "g2" {
  name = %q
}
resource "truenas_user" "u" {
  username          = %q
  full_name         = "tfhunt groups import"
  password_disabled = true
  home              = "/var/empty"
  smb               = false
  group             = truenas_group.g1.id
  groups            = [truenas_group.g2.id]
}
`, n+"a", n+"b", n), nil)
}

// dataset.force_destroy / zvol.force_destroy (F6): a provider-only flag an
// import cannot know, so import records false and a config with true plans
// one state-only update (the box is not touched); the plan after it must be
// empty. Terraform offers no way to take a value from config during import,
// so this one-shot diff is the documented behaviour, not a loop.
func huntImportPersistOneShot(t *testing.T, addr, cfg string) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	hdr := "resource \"" + strings.SplitN(addr, ".", 2)[0] + "\" \"" + strings.SplitN(addr, ".", 2)[1] + "\" {"
	i := strings.Index(full, hdr)
	j := i + strings.Index(full[i:], "\n}\n") + 3
	without := full[:i] + full[j:] + fmt.Sprintf("\nremoved {\n  from = %s\n  lifecycle {\n    destroy = false\n  }\n}\n", addr)
	var id string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full, Check: func(s *terraform.State) error {
				id = s.RootModule().Resources[addr].Primary.ID
				return nil
			}},
			{Config: without},
			{Config: full, ResourceName: addr, ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				logPlanDiff{t}, plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)}}},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

func TestHuntImport_datasetForceDestroy(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntapifd")
	huntImportPersistOneShot(t, "truenas_dataset.d", fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name          = %q
  force_destroy = true
}
`, n))
}

func TestHuntImport_zvolForceDestroy(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntapizfd")
	huntImportPersistOneShot(t, "truenas_zvol.z", fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name          = %q
  volsize       = 16777216
  sparse        = true
  force_destroy = true
}
`, n))
}

// keychain_ssh_keypair.generate (RequiresReplace, import-ignored)
func TestHuntImport_keypairGenerate(t *testing.T) {
	n := acctest.RandName("tfhuntapikp")
	huntImportC1(t, "truenas_keychain_ssh_keypair.k", fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "k" {
  name     = %q
  generate = true
}
`, n), "generate")
}

// snapshot.recursive (RequiresReplace, import-ignored)
func TestHuntImport_snapshotRecursive(t *testing.T) {
	n := acctest.RandName("tfhuntapisnap")
	huntImportC1(t, "truenas_snapshot.s", huntDS(n)+`
resource "truenas_snapshot" "s" {
  dataset   = truenas_dataset.fx.name
  name      = "tfhuntsnap"
  recursive = true
}
`, "recursive")
}

// filesystem_permissions.recursive/traverse (Optional, import-ignored)
func TestHuntImport_permissionsRecursive(t *testing.T) {
	n := acctest.RandName("tfhuntapiperm")
	huntImportPersist(t, "truenas_filesystem_permissions.p", huntDS(n)+`
resource "truenas_filesystem_permissions" "p" {
  path      = truenas_dataset.fx.mountpoint
  mode      = "0755"
  uid       = 0
  gid       = 0
  recursive = true
  traverse  = true
}
`, nil)
}

// ntp_server.force (Optional, import-ignored)
func TestHuntImport_ntpForce(t *testing.T) {
	huntImportPersist(t, "truenas_ntp_server.n", `
resource "truenas_ntp_server" "n" {
  address = "127.127.1.2"
  iburst  = false
  burst   = false
  prefer  = false
  minpoll = 5
  maxpoll = 8
  force   = true
}
`, nil)
}

// zvol.sparse (Optional, import-ignored)
func TestHuntImport_zvolSparse(t *testing.T) {
	n := acctest.RandName("tfhuntapizv")
	huntImportPersist(t, "truenas_zvol.z", fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 16777216
  sparse  = true
}
`, acctest.TestPool()+"/"+n), nil)
}

// user.group_create + groups (import-ignored)
func TestHuntImport_userGroupCreate(t *testing.T) {
	n := acctest.RandName("tfhuntapiu")
	huntImportPersist(t, "truenas_user.u", fmt.Sprintf(`
resource "truenas_group" "g" {
  name = %q
}
resource "truenas_user" "u" {
  username          = %q
  full_name         = "tfhunt import"
  password_disabled = true
  home              = "/var/empty"
  smb               = false
  group_create      = true
  groups            = [truenas_group.g.id]
}
`, n+"g", n), nil)
}

// alert_service.attributes (import-ignored "re-normalized")
func TestHuntImport_alertServiceAttributes(t *testing.T) {
	n := acctest.RandName("tfhuntapialert")
	huntImportPersist(t, "truenas_alert_service.a", fmt.Sprintf(`
resource "truenas_alert_service" "a" {
  name    = %q
  level   = "WARNING"
  enabled = false
  attributes = jsonencode({
    type  = "Mail"
    email = "tfhunt@example.com"
  })
}
`, n), nil)
}

// acme_dns_authenticator.attributes (import-ignored "re-normalized")
func TestHuntImport_acmeAttributes(t *testing.T) {
	n := acctest.RandName("tfhuntapiacme")
	huntImportPersist(t, "truenas_acme_dns_authenticator.a", fmt.Sprintf(`
resource "truenas_acme_dns_authenticator" "a" {
  name = %q
  attributes = jsonencode({
    authenticator = "cloudflare"
    api_token     = "tfhunt-token"
  })
}
`, n), nil)
}

// nfs_share.networks (import-ignored "normalized server-side")
func TestHuntImport_nfsNetworks(t *testing.T) {
	n := acctest.RandName("tfhuntapinfs")
	huntImportPersist(t, "truenas_nfs_share.s", huntDS(n)+`
resource "truenas_nfs_share" "s" {
  path     = truenas_dataset.fx.mountpoint
  networks = ["10.77.3.4/16", "10.66.0.0/16"]
  enabled  = false
}
`, nil)
}

// filesystem_permissions.mode: the validator accepts 3 octal digits ("755") but
// Read stores the 4-digit form ("0755") -> inconsistent result after apply.
func TestHuntPermissionsMode3Digit(t *testing.T) {
	acctest.PreCheck(t)
	n := acctest.RandName("tfhuntapimode")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + huntDS(n) + `
resource "truenas_filesystem_permissions" "p" {
  path = truenas_dataset.fx.mountpoint
  mode = "755"
}
`,
		}},
	})
}
