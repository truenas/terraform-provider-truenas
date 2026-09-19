// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/crypto/ssh"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccReplication_basic tests create, update, and import of a LOCAL push
// replication task. Source and target datasets are own-created fixtures; the
// task provides also_include_naming_schema because the API requires a
// periodic-task binding, a naming schema, or a name regex for push tasks.
func TestAccReplication_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-repl")
	srcDS := acctest.RandName("tf-acc-repl-src")
	dstDS := acctest.RandName("tf-acc-repl-dst")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccReplicationConfig(name, srcDS, dstDS, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "name", name),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "direction", "PUSH"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "transport", "LOCAL"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_replication_task.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccReplicationConfig(name, srcDS, dstDS, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "enabled", "false"),
				),
			},
			{
				ResourceName:      "truenas_replication_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccReplicationConfig(name, srcDS, dstDS string, enabled bool) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = "%[1]s/%[2]s"
}

resource "truenas_dataset" "dst" {
  name = "%[1]s/%[3]s"
}

resource "truenas_replication_task" "test" {
  name             = %[4]q
  direction        = "PUSH"
  transport        = "LOCAL"
  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"
  enabled          = %[5]v

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
}
`, pool, srcDS, dstDS, name, enabled)
}

// testAccCheckReplicationDestroyed verifies the task is gone from the API.
func testAccCheckReplicationDestroyed(name string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "replication.query",
			[][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("querying replication tasks: %w", err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("decoding replication.query: %w", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("replication task %q still exists (id %d)", name, results[0].ID)
		}
		return nil
	}
}

// genEd25519OpenSSHKeyPair generates an ed25519 key pair and returns the
// OpenSSH-format PEM private key plus the exact authorized_keys-format
// public key line — identical in behavior to the like-named helper in
// keychain_ssh_connection's and keychain_ssh_keypair's acceptance tests,
// kept package-local here too.
func genEd25519OpenSSHKeyPair(t *testing.T, comment string) (privateKeyPEM, publicKeyLine string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ed25519 key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		t.Fatalf("marshaling OpenSSH private key: %v", err)
	}
	privateKeyPEM = string(pem.EncodeToMemory(block))

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("deriving ssh public key: %v", err)
	}
	publicKeyLine = string(ssh.MarshalAuthorizedKey(sshPub))

	return privateKeyPEM, publicKeyLine
}

// scanRemoteHostKey calls keychaincredential.remote_ssh_host_key_scan
// against the acceptance-test TrueNAS box's own SSH server (a genuine
// loopback), returning the multi-line host key blob "remote_host_key"
// expects verbatim — see keychain_ssh_connection's acceptance test for the
// identical, identically-documented helper.
func scanRemoteHostKey(t *testing.T, host string) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "keychaincredential.remote_ssh_host_key_scan", map[string]any{
		"host": host,
		"port": 22,
	})
	if err != nil {
		t.Fatalf("remote_ssh_host_key_scan against %s failed: %v", host, err)
	}
	var hostKey string
	if err := json.Unmarshal(raw, &hostKey); err != nil {
		t.Fatalf("parsing remote_ssh_host_key_scan response: %v", err)
	}
	return hostKey
}

// takeMatchingSnapshot creates a snapshot on dataset whose name matches the
// "auto-%Y-%m-%d_%H-%M" naming schema testAccReplicationSSHConfig's
// also_include_naming_schema uses, via a direct pool.snapshot.create call —
// there is no Terraform-managed way to produce a timestamp-based name
// without a diff-producing timestamp() in the config (which would make the
// resource non-stable across the test's multiple apply steps), so this
// mirrors CheckDestroy's established pattern of driving the API directly
// from a resource.TestCheckFunc for test-fixture setup.
func takeMatchingSnapshot(t *testing.T, dataset string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		suffix := time.Now().Format("2006-01-02_15-04")
		raw, err := acctest.Client().Call(context.Background(), "pool.snapshot.create", map[string]any{
			"dataset": dataset,
			"name":    "auto-" + suffix,
		})
		if err != nil {
			return fmt.Errorf("creating source snapshot on %s: %w", dataset, err)
		}
		var snap struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			return fmt.Errorf("parsing pool.snapshot.create response: %w", err)
		}
		t.Logf("created source snapshot %s", snap.ID)
		return nil
	}
}

// runReplicationAndVerify calls replication.run(id) as a synchronous job and
// asserts it completes without error. Verified live (see task-6-report.md):
// a real SSH push replication of a single small snapshot completes in
// ~14s on first run (deterministic error otherwise: destination-unmount
// permission denial when sudo=false, since the SSH user isn't root — fixed
// by this fixture's sudo=true) and ~2s on a repeat no-op run — fast and
// deterministic enough to verify the actual data-transfer round trip here,
// rather than falling back to a state-only check.
func runReplicationAndVerify(t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		idStr, ok := rs.Primary.Attributes["id"]
		if !ok {
			return fmt.Errorf("%s has no id attribute", resourceName)
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing id %q: %w", idStr, err)
		}

		start := time.Now()
		_, err = acctest.Client().CallJob(context.Background(), "replication.run", id)
		elapsed := time.Since(start)
		if err != nil {
			return fmt.Errorf("replication.run(%d) failed after %s: %w", id, elapsed, err)
		}
		t.Logf("replication.run(%d) succeeded in %s", id, elapsed)
		return nil
	}
}

// TestAccReplication_RemoteSSH exercises transport = "SSH" end to end: an
// ed25519 truenas_keychain_ssh_keypair + truenas_keychain_ssh_connection
// fixture pair (HCL-referenced, per the loopback pattern established by
// keychain_ssh_connection's own acceptance test), a throwaway
// truenas_user fixture (also HCL-referenced) whose sshpubkey and
// sudo_commands_nopasswd are set directly in config rather than mutating
// any pre-existing box account, and a PUSH replication task over that
// connection — full contract (create with compression/speed_limit/sudo
// set, in-place update, import), plus a genuine replication.run round
// trip against a real snapshot (see runReplicationAndVerify's doc comment
// for why a full run was chosen over a state-only check).
//
// This used to authorize the test key on the box's own "truenas_admin"
// user and rely on that account's sudo config already being passwordless.
// That assumption held on the 25.10 disposable VM but not on the 26.0
// box (real Task 10 sweep failure: replication.run's remote zfs receive
// hit "sudo: a password is required" because truenas_admin there has
// sudo_commands=["ALL"] but an empty sudo_commands_nopasswd — a
// per-box account configuration difference, not a TrueNAS release
// difference or a provider defect). Provisioning a disposable
// Terraform-managed user with sudo_commands_nopasswd = ["ALL"] makes the
// test self-sufficient and portable across boxes without ever touching a
// pre-existing account's sudo policy — especially important on the 26.0
// box, which is production-serving and must never have its admin
// account's privileges altered by a test run.
func TestAccReplication_RemoteSSH(t *testing.T) {
	acctest.PreCheck(t)

	host := acctest.EndpointHost()
	username := acctest.RandName("tf-acc-replssh-user")
	homeDS := acctest.RandName("tf-acc-replssh-home")
	keypairName := acctest.RandName("tf-acc-replssh-keypair")
	connName := acctest.RandName("tf-acc-replssh-conn")
	privateKeyPEM, publicKeyLine := genEd25519OpenSSHKeyPair(t, keypairName)
	// ssh.MarshalAuthorizedKey appends a trailing newline, but TrueNAS's
	// user.create/user.update strip it before persisting sshpubkey, so the
	// server's post-create read-back never carries it. Terraform's plan
	// step otherwise uses the config value (including the newline) as the
	// planned value for this Optional+Computed attribute, which then
	// mismatches the applied state and trips "provider produced
	// inconsistent result after apply" (probed live). Trimming here,
	// test-side, keeps the config's value identical to what the box
	// stores. Provider-side note for final review: truenas_user.sshpubkey
	// has no whitespace normalization of its own, so any real config
	// supplying a trailing newline hits this same error — a candidate for
	// a provider-side fix (e.g. a plan modifier trimming the configured
	// value), not implemented here since a test-side trim fully unblocks
	// this test.
	publicKeyLine = strings.TrimSpace(publicKeyLine)
	hostKey := scanRemoteHostKey(t, host)

	name := acctest.RandName("tf-acc-repl-ssh")
	srcDS := acctest.RandName("tf-acc-repl-ssh-src")
	dstDS := acctest.RandName("tf-acc-repl-ssh-dst")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationSSHDestroyed(name, username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccReplicationSSHConfig(
					username, homeDS, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, srcDS, dstDS, name, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_replication_task.test", "id"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "name", name),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "direction", "PUSH"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "transport", "SSH"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "sudo", "true"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "compression", "LZ4"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "speed_limit", "1048576"),
					resource.TestCheckResourceAttrPair(
						"truenas_replication_task.test", "ssh_credentials",
						"truenas_keychain_ssh_connection.test", "id"),
					resource.TestCheckResourceAttr("truenas_user.sshuser", "username", username),
					// Real data-transfer round trip: take a snapshot matching
					// the task's also_include_naming_schema, then run it.
					takeMatchingSnapshot(t, fmt.Sprintf("%s/%s", acctest.TestPool(), srcDS)),
					runReplicationAndVerify(t, "truenas_replication_task.test"),
				),
			},
			// In-place update: rename. transport carries RequiresReplace (see
			// schema.go) so this step deliberately leaves it (and every other
			// SSH-only attribute) unchanged, matching TestAccReplication_basic's
			// enabled-toggle-only update step.
			{
				Config: acctest.ProviderConfig() + testAccReplicationSSHConfig(
					username, homeDS, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, srcDS, dstDS, name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "enabled", "false"),
				),
			},
			{
				ResourceName:      "truenas_replication_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccReplication_SSHNetcat exercises transport = "SSH+NETCAT" and the
// netcat_* attributes. It reuses the same SSH keypair/connection/user
// scaffolding as TestAccReplication_RemoteSSH (SSH+NETCAT authenticates over
// SSH exactly like SSH does) but sets transport = "SSH+NETCAT" plus the
// netcat active/passive fields. It verifies the create/read/import contract
// only — not a live replication.run: the netcat data path over loopback is
// not a reliable round trip in this single-box fixture, whereas the create +
// read-back proves the provider maps the new attributes to and from the API.
func TestAccReplication_SSHNetcat(t *testing.T) {
	acctest.PreCheck(t)

	host := acctest.EndpointHost()
	username := acctest.RandName("tf-acc-replnc-user")
	homeDS := acctest.RandName("tf-acc-replnc-home")
	keypairName := acctest.RandName("tf-acc-replnc-keypair")
	connName := acctest.RandName("tf-acc-replnc-conn")
	privateKeyPEM, publicKeyLine := genEd25519OpenSSHKeyPair(t, keypairName)
	publicKeyLine = strings.TrimSpace(publicKeyLine)
	hostKey := scanRemoteHostKey(t, host)

	name := acctest.RandName("tf-acc-repl-nc")
	srcDS := acctest.RandName("tf-acc-repl-nc-src")
	dstDS := acctest.RandName("tf-acc-repl-nc-dst")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationSSHDestroyed(name, username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccReplicationNetcatConfig(
					username, homeDS, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, srcDS, dstDS, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_replication_task.test", "id"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "transport", "SSH+NETCAT"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "netcat_active_side", "LOCAL"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "netcat_active_side_listen_address", "127.0.0.1"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "netcat_active_side_port_min", "20000"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "netcat_active_side_port_max", "20100"),
					resource.TestCheckResourceAttr("truenas_replication_task.test", "netcat_passive_side_connect_address", "127.0.0.1"),
					resource.TestCheckResourceAttrPair(
						"truenas_replication_task.test", "ssh_credentials",
						"truenas_keychain_ssh_connection.test", "id"),
				),
			},
			{
				ResourceName:      "truenas_replication_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccReplicationNetcatConfig(username, homeDS, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, srcDS, dstDS, name string) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = %[1]q
}

resource "truenas_dataset" "dst" {
  name = %[2]q
}

resource "truenas_dataset" "sshuser_home" {
  name = %[3]q
}

resource "truenas_user" "sshuser" {
  username               = %[4]q
  full_name              = "TF Acceptance NETCAT Replication User"
  password_disabled      = true
  home                   = "/mnt/${truenas_dataset.sshuser_home.name}"
  shell                  = "/usr/bin/bash"
  group_create           = true
  sshpubkey              = %[5]q
  sudo_commands_nopasswd = ["ALL"]
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = %[6]q
  private_key = %[7]q
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = %[8]q
  host            = %[9]q
  port            = 22
  username        = truenas_user.sshuser.username
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = %[10]q
  connect_timeout = 10
}

resource "truenas_replication_task" "test" {
  name             = %[11]q
  direction        = "PUSH"
  transport        = "SSH+NETCAT"
  ssh_credentials  = truenas_keychain_ssh_connection.test.id
  sudo             = true

  netcat_active_side                  = "LOCAL"
  netcat_active_side_listen_address   = "127.0.0.1"
  netcat_active_side_port_min         = 20000
  netcat_active_side_port_max         = 20100
  netcat_passive_side_connect_address = "127.0.0.1"

  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"
  readonly         = "IGNORE"

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
}
`, pool+"/"+srcDS, pool+"/"+dstDS, pool+"/"+homeDS, username, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, name)
}

