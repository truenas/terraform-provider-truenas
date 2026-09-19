// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// dsPreCheckStatusRequiresDisabled fatals unless directoryservices.status
// reports the service as not currently joined/enabled before a directory
// services acceptance test starts mutating box state. Confirmed live on the
// TrueNAS 25.10 disposable test VM: a box that has never been configured, or
// one that has since been disabled after a prior join, reports
// {"status": null, "status_msg": null, "type": null} — the datasource's own
// "status" attribute description documents this same "DISABLED, ...; null
// if directory services are disabled" duality. Either shape is accepted
// here as "disabled"; anything else (HEALTHY/JOINING/FAULTED/LEAVING) means
// a previous test run in this package left the box mid-join, and that must
// not be silently built on top of.
func dsPreCheckStatusRequiresDisabled(t *testing.T) {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "directoryservices.status")
	if err != nil {
		t.Fatalf("error reading directoryservices.status before test start: %v", err)
	}
	var st struct {
		Status *string `json:"status"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("error parsing directoryservices.status response: %v", err)
	}
	if st.Status != nil && *st.Status != "DISABLED" {
		t.Fatalf("directoryservices.status = %q, want DISABLED (or null) before starting this test — the box "+
			"must not have an active directory service join in progress; refusing to run on top of it", *st.Status)
	}
}

// dsPreCheck runs acctest.DSCheck (which itself skips unless TRUENAS_DS=1
// and enforces the disposable-VM TRUENAS_DS_ALLOWED_ENDPOINT guard), then
// requires the domain/user/password env vars a live Active Directory join
// needs.
//
// It does NOT verify DNS is pointed at the domain controller server-side:
// probing core.get_methods for a dnsclient/dns forward-lookup-or-SRV method
// (TrueNAS 25.10, live) found only dns.query, which lists locally configured
// resolvers rather than performing a resolver-side SRV lookup — there is no
// middleware method to drive an on-box "_ldap._tcp.<domain>" SRV check.
// Per the task-8 brief's documented fallback, this test instead lets the
// join step's middleware error speak: if the box's resolver isn't pointed
// at the domain's DC, directoryservices.update fails with a clear DNS/DC
// error rather than silently hanging. Pointing the box's nameserver at the
// DC (and restoring it afterward) is an operational precondition for this
// test run, not something the test itself can safely automate — see
// task-8-report.md for the exact commands used against the disposable VM.
func dsPreCheck(t *testing.T) (domain, user, password string) {
	t.Helper()
	acctest.DSCheck(t)
	dsPreCheckStatusRequiresDisabled(t)
	domain = os.Getenv("TRUENAS_DS_DOMAIN")
	user = os.Getenv("TRUENAS_DS_USER")
	password = os.Getenv("TRUENAS_DS_PASSWORD")
	if domain == "" || user == "" || password == "" {
		t.Fatal("TRUENAS_DS=1 requires TRUENAS_DS_DOMAIN, TRUENAS_DS_USER, and TRUENAS_DS_PASSWORD to be set")
	}
	return domain, user, password
}

// dsPreCheckLDAP runs acctest.DSCheck plus dsPreCheckStatusRequiresDisabled
// (see dsPreCheck), then requires the LDAP server/bind env vars a live
// plain-LDAP bind needs. Unlike dsPreCheck (which t.Fatals on missing AD
// env vars — an AD join is expected to always be configured whenever
// TRUENAS_DS=1 is set for this package), this self-skips: LDAP and AD tests
// share the TRUENAS_DS=1 gate but exercise independent live environments
// (disposable AD domain controller vs. disposable OpenLDAP VM), and a
// TRUENAS_DS=1 run that only has one of the two environments available
// should still be able to run the other test.
func dsPreCheckLDAP(t *testing.T) (serverURL, baseDN, bindDN, bindPW string) {
	t.Helper()
	acctest.DSCheck(t)
	dsPreCheckStatusRequiresDisabled(t)
	serverURL = os.Getenv("TRUENAS_DS_LDAP_URL")
	baseDN = os.Getenv("TRUENAS_DS_LDAP_BASEDN")
	bindDN = os.Getenv("TRUENAS_DS_LDAP_BINDDN")
	bindPW = os.Getenv("TRUENAS_DS_LDAP_BINDPW")
	var missing []string
	if serverURL == "" {
		missing = append(missing, "TRUENAS_DS_LDAP_URL")
	}
	if baseDN == "" {
		missing = append(missing, "TRUENAS_DS_LDAP_BASEDN")
	}
	if bindDN == "" {
		missing = append(missing, "TRUENAS_DS_LDAP_BINDDN")
	}
	if bindPW == "" {
		missing = append(missing, "TRUENAS_DS_LDAP_BINDPW")
	}
	if len(missing) > 0 {
		t.Skipf("Skipping: TRUENAS_DS=1 is set but missing required env var(s) for the LDAP acceptance test: %s",
			strings.Join(missing, ", "))
	}
	return serverURL, baseDN, bindDN, bindPW
}

// dsPreCheckIPA runs acctest.DSCheck plus dsPreCheckStatusRequiresDisabled
// (see dsPreCheck), then requires the IPA target/domain/password env vars a
// live FreeIPA join needs. Self-skips (mirroring dsPreCheckLDAP, not
// dsPreCheck's t.Fatal) since a TRUENAS_DS=1 run that only has the AD and/or
// LDAP environment available should still be able to run those tests
// without the IPA one being configured.
//
// Unlike the AD test (dsPreCheck), there is no separate "user" env var:
// the disposable tftest-ipa FreeIPA realm's join credential is always the
// well-known "admin" account (see TestAccDirectoryServices_IPA), so only
// the password is read from the environment.
//
// Like the AD test, a live IPA join needs the TrueNAS box's own DNS
// resolver pointed at the IPA server (192.168.1.252, which runs its own
// DNS for the tfipa.lan zone) — an operational precondition around the
// test run, not something this test itself changes; see task-4-report.md
// for the exact save/point/restore commands used, and note this is a
// DIFFERENT nameserver than the AD test needs (192.168.1.250), so the two
// live runs cannot share one nameserver setting.
func dsPreCheckIPA(t *testing.T) (targetServer, domain, password string) {
	t.Helper()
	acctest.DSCheck(t)
	dsPreCheckStatusRequiresDisabled(t)
	targetServer = os.Getenv("TRUENAS_DS_IPA_TARGET")
	domain = os.Getenv("TRUENAS_DS_IPA_DOMAIN")
	password = os.Getenv("TRUENAS_DS_IPA_PASSWORD")
	var missing []string
	if targetServer == "" {
		missing = append(missing, "TRUENAS_DS_IPA_TARGET")
	}
	if domain == "" {
		missing = append(missing, "TRUENAS_DS_IPA_DOMAIN")
	}
	if password == "" {
		missing = append(missing, "TRUENAS_DS_IPA_PASSWORD")
	}
	if len(missing) > 0 {
		t.Skipf("Skipping: TRUENAS_DS=1 is set but missing required env var(s) for the IPA acceptance test: %s",
			strings.Join(missing, ", "))
	}
	return targetServer, domain, password
}

// ipaBaseDNFromDomain derives the LDAP base DN TrueNAS should use for an IPA
// join from the IPA domain name, matching FreeIPA's own convention of one
// "dc=" RDN per dot-separated domain label (e.g. "tfipa.lan" ->
// "dc=tfipa,dc=lan"). Confirmed live against the disposable tftest-ipa VM
// (212, 192.168.1.252) via an LDAP rootDSE query for defaultNamingContext
// run through the TrueNAS box's python3-ldap (it has no ldapsearch binary —
// see the LDAP test's own precedent): {"defaultNamingContext":
// ["dc=tfipa,dc=lan"]}. There is no TRUENAS_DS_IPA_BASEDN env var (unlike
// the LDAP test's TRUENAS_DS_LDAP_BASEDN) — FreeIPA always derives its base
// DN from the domain this way, so deriving it here avoids one more env var
// for a value that isn't actually independent.
func ipaBaseDNFromDomain(domain string) string {
	labels := strings.Split(domain, ".")
	for i, l := range labels {
		labels[i] = "dc=" + l
	}
	return strings.Join(labels, ",")
}

// TestAccDirectoryServices_IPA drives the singleton truenas_directoryservices
// resource through a full FreeIPA join lifecycle against the disposable
// TrueNAS 25.10 test VM and the disposable tftest-ipa FreeIPA VM (212,
// realm TFIPA.LAN, domain tfipa.lan, 192.168.1.252): join (enable=true,
// service_type=IPA, credential_type KERBEROS_USER using the realm's "admin"
// account, waiting for directoryservices.status to report HEALTHY), update
// a non-rejoin field (timeout) in place, then destroy — which disables
// directory services (enable=false) without leaving the realm — and
// verifies the box-side config shows disabled afterward. This run also
// exercises the LDAP -> IPA service_type switch path (resetStaleServiceType
// — see TestAccDirectoryServices_LDAP's doc comment): the box's directory
// services configuration is LDAP residue (from that test, or a prior run of
// this one) going into this test.
//
// Requires TF_ACC=1, TRUENAS_DS=1, TRUENAS_DS_ALLOWED_ENDPOINT (must equal
// TRUENAS_ENDPOINT — see acctest.DSCheck), and TRUENAS_DS_IPA_TARGET/DOMAIN/
// PASSWORD for the real, reachable FreeIPA server — see dsPreCheckIPA. The
// TrueNAS box's own DNS resolver must be pointed at the IPA server
// (192.168.1.252) before this test runs and restored afterward — see
// dsPreCheckIPA's doc comment and task-4-report.md.
func TestAccDirectoryServices_IPA(t *testing.T) {
	targetServer, domain, password := dsPreCheckIPA(t)
	const hostname = "tn2510"
	const user = "admin"
	baseDN := ipaBaseDNFromDomain(domain)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDirectoryServicesDisabledWithType("IPA"),
		Steps: []resource.TestStep{
			// Step 1: join the IPA realm, wait for HEALTHY.
			{
				Config: acctest.ProviderConfig() + testAccDirectoryServicesIPAConfig(targetServer, hostname, domain, baseDN, user, password, 10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "id", "directoryservices"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "service_type", "IPA"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "enable", "true"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "timeout", "10"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ipa.target_server", targetServer),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ipa.hostname", hostname),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ipa.domain", domain),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ipa.basedn", baseDN),
					testAccCheckDirectoryServicesStatus("HEALTHY"),
				),
			},
			// Step 2: update timeout in place (no re-join expected).
			{
				Config: acctest.ProviderConfig() + testAccDirectoryServicesIPAConfig(targetServer, hostname, domain, baseDN, user, password, 20),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "timeout", "20"),
					testAccCheckDirectoryServicesStatus("HEALTHY"),
				),
			},
			// Step 3: destroy is implicit (Steps end, CheckDestroy runs) —
			// verifies enable=false and does NOT call directoryservices.leave.
		},
	})
}

func testAccDirectoryServicesIPAConfig(targetServer, hostname, domain, baseDN, user, password string, timeout int) string {
	return fmt.Sprintf(`
resource "truenas_directoryservices" "test" {
  service_type = "IPA"
  enable       = true
  timeout      = %d

  credential = {
    credential_type = "KERBEROS_USER"
    username        = %q
    password        = %q
  }

  configuration_ipa = {
    target_server         = %q
    hostname               = %q
    domain                 = %q
    basedn                 = %q
    validate_certificates  = false
  }
}
`, timeout, user, password, targetServer, hostname, domain, baseDN)
}

// TestAccDirectoryServices_ActiveDirectory drives the singleton
// truenas_directoryservices resource through a full Active Directory join
// lifecycle against the disposable TrueNAS 25.10 test VM: join
// (enable=true, waiting for directoryservices.status to report HEALTHY),
// update a non-join field (timeout) in place, then destroy — which disables
// directory services (enable=false) WITHOUT leaving the domain — and
// verifies the box-side config shows disabled afterward.
//
// The join config also sets an explicit "idmap" block with UID/GID ranges
// that deliberately differ from TrueNAS's own defaults (confirmed live,
// see task-8-report.md: builtin 90000001-100000000, idmap_domain
// 100000001-200000000 backend RID) — builtin 91000001-92000000,
// idmap_domain (backend RID) 201000001-202000000 — to verify the explicit
// idmap block (Task 1 of this plan) actually reaches TrueNAS and round-trips
// on read, rather than merely accepting whatever server-side default would
// have applied anyway. Both ranges are comfortably inside the schema's
// idmapRangeValidators bound (1000-2147000000) and non-overlapping with
// each other.
//
// Requires TF_ACC=1, TRUENAS_DS=1, TRUENAS_DS_ALLOWED_ENDPOINT (must equal
// TRUENAS_ENDPOINT — see acctest.DSCheck), and TRUENAS_DS_DOMAIN/USER/
// PASSWORD for a real, reachable Active Directory domain controller. This
// test makes real changes to that domain (creates and later leaves in
// place, but disables, a TrueNAS computer account) and must only be run
// against a disposable domain/box.
//
// Like TestAccDirectoryServices_IPA, this join needs the TrueNAS box's own
// DNS resolver pointed at the domain's controller (192.168.1.250 for the
// TFTEST.LAN Samba AD DC) — an operational precondition around the test
// run, not something the test itself changes; see dsPreCheck's doc comment
// and task-4-report.md for the exact save/point/restore commands used.
func TestAccDirectoryServices_ActiveDirectory(t *testing.T) {
	domain, user, password := dsPreCheck(t)
	const hostname = "tn2510"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDirectoryServicesDisabled,
		Steps: []resource.TestStep{
			// Step 1: join the domain, wait for HEALTHY.
			{
				Config: acctest.ProviderConfig() + testAccDirectoryServicesConfig(hostname, domain, user, password, 10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "id", "directoryservices"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "service_type", "ACTIVEDIRECTORY"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "enable", "true"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "timeout", "10"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.hostname", hostname),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.domain", domain),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.idmap.builtin.range_low", "91000001"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.idmap.builtin.range_high", "92000000"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.idmap.idmap_domain.idmap_backend", "RID"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.idmap.idmap_domain.range_low", "201000001"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_activedirectory.idmap.idmap_domain.range_high", "202000000"),
					testAccCheckDirectoryServicesStatus("HEALTHY"),
				),
			},
			// Step 2: update timeout in place (no re-join expected).
			{
				Config: acctest.ProviderConfig() + testAccDirectoryServicesConfig(hostname, domain, user, password, 20),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "timeout", "20"),
					testAccCheckDirectoryServicesStatus("HEALTHY"),
				),
			},
			// Step 3: destroy is implicit (Steps end, CheckDestroy runs) —
			// verifies enable=false and does NOT call directoryservices.leave.
		},
	})
}

// TestAccDirectoryServices_LDAP drives the singleton truenas_directoryservices
// resource through a full plain-LDAP bind lifecycle against the disposable
// TrueNAS 25.10 test VM and the disposable OpenLDAP VM (VM 211,
// tftest-ldap): bind (enable=true, service_type=LDAP, credential_type
// LDAP_PLAIN, waiting for directoryservices.status to report HEALTHY),
// confirm the seeded LDAP user is visible via user.query, update a
// non-rejoin field (timeout) in place, then destroy — which disables
// directory services (enable=false) without unbinding — and verifies the
// box-side config shows disabled afterward.
//
// Requires TF_ACC=1, TRUENAS_DS=1, TRUENAS_DS_ALLOWED_ENDPOINT (must equal
// TRUENAS_ENDPOINT — see acctest.DSCheck), and TRUENAS_DS_LDAP_URL/BASEDN/
// BINDDN/BINDPW for a real, reachable plain-LDAP server (self-signed TLS is
// fine: validate_certificates=false). No DNS/nameserver changes are needed
// for LDAP (the server is addressed by IP or by a name the box's existing
// resolver can already reach) — unlike TestAccDirectoryServices_ActiveDirectory,
// there is no server-side DNS precondition to document here.
//
// This test's live run against the shared disposable VM 110 (which had a
// prior TestAccDirectoryServices_ActiveDirectory join in its history — see
// that test's own doc comment) is exactly what surfaced the
// resetStaleServiceType workaround in resource.go: switching service_type
// away from a previous ACTIVEDIRECTORY join left a stale kerberos_realm in
// place that TrueNAS could not otherwise clear via a normal enabling
// update, and this test would fail with "[EFAULT] Directory services are
// not configured to use kerberos credentials" without it (see that
// function's doc comment for the full trace).
func TestAccDirectoryServices_LDAP(t *testing.T) {
	serverURL, baseDN, bindDN, bindPW := dsPreCheckLDAP(t)
	// StartTLS applies only over a plain "ldap://" URL; "ldaps://" already
	// carries its own TLS and starttls would be redundant/rejected.
	startTLS := strings.HasPrefix(serverURL, "ldap://")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDirectoryServicesDisabledWithType("LDAP"),
		Steps: []resource.TestStep{
			// Step 1: bind, wait for HEALTHY, confirm the seeded LDAP user
			// resolves through the directory service.
			{
				Config: acctest.ProviderConfig() + testAccDirectoryServicesLDAPConfig(serverURL, baseDN, bindDN, bindPW, startTLS, 10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "id", "directoryservices"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "service_type", "LDAP"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "enable", "true"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "timeout", "10"),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ldap.basedn", baseDN),
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ldap.server_urls.0", serverURL),
					testAccCheckDirectoryServicesStatus("HEALTHY"),
					testAccCheckLDAPSeededUserVisible("tfuser1", 21001),
				),
			},
			// Step 2: update timeout in place (no re-bind expected).
			{
				Config: acctest.ProviderConfig() + testAccDirectoryServicesLDAPConfig(serverURL, baseDN, bindDN, bindPW, startTLS, 20),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_directoryservices.test", "timeout", "20"),
					testAccCheckDirectoryServicesStatus("HEALTHY"),
				),
			},
			// Step 3: destroy is implicit (Steps end, CheckDestroy runs) —
			// verifies enable=false and does NOT attempt to unbind/leave.
		},
	})
}

func testAccDirectoryServicesLDAPConfig(serverURL, baseDN, bindDN, bindPW string, startTLS bool, timeout int) string {
	return fmt.Sprintf(`
resource "truenas_directoryservices" "test" {
  service_type = "LDAP"
  enable       = true
  timeout      = %d

  credential = {
    credential_type = "LDAP_PLAIN"
    binddn          = %q
    bindpw          = %q
  }

  configuration_ldap = {
    server_urls           = [%q]
    basedn                = %q
    starttls              = %t
    validate_certificates = false
  }
}
`, timeout, bindDN, bindPW, serverURL, baseDN, startTLS)
}

// testAccCheckLDAPSeededUserVisible asserts that the given username/uid
// pair (one of the RFC2307 users seeded onto the disposable OpenLDAP VM —
// see task-2-report.md) resolves through user.query once directory
// services reports HEALTHY.
//
// No special query flag/"extra" option is needed: middlewared's
// account.py UserService.query calls filters_include_ds_accounts(filters)
// and, unless the filters explicitly restrict to local/builtin accounts,
// includes directoryservices.cache.query results alongside local ones
// whenever directoryservices.status reports a non-null type. For a
// single-field "=" filter on username (exactly what's used here),
// directoryservices.cache.query's special case issues a direct NSS lookup
// (getpwnam via SSSD) and populates the cache on demand rather than relying
// on a slower, periodic full cache fill — probed live via a throwaway
// `user.query` section added temporarily to cmd/debug_api/main.go: the
// seeded tfuser1 (uid 21001) resolved on the FIRST attempt immediately
// after HEALTHY, with no retry needed, so this check is a hard assertion
// rather than the softer "assert HEALTHY only" fallback the task brief
// allows for cache-timing flakiness.
func testAccCheckLDAPSeededUserVisible(username string, uid int64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "user.query", [][]any{{"username", "=", username}})
		if err != nil {
			return fmt.Errorf("error querying user.query for %q: %w", username, err)
		}
		var users []struct {
			Username string `json:"username"`
			UID      int64  `json:"uid"`
			Local    bool   `json:"local"`
		}
		if err := json.Unmarshal(raw, &users); err != nil {
			return fmt.Errorf("error parsing user.query response: %w", err)
		}
		if len(users) != 1 {
			return fmt.Errorf("user.query(username=%q) returned %d results, want 1 (raw: %s)", username, len(users), raw)
		}
		if users[0].UID != uid {
			return fmt.Errorf("user.query(username=%q) uid = %d, want %d", username, users[0].UID, uid)
		}
		if users[0].Local {
			return fmt.Errorf("user.query(username=%q) local = true, want false (expected an LDAP-provided account)", username)
		}
		return nil
	}
}

// testAccDirectoryServicesConfig builds the ACTIVEDIRECTORY join HCL,
// including an explicit "idmap" block with non-default UID/GID ranges (see
// TestAccDirectoryServices_ActiveDirectory's doc comment for why) — the
// idmap values are fixed regardless of "timeout", so the in-place
// timeout-only update step produces no idmap diff.
func testAccDirectoryServicesConfig(hostname, domain, user, password string, timeout int) string {
	return fmt.Sprintf(`
resource "truenas_directoryservices" "test" {
  service_type = "ACTIVEDIRECTORY"
  enable       = true
  timeout      = %d

  credential = {
    credential_type = "KERBEROS_USER"
    username        = %q
    password        = %q
  }

  configuration_activedirectory = {
    hostname = %q
    domain   = %q

    idmap = {
      builtin = {
        range_low  = 91000001
        range_high = 92000000
      }
      idmap_domain = {
        idmap_backend = "RID"
        range_low     = 201000001
        range_high    = 202000000
      }
    }
  }
}
`, timeout, user, password, hostname, domain)
}

// testAccCheckDirectoryServicesStatus asserts directoryservices.status
// reports the given status directly against the box, independent of
// whatever the provider itself wrote into state (the datasource/resource
// don't expose status on the resource, only the datasource does — this
// check exists so the resource-only test steps can still assert on health).
func testAccCheckDirectoryServicesStatus(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "directoryservices.status")
		if err != nil {
			return fmt.Errorf("error reading directoryservices.status: %w", err)
		}
		var st struct {
			Status *string `json:"status"`
		}
		if err := json.Unmarshal(raw, &st); err != nil {
			return fmt.Errorf("error parsing directoryservices.status response: %w", err)
		}
		got := "null"
		if st.Status != nil {
			got = *st.Status
		}
		if got != want {
			return fmt.Errorf("directoryservices.status = %q, want %q", got, want)
		}
		return nil
	}
}

// testAccCheckDirectoryServicesDisabled is the CheckDestroy function for
// the ACTIVEDIRECTORY test: after the resource is destroyed,
// directoryservices.config must show enable=false (Delete calls
// directoryservices.update with enable=false only) while the domain join
// itself is left in place (never directoryservices.leave), i.e.
// service_type remains ACTIVEDIRECTORY.
func testAccCheckDirectoryServicesDisabled(s *terraform.State) error {
	return testAccCheckDirectoryServicesDisabledWithType("ACTIVEDIRECTORY")(s)
}

// testAccCheckDirectoryServicesDisabledWithType builds a CheckDestroy
// function parametrized on the expected surviving service_type: after the
// resource is destroyed, directoryservices.config must show enable=false
// (Delete calls directoryservices.update with enable=false only) while the
// join/bind configuration itself is left in place (never
// directoryservices.leave, and for LDAP there is no "leave" concept to
// begin with — the bind is simply disabled).
func testAccCheckDirectoryServicesDisabledWithType(wantServiceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "directoryservices.config")
		if err != nil {
			return fmt.Errorf("error reading directoryservices.config: %w", err)
		}
		var cfg struct {
			Enable      bool    `json:"enable"`
			ServiceType *string `json:"service_type"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return fmt.Errorf("error parsing directoryservices.config response: %w", err)
		}
		if cfg.Enable {
			return fmt.Errorf("directoryservices.config still shows enable=true after destroy")
		}
		if cfg.ServiceType == nil || *cfg.ServiceType != wantServiceType {
			st := "null"
			if cfg.ServiceType != nil {
				st = *cfg.ServiceType
			}
			return fmt.Errorf("directoryservices.config service_type = %s, want %s to remain "+
				"(destroy must disable, not leave/unbind, the configuration)", st, wantServiceType)
		}
		return nil
	}
}
