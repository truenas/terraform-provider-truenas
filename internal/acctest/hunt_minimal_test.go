// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: the FIRST config a newcomer writes — only the
// attributes the schema marks Required, nothing else. If the API needs more,
// the provider should default it or say so at plan time, not surface a raw
// `[EINVAL] ...: Field required` at apply (F2 was truenas_user). Each test
// applies the minimal config, re-plans (must be empty) and destroys.

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntMinimal(t *testing.T, cfg string) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

func TestHuntMinimal_group(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_group" "g" {
  name = %q
}
`, acctest.RandName("tfhuntmin")))
}

// F2: username + full_name + password is the smallest user the UI lets you
// make; the provider now defaults group_create the same way.
func TestHuntMinimal_user(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_user" "u" {
  username  = %q
  full_name = "minimal"
  password  = "TfHunt-Min-Pw-1"
}
`, acctest.RandName("tfhuntmin")))
}

// F2: with no password choice at all the user gets a plan-time error naming
// the two options, not the API's "Password is required" at apply.
func TestHuntMinimal_userNoGroupChoice(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_user" "u" {
  username  = %q
  full_name = "minimal"
}
`, acctest.RandName("tfhuntmin")),
			ExpectError: regexp.MustCompile(`needs a password or password_disabled = true`),
		}},
	})
}

func TestHuntMinimal_zvol(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 16777216
}
`, acctest.TestPool()+"/"+acctest.RandName("tfhuntmin")))
}

func TestHuntMinimal_snapshot(t *testing.T) {
	n := acctest.RandName("tfhuntmin")
	huntMinimal(t, huntDS(n)+`
resource "truenas_snapshot" "s" {
  dataset = truenas_dataset.fx.name
  name    = "min"
}
`)
}

func TestHuntMinimal_periodicSnapshot(t *testing.T) {
	n := acctest.RandName("tfhuntmin")
	huntMinimal(t, huntDS(n)+`
resource "truenas_periodic_snapshot_task" "p" {
  dataset        = truenas_dataset.fx.name
  lifetime_value = 1
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%Y-%m-%d_%H-%M"
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`)
}

func TestHuntMinimal_iscsiExtentZvol(t *testing.T) {
	n := acctest.RandName("tfhuntmin")
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 16777216
}
resource "truenas_iscsi_extent" "e" {
  name = %q
  type = "DISK"
  disk = "zvol/${truenas_zvol.z.name}"
}
`, acctest.TestPool()+"/"+n, n))
}

func TestHuntMinimal_iscsiPortal(t *testing.T) {
	huntMinimal(t, `
resource "truenas_iscsi_portal" "p" {
  listen = [{ ip = "0.0.0.0" }]
}
`)
}

func TestHuntMinimal_iscsiInitiator(t *testing.T) {
	huntMinimal(t, `
resource "truenas_iscsi_initiator" "i" {
}
`)
}

func TestHuntMinimal_apiKey(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_api_key" "k" {
  name     = %q
  username = "root"
}
`, acctest.RandName("tfhuntmin")))
}

func TestHuntMinimal_privilege(t *testing.T) {
	n := acctest.RandName("tfhuntmin")
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_group" "g" {
  name = %q
}
resource "truenas_privilege" "p" {
  name         = %q
  web_shell    = false
  local_groups = [truenas_group.g.gid]
  roles        = ["READONLY_ADMIN"]
}
`, n, n))
}

func TestHuntMinimal_keychainKeypair(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "k" {
  name     = %q
  generate = true
}
`, acctest.RandName("tfhuntmin")))
}

func TestHuntMinimal_alertService(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_alert_service" "a" {
  name  = %q
  level = "WARNING"
  attributes = jsonencode({ type = "Slack", url = "https://hooks.slack.invalid/min" })
}
`, acctest.RandName("tfhuntmin")))
}

func TestHuntMinimal_cloudsyncCredentials(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "c" {
  name = %q
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIAMIN"
    secret_access_key = "min-secret"
  })
}
`, acctest.RandName("tfhuntmin")))
}

func TestHuntMinimal_staticRoute(t *testing.T) {
	huntMinimal(t, `
resource "truenas_static_route" "r" {
  destination = "198.51.100.0/24"
  gateway     = "192.0.2.1"
}
`)
}

func TestHuntMinimal_tunable(t *testing.T) {
	huntMinimal(t, `
resource "truenas_tunable" "t" {
  type  = "SYSCTL"
  var   = "kernel.hung_task_timeout_secs"
  value = "122"
}
`)
}

func TestHuntMinimal_initShutdownScript(t *testing.T) {
	huntMinimal(t, `
resource "truenas_init_shutdown_script" "i" {
  type    = "COMMAND"
  command = "echo minimal"
  when    = "POSTINIT"
}
`)
}

// certificate is the GOOD pattern: a CSR without its per-create_type fields
// is rejected at plan time with a provider message, not a raw API error at
// apply. Nit: the message does not say which field is missing.
func TestHuntMinimal_certificateCSR(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_certificate" "c" {
  name        = %q
  create_type = "CERTIFICATE_CREATE_CSR"
  common      = "minimal.example.com"
}
`, acctest.RandName("tfhuntmin")),
			ExpectError: regexp.MustCompile(`Missing required field for create_type CERTIFICATE_CREATE_CSR`),
		}},
	})
}

func TestHuntMinimal_aclTemplate(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_acl_template" "a" {
  name    = %q
  acltype = "NFS4"
  acl = jsonencode([
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
  ])
}
`, acctest.RandName("tfhuntmin")))
}