func testAccReplicationSSHConfig(username, homeDS, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, srcDS, dstDS, name string, enabled bool) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = %[1]q
}

resource "truenas_dataset" "dst" {
  name = %[2]q
}

# user_create requires the home directory of any user with an sshpubkey to
# be a writable path inside a data pool (a stub like /var/empty is
# rejected), so the throwaway SSH user below gets its own dedicated,
# Terraform-managed dataset as a home — destroyed along with the user
# fixture, never touching any pre-existing pool path.
resource "truenas_dataset" "sshuser_home" {
  name = %[3]q
}

resource "truenas_user" "sshuser" {
  username               = %[4]q
  full_name              = "TF Acceptance SSH Replication User"
  password_disabled      = true
  home                   = "/mnt/${truenas_dataset.sshuser_home.name}"
  shell                  = "/usr/bin/bash"
  group_create           = true
  sshpubkey              = %[5]q
  sudo_commands_nopasswd = ["ALL"]
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = %[6]q
  private_key = %[7]q
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = %[8]q
  host            = %[9]q
  port            = 22
  username        = truenas_user.sshuser.username
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = %[10]q
  connect_timeout = 10
}

resource "truenas_replication_task" "test" {
  name             = %[11]q
  direction        = "PUSH"
  transport        = "SSH"
  ssh_credentials  = truenas_keychain_ssh_connection.test.id
  sudo             = true
  compression      = "LZ4"
  speed_limit      = 1048576
  source_datasets  = [truenas_dataset.src.name]
  target_dataset   = truenas_dataset.dst.name
  recursive        = false
  auto             = false
  retention_policy = "SOURCE"
  readonly         = "IGNORE"
  enabled          = %[12]v

  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
}
`, pool+"/"+srcDS, pool+"/"+dstDS, pool+"/"+homeDS, username, publicKeyLine, keypairName, privateKeyPEM, connName, host, hostKey, name, enabled)
}

// testAccCheckReplicationSSHDestroyed verifies the replication task, the
// keychain connection, the keychain keypair, and the throwaway SSH user
// fixture are all gone, matching keychain_ssh_connection's acceptance
// test's multi-resource CheckDestroy pattern.
func testAccCheckReplicationSSHDestroyed(name, username string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()

		raw, err := c.Call(context.Background(), "replication.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("querying replication tasks: %w", err)
		}
		var replResults []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &replResults); err != nil {
			return fmt.Errorf("decoding replication.query: %w", err)
		}
		if len(replResults) > 0 {
			return fmt.Errorf("replication task %q still exists (id %d)", name, replResults[0].ID)
		}

		for _, addr := range []string{"truenas_keychain_ssh_connection.test", "truenas_keychain_ssh_keypair.test"} {
			rs, ok := s.RootModule().Resources[addr]
			if !ok {
				return fmt.Errorf("resource %s not found in pre-destroy state", addr)
			}
			idStr, ok := rs.Primary.Attributes["id"]
			if !ok {
				return fmt.Errorf("%s has no id attribute in pre-destroy state", addr)
			}
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing id %q for %s: %v", idStr, addr, err)
			}

			raw, err := c.Call(context.Background(), "keychaincredential.query", [][]any{{"id", "=", id}})
			if err != nil {
				return fmt.Errorf("error checking keychain credential id=%d (%s): %v", id, addr, err)
			}
			var credResults []struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(raw, &credResults); err != nil {
				return fmt.Errorf("error parsing keychaincredential.query response: %v", err)
			}
			if len(credResults) > 0 {
				return fmt.Errorf("keychain credential id=%d (%s) still exists", id, addr)
			}
		}

		userRaw, err := c.Call(context.Background(), "user.query", [][]any{{"username", "=", username}})
		if err != nil {
			return fmt.Errorf("querying user %q: %w", username, err)
		}
		var userResults []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(userRaw, &userResults); err != nil {
			return fmt.Errorf("decoding user.query: %w", err)
		}
		if len(userResults) > 0 {
			return fmt.Errorf("throwaway SSH user %q still exists (id %d)", username, userResults[0].ID)
		}
		return nil
	}
}
