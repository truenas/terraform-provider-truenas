// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Minimal (Required-only) configs, batch 2: NVMe-oF, iSCSI auth/mapping,
// kerberos realm, cloudsync task, VM. Same question as batch 1 — does the
// first config a user writes apply, or fail with a raw API message?

import (
	"fmt"
	"strings"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntMinimal_nvmetSubsys(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_nvmet_subsys" "s" {
  name = %q
}
`, acctest.RandName("tfhuntmin")))
}

func TestHuntMinimal_nvmetHost(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_nvmet_host" "h" {
  hostnqn = %q
}
`, acctest.RandNQN()))
}

func TestHuntMinimal_nvmetPort(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_nvmet_port" "p" {
  addr_trtype  = "TCP"
  addr_traddr  = %q
  addr_trsvcid = 4421
}
`, acctest.EndpointHost()))
}

func TestHuntMinimal_iscsiAuth(t *testing.T) {
	huntMinimal(t, `
resource "truenas_iscsi_auth" "a" {
  tag    = 9901
  user   = "tfhuntmin"
  secret = "tfhuntmin-sec1" # API: 12–16 chars
}
`)
}

func TestHuntMinimal_iscsiTargetExtent(t *testing.T) {
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
resource "truenas_iscsi_target" "t" {
  name = %q
}
resource "truenas_iscsi_targetextent" "te" {
  target = truenas_iscsi_target.t.id
  extent = truenas_iscsi_extent.e.id
}
`, acctest.TestPool()+"/"+n, n, n))
}

func TestHuntMinimal_kerberosRealm(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_kerberos_realm" "r" {
  realm = %q
}
`, "TFHUNTMIN"+acctest.RandName("")+".LAN"))
}

func TestHuntMinimal_cloudsyncTask(t *testing.T) {
	n := acctest.RandName("tfhuntmin")
	huntMinimal(t, huntDS(n)+fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "c" {
  name = %q
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIAMIN"
    secret_access_key = "min-secret"
    endpoint          = "http://192.0.2.9:9000"
    region            = "us-east-1"
  })
}
resource "truenas_cloudsync_task" "t" {
  description   = %q
  path          = truenas_dataset.fx.mountpoint
  credentials   = truenas_cloudsync_credentials.c.id
  direction     = "PUSH"
  transfer_mode = "COPY"
  attributes    = jsonencode({ bucket = "b", folder = "x" })
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, n, n))
}

func TestHuntMinimal_vm(t *testing.T) {
	huntMinimal(t, fmt.Sprintf(`
resource "truenas_vm" "v" {
  name   = %q
  memory = 512
}
`, "tfhuntmin"+strings.Map(func(r rune) rune { // vm names: alphanumeric only
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, acctest.RandName(""))))
}
