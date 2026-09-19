// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair_test

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

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/crypto/ssh"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// genEd25519OpenSSHKeyPair generates an ed25519 key pair and returns the
// OpenSSH-format PEM private key (via golang.org/x/crypto/ssh's
// MarshalPrivateKey — already a transitive module dependency, per go.mod,
// so this adds no new dependency) plus the exact "authorized_keys"-format
// public key line TrueNAS is expected to derive from it.
//
// TrueNAS embeds the comment from the supplied private key's OpenSSH
// structure into the public key it derives (probed live: a key marshaled
// with comment "tf-probe" round-tripped through keychaincredential.create
// with only "private_key" set came back with "public_key" ending in that
// same comment) — ssh.MarshalAuthorizedKey itself never adds a comment, so
// it is appended here to build the exact expected value, including the
// trailing newline both TrueNAS's response and MarshalAuthorizedKey always
// include.
func genEd25519OpenSSHKeyPair(t *testing.T, comment string) (privateKeyPEM, expectedPublicKey string) {
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
	line := strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(sshPub)), "\n")
	expectedPublicKey = line + " " + comment + "\n"

	return privateKeyPEM, expectedPublicKey
}

// TestAccKeychainSSHKeyPair_generated exercises the generate=true path:
// TrueNAS generates the key pair server-side (via
// keychaincredential.generate_ssh_key_pair), rename in place, import by
// numeric id ("generate" is unknowable on import — see schema.go), destroy
// verified via a live keychaincredential.query.
func TestAccKeychainSSHKeyPair_generated(t *testing.T) {
	name := acctest.RandName("tf-acc-keypair-gen")
	renamed := name + "-renamed"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainSSHKeyPairDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHKeyPairGeneratedConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_keypair.test", "id"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "name", name),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "generate", "true"),
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_keypair.test", "private_key"),
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_keypair.test", "public_key"),
				),
			},
			// Rename in place — must not require replacement.
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHKeyPairGeneratedConfig(renamed),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "name", renamed),
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_keypair.test", "private_key"),
				),
			},
			{
				ResourceName:      "truenas_keychain_ssh_keypair.test",
				ImportState:       true,
				ImportStateVerify: true,
				// "generate" has no wire counterpart and cannot be recovered
				// on import (see schema.go / ImportState's doc comments).
				ImportStateVerifyIgnore: []string{"generate"},
			},
		},
	})
}

func testAccKeychainSSHKeyPairGeneratedConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "test" {
  name     = %q
  generate = true
}
`, name)
}

// TestAccKeychainSSHKeyPair_supplied exercises the user-supplied-key path:
// an ed25519 key pair generated in-test (Go stdlib crypto/ed25519 +
// golang.org/x/crypto/ssh's OpenSSH marshaling — see
// genEd25519OpenSSHKeyPair), verifying both that "private_key" round-trips
// byte-for-byte (it is Sensitive, not WriteOnly — probed live, see
// model.go) and that TrueNAS derives "public_key" automatically when it is
// omitted from the create payload.
func TestAccKeychainSSHKeyPair_supplied(t *testing.T) {
	name := acctest.RandName("tf-acc-keypair-supplied")
	renamed := name + "-renamed"
	privateKeyPEM, expectedPublicKey := genEd25519OpenSSHKeyPair(t, name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainSSHKeyPairDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHKeyPairSuppliedConfig(name, privateKeyPEM),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_keypair.test", "id"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "name", name),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "generate", "false"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "private_key", privateKeyPEM),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "public_key", expectedPublicKey),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHKeyPairSuppliedConfig(renamed, privateKeyPEM),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "name", renamed),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_keypair.test", "private_key", privateKeyPEM),
				),
			},
			{
				ResourceName:            "truenas_keychain_ssh_keypair.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"generate"},
			},
		},
	})
}

func testAccKeychainSSHKeyPairSuppliedConfig(name, privateKeyPEM string) string {
	return fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "test" {
  name        = %q
  private_key = %q
}
`, name, privateKeyPEM)
}

// testAccCheckKeychainSSHKeyPairDestroyed queries keychaincredential by the
// id captured in the pre-destroy state, matching the established pattern
// used elsewhere in this provider (e.g. truenas_kerberos_keytab's
// acceptance test).
func testAccCheckKeychainSSHKeyPairDestroyed(s *terraform.State) error {
	rs, ok := s.RootModule().Resources["truenas_keychain_ssh_keypair.test"]
	if !ok {
		return fmt.Errorf("resource truenas_keychain_ssh_keypair.test not found in pre-destroy state")
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("truenas_keychain_ssh_keypair.test has no id attribute in pre-destroy state")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing keychain credential id %q: %v", idStr, err)
	}

	c := acctest.Client()
	raw, err := c.Call(context.Background(), "keychaincredential.query", [][]any{{"id", "=", id}})
	if err != nil {
		return fmt.Errorf("error checking keychain credential id=%d: %v", id, err)
	}
	var results []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing keychaincredential.query response: %v", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("keychain credential id=%d still exists", id)
	}
	return nil
}
