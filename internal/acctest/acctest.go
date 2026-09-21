// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"truenas": providerserver.NewProtocol6WithError(provider.New("test")()),
}

var (
	clientOnce sync.Once
	testClient *client.Client
)

// Endpoint returns the TrueNAS WebSocket endpoint to use for acceptance
// tests, from the TRUENAS_ENDPOINT environment variable. When unset it
// returns a syntactically valid RFC 6761 .invalid placeholder: test config
// strings are built before PreCheck gets a chance to skip, so this must not
// fail — PreCheck enforces the real requirement before anything dials out.
func Endpoint() string {
	if v := os.Getenv("TRUENAS_ENDPOINT"); v != "" {
		// Same legacy-path rewrite the provider applies: the client speaks
		// JSON-RPC 2.0, so a configured /websocket endpoint must land on
		// /api/current (acctest.Client() dials this URL directly).
		if base, ok := strings.CutSuffix(v, "/websocket"); ok {
			return base + "/api/current"
		}
		return v
	}
	return "wss://truenas.invalid/api/current"
}

// EndpointHost returns the bare host (IP or name) portion of Endpoint(),
// for tests that must bind or reference an address on the target box
// (e.g. iSCSI portal listen IPs, NVMe-oF port addresses).
func EndpointHost() string {
	u, err := url.Parse(Endpoint())
	if err != nil || u.Host == "" {
		panic("acctest: cannot parse host from TRUENAS_ENDPOINT")
	}
	return u.Hostname()
}

// TestPool returns the ZFS pool to use for acceptance tests, from
// TRUENAS_TEST_POOL, defaulting to "tank".
func TestPool() string {
	if v := os.Getenv("TRUENAS_TEST_POOL"); v != "" {
		return v
	}
	return "tank"
}

// RandName returns prefix + "-" + an 8 character lowercase hex suffix,
// suitable for generating unique resource names in acceptance tests.
// It panics only if the system RNG fails.
func RandName(prefix string) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		panic("acctest: rand.Read failed: " + err.Error())
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
}

// RandNQN returns a unique NVMe host NQN in the uuid form TrueNAS validates:
// the uuid segment must be a full RFC-4122 UUID, so a short random suffix is
// rejected ("uuid is incorrect length").
func RandNQN() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("acctest: rand.Read failed: " + err.Error())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("nqn.2014-08.org.nvmexpress:uuid:%x-%x-%x-%x-%x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

// Client returns a singleton *client.Client connected to the acceptance test TrueNAS.
func Client() *client.Client {
	clientOnce.Do(func() {
		tlsCfg, _ := client.BuildTLSConfig(true, "")
		c := client.New(Endpoint(), tlsCfg)
		var authFn func(ctx context.Context) error
		if apiKey := os.Getenv("TRUENAS_API_KEY"); apiKey != "" {
			// Upgrades to SCRAM on 26.0+ when TRUENAS_USERNAME names the
			// key owner; plain login otherwise.
			username := os.Getenv("TRUENAS_USERNAME")
			authFn = func(ctx context.Context) error { return client.AuthAPIKeyAuto(ctx, c, username, apiKey) }
		} else {
			username := os.Getenv("TRUENAS_USERNAME")
			password := os.Getenv("TRUENAS_PASSWORD")
			authFn = func(ctx context.Context) error { return client.AuthPassword(ctx, c, username, password) }
		}
		if err := c.Connect(context.Background(), authFn); err != nil {
			panic("acctest: cannot connect to TrueNAS: " + err.Error())
		}
		testClient = c
	})
	return testClient
}

// RestoreCall invokes an idempotent singleton-config read or write (e.g. a
// t.Cleanup-registered restore, or the paired "read the original value"
// call that precedes it) through the shared acctest.Client() connection,
// with the same transient-failure retry and reconnect behavior as
// client.CallRead.
//
// The long-lived acctest.Client() connection sits idle for the whole
// duration of a Tier-2 set-and-restore test's Terraform steps (which run
// over the provider's own, separate connection), so by the time t.Cleanup
// fires it has occasionally gone stale and a plain Call returns "not
// connected" — a real, observed flake (see task-2-report.md and this
// task's report), not a resource defect: the Terraform steps themselves,
// including the step that already applies the restored value, still
// succeeded. Both directions (read-original and restore-write) are
// idempotent, so retrying either on a transient failure is safe — same
// justification CallRead already relies on for reads.
func RestoreCall(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return Client().CallRead(ctx, method, params...)
}

// ServerVersionAtLeast reports whether the acceptance-test server is at or
// above major.minor, for gating attributes that exist only on newer TrueNAS
// releases (e.g. nvmet_host description, 26.0+). Unlike most helpers it
// dials the box, so it enforces the PreCheck env requirements itself:
// tests call it while building config strings, before resource.Test runs
// PreCheck. Fatals if the version cannot be determined.
func ServerVersionAtLeast(t *testing.T, major, minor int) bool {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 to run acceptance tests")
	}
	if os.Getenv("TRUENAS_ENDPOINT") == "" {
		t.Fatal("Set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/api/current) for acceptance tests")
	}
	ok, err := Client().VersionAtLeast(context.Background(), major, minor)
	if err != nil {
		t.Fatalf("acctest: cannot determine server version: %v", err)
	}
	return ok
}

// PreCheck verifies required env vars are set before running acceptance tests.
func PreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 to run acceptance tests")
	}
	if os.Getenv("TRUENAS_ENDPOINT") == "" {
		t.Fatal("Set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/api/current) for acceptance tests")
	}
	if os.Getenv("TRUENAS_API_KEY") == "" && (os.Getenv("TRUENAS_USERNAME") == "" || os.Getenv("TRUENAS_PASSWORD") == "") {
		t.Fatal("Set TRUENAS_API_KEY or TRUENAS_USERNAME+TRUENAS_PASSWORD for acceptance tests")
	}
}

// DisruptiveCheck gates singleton set-and-restore tests that mutate
// box-wide, non-idempotent state (e.g. global config singletons). It runs
// PreCheck and then skips unless TRUENAS_DISRUPTIVE=1 is set, since these
// tests can leave the box in a different state than they found it.
func DisruptiveCheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_DISRUPTIVE") != "1" {
		t.Skip("Set TRUENAS_DISRUPTIVE=1 to run disruptive acceptance tests")
	}
}

// AppsCheck gates tests that exercise app resources, which pull container
// images from the network and can be slow or bandwidth-heavy. It runs
// PreCheck and then skips unless TRUENAS_APPS=1 is set.
func AppsCheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_APPS") != "1" {
		t.Skip("Set TRUENAS_APPS=1 to run app acceptance tests")
	}
}

// DSCheck gates tests that join/leave an Active Directory domain via the
// truenas_directoryservices resource. It runs PreCheck and then skips
// unless TRUENAS_DS=1 is set, since domain-join tests mutate real
// directory-service state and can take minutes to run.
//
// When TRUENAS_DS=1 it additionally enforces a disposable-box guard: the
// env var TRUENAS_DS_ALLOWED_ENDPOINT must be set and must equal
// Endpoint() exactly. Domain-join acceptance tests must run ONLY against a
// disposable test VM, never a shared or production TrueNAS box — an
// unintended AD join (or a failed join left half-configured) is much more
// disruptive than the other acceptance tests in this suite, so this check
// fails loudly (t.Fatal, not t.Skip) rather than silently doing the wrong
// thing if the guard env var is missing or mismatched.
func DSCheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_DS") != "1" {
		t.Skip("Set TRUENAS_DS=1 to run directory services (Active Directory join) acceptance tests")
	}
	allowed := os.Getenv("TRUENAS_DS_ALLOWED_ENDPOINT")
	if allowed == "" {
		t.Fatal("TRUENAS_DS=1 requires TRUENAS_DS_ALLOWED_ENDPOINT to be set to the disposable test VM's " +
			"endpoint, as a guard against accidentally domain-joining a shared or production TrueNAS box")
	}
	if allowed != Endpoint() {
		t.Fatalf("TRUENAS_DS_ALLOWED_ENDPOINT (%q) does not match TRUENAS_ENDPOINT (%q): refusing to run "+
			"directory services acceptance tests against a box that isn't the disposable test VM", allowed, Endpoint())
	}
}

// HACheck gates tests that exercise Enterprise HA failover behavior via the
// truenas_failover_config resource/datasource. It runs PreCheck and then
// skips unless TRUENAS_HA=1 is set, since these tests mutate the box's
// live failover configuration (timeout, and — outside any committed test —
// the genuinely disruptive "disabled"/"master" fields) and are only
// meaningful against a disposable HA pair.
//
// When TRUENAS_HA=1 it additionally enforces a disposable-box guard,
// mirroring DSCheck's: TRUENAS_HA_ALLOWED_ENDPOINT must be set and must
// equal Endpoint() exactly, else t.Fatal — HA tests must never run against
// a box that isn't explicitly designated disposable, since an unintended
// failover trigger is far more disruptive than the other acceptance tests
// in this suite.
//
// Finally, once both env guards pass, it does a live failover.licensed
// probe: a box that is reachable and correctly guarded but not actually
// licensed for Enterprise HA (e.g. the cross-release 26.0 box used only
// for shape probing in this provider's own test matrix) has nothing
// meaningful for an HA-gated test to exercise. That is a clean t.Skip, not
// a t.Fatal — unlike the endpoint guard above, an unlicensed box is not a
// safety problem, just an environment that doesn't support the feature.
func HACheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_HA") != "1" {
		t.Skip("Set TRUENAS_HA=1 to run Enterprise HA / failover acceptance tests")
	}
	allowed := os.Getenv("TRUENAS_HA_ALLOWED_ENDPOINT")
	if allowed == "" {
		t.Fatal("TRUENAS_HA=1 requires TRUENAS_HA_ALLOWED_ENDPOINT to be set to the disposable HA test box's " +
			"endpoint, as a guard against accidentally exercising real failover behavior against a shared or " +
			"production TrueNAS box")
	}
	if allowed != Endpoint() {
		t.Fatalf("TRUENAS_HA_ALLOWED_ENDPOINT (%q) does not match TRUENAS_ENDPOINT (%q): refusing to run "+
			"Enterprise HA acceptance tests against a box that isn't the disposable HA test box", allowed, Endpoint())
	}

	raw, err := Client().CallRead(context.Background(), "failover.licensed")
	if err != nil {
		t.Fatalf("acctest: cannot determine failover.licensed: %v", err)
	}
	var licensed bool
	if err := json.Unmarshal(raw, &licensed); err != nil {
		t.Fatalf("acctest: cannot parse failover.licensed response: %v", err)
	}
	if !licensed {
		t.Skip("failover.licensed=false on this box: not licensed for Enterprise HA, skipping")
	}
}

// IPMICheck gates tests that exercise a box's BMC LAN configuration via the
// truenas_ipmi_lan resource/datasource. IPMI LAN is a baseboard-management-
// controller feature present on any physical box with a BMC; it is unrelated
// to Enterprise HA (these tests were originally gated behind HACheck only
// because the sole BMC-equipped disposable box available then happened to be
// an HA pair). It runs PreCheck and then skips unless TRUENAS_IPMI=1 is set.
//
// When TRUENAS_IPMI=1 it additionally enforces a disposable-box guard,
// mirroring DSCheck/HACheck's: TRUENAS_IPMI_ALLOWED_ENDPOINT must be set and
// must equal Endpoint() exactly, else t.Fatal. Any test that mutates BMC LAN
// state (e.g. toggling a channel's 802.1Q vlan) is layered further behind
// DisruptiveCheck; this guard is the outer safety net ensuring such a test
// can only ever run against a box explicitly designated for it, never a box
// reached by accident through an ambient TRUENAS_ENDPOINT.
func IPMICheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_IPMI") != "1" {
		t.Skip("Set TRUENAS_IPMI=1 to run IPMI LAN (BMC) acceptance tests")
	}
	allowed := os.Getenv("TRUENAS_IPMI_ALLOWED_ENDPOINT")
	if allowed == "" {
		t.Fatal("TRUENAS_IPMI=1 requires TRUENAS_IPMI_ALLOWED_ENDPOINT to be set to the target box's " +
			"endpoint, as a guard against accidentally reconfiguring a BMC LAN channel on a box that " +
			"wasn't explicitly designated for IPMI testing")
	}
	if allowed != Endpoint() {
		t.Fatalf("TRUENAS_IPMI_ALLOWED_ENDPOINT (%q) does not match TRUENAS_ENDPOINT (%q): refusing to run "+
			"IPMI LAN acceptance tests against a box that isn't the designated IPMI test box", allowed, Endpoint())
	}
}

// ACMECheck gates the live ACME issuance acceptance test
// (TestAccCertificate_acmeIssuance), which drives a real certificate order
// end to end against an ACME CA (a Pebble test server) using the DNS-01
// "shell" authenticator. It runs PreCheck and then skips unless
// TRUENAS_ACME=1 is set, since the test stands up a real ACME account, an
// on-box shell script, and a trusted-store CA import, and takes tens of
// seconds per run.
//
// When TRUENAS_ACME=1 it additionally requires the three env vars that make
// the test portable and self-contained (t.Fatal, mirroring DSCheck/HACheck's
// loud failure when the gate is on but its environment is not provided):
//
//   - TRUENAS_ACME_DIRECTORY   — the ACME directory URL (e.g. the Pebble
//     directory https://<pebble>:14000/dir). See ACMEDirectory for why a
//     trailing slash is enforced.
//   - TRUENAS_ACME_CHALLTESTSRV — the challtestsrv management HTTP base URL
//     (e.g. http://<pebble>:8055), where the DNS-01 shell script publishes
//     and clears the challenge TXT records.
//   - TRUENAS_ACME_CA_PEM      — path to (or literal content of) the PEM the
//     ACME directory endpoint's TLS is signed by. For Pebble's default setup
//     that is the self-signed directory-endpoint cert itself (fetchable with
//     `openssl s_client -connect <pebble>:14000`), NOT the issuance root from
//     :15000/roots/0 — the two are different certs. The test imports it into
//     the box's trusted store so the ACME client trusts the directory's TLS.
func ACMECheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_ACME") != "1" {
		t.Skip("Set TRUENAS_ACME=1 to run the live ACME issuance acceptance test")
	}
	for _, k := range []string{"TRUENAS_ACME_DIRECTORY", "TRUENAS_ACME_CHALLTESTSRV", "TRUENAS_ACME_CA_PEM"} {
		if os.Getenv(k) == "" {
			t.Fatalf("TRUENAS_ACME=1 requires %s to be set (see acctest.ACMECheck) so the ACME "+
				"issuance test is portable and self-contained", k)
		}
	}
}

// ACMEDirectory returns the ACME directory URL from TRUENAS_ACME_DIRECTORY,
// normalized to end with a single trailing slash. The trailing slash is
// load-bearing, not cosmetic: TrueNAS stores an acme.registration keyed by
// the slash-normalized directory but looks it up for reuse by the raw URI
// passed to certificate.create. Without the slash the second run fails with
// "A registration with the specified directory uri already exists"; with it,
// the one ACME account is reused and issuance is repeatable (see
// MIDDLEWARE-FINDINGS.md). Returns a .invalid placeholder when unset so
// config strings build before PreCheck/ACMECheck skips.
func ACMEDirectory() string {
	v := os.Getenv("TRUENAS_ACME_DIRECTORY")
	if v == "" {
		return "https://acme.invalid/dir/"
	}
	if !strings.HasSuffix(v, "/") {
		v += "/"
	}
	return v
}

// ACMEChalltestsrv returns the challtestsrv management HTTP base URL from
// TRUENAS_ACME_CHALLTESTSRV, with any trailing slash trimmed. Placeholder
// when unset (config strings build before the gate skips).
func ACMEChalltestsrv() string {
	v := os.Getenv("TRUENAS_ACME_CHALLTESTSRV")
	if v == "" {
		return "http://challtestsrv.invalid:8055"
	}
	return strings.TrimRight(v, "/")
}

// ACMECAPEM returns the trusted-store CA PEM from TRUENAS_ACME_CA_PEM,
// treating the value as a file path when it names a readable file and
// otherwise as literal PEM content. Fatals if a path is given but unreadable.
func ACMECAPEM(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TRUENAS_ACME_CA_PEM")
	if v == "" {
		return ""
	}
	if b, err := os.ReadFile(v); err == nil {
		return string(b)
	} else if strings.Contains(v, "BEGIN CERTIFICATE") {
		return v
	} else {
		t.Fatalf("TRUENAS_ACME_CA_PEM=%q is neither a readable file nor PEM content: %v", v, err)
		return ""
	}
}

// UploadFile writes content to remotePath on the acceptance-test box via the
// TrueNAS HTTP upload endpoint (POST /_upload driving filesystem.put),
// authenticated with the API key, and waits for the resulting job to finish.
// This is the portable way to place a file on the box (e.g. a DNS-01 shell
// script) — the JSON-RPC websocket client can't stream filesystem.put's input
// pipe. mode is the octal-as-decimal Unix mode for the created file (e.g.
// 0o755 == 493). Requires TRUENAS_API_KEY (fatals otherwise).
func UploadFile(t *testing.T, remotePath string, content []byte, mode int) {
	t.Helper()
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if apiKey == "" {
		t.Fatal("acctest.UploadFile requires TRUENAS_API_KEY")
	}
	u, err := url.Parse(Endpoint())
	if err != nil {
		t.Fatalf("acctest.UploadFile: cannot parse endpoint: %v", err)
	}
	// The upload endpoint mirrors the WebSocket endpoint's transport:
	// wss:// -> https://, ws:// -> http:// (test/local boxes).
	scheme := "https"
	if u.Scheme == "ws" {
		scheme = "http"
	}
	uploadURL := scheme + "://" + u.Host + "/_upload"

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	meta := fmt.Sprintf(`{"method":"filesystem.put","params":[%q,{"mode":%d}]}`, remotePath, mode)
	if err := w.WriteField("data", meta); err != nil {
		t.Fatalf("acctest.UploadFile: write data field: %v", err)
	}
	fw, err := w.CreateFormFile("file", "upload.bin")
	if err != nil {
		t.Fatalf("acctest.UploadFile: create file field: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("acctest.UploadFile: write file content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("acctest.UploadFile: close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, uploadURL, &body)
	if err != nil {
		t.Fatalf("acctest.UploadFile: new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("acctest.UploadFile: POST %s: %v", uploadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("acctest.UploadFile: POST %s returned %d: %s", uploadURL, resp.StatusCode, rb)
	}
	var jr struct {
		JobID int64 `json:"job_id"`
	}
	if err := json.Unmarshal(rb, &jr); err != nil || jr.JobID == 0 {
		t.Fatalf("acctest.UploadFile: unexpected upload response %q (err %v)", rb, err)
	}
	waitJob(t, jr.JobID)
}

// waitJob polls core.get_jobs for the given job id until it reaches a
// terminal state, fataling on FAILED or timeout. Used by UploadFile to
// confirm the file is on disk before a test uses it.
func waitJob(t *testing.T, jobID int64) {
	t.Helper()
	c := Client()
	for i := 0; i < 60; i++ {
		raw, err := c.CallRead(context.Background(), "core.get_jobs", [][]any{{"id", "=", jobID}})
		if err != nil {
			t.Fatalf("acctest.waitJob: core.get_jobs: %v", err)
		}
		var jobs []struct {
			State string `json:"state"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(raw, &jobs); err != nil {
			t.Fatalf("acctest.waitJob: parse jobs: %v", err)
		}
		if len(jobs) == 1 {
			switch jobs[0].State {
			case "SUCCESS":
				return
			case "FAILED", "ABORTED":
				t.Fatalf("acctest.waitJob: job %d %s: %s", jobID, jobs[0].State, jobs[0].Error)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("acctest.waitJob: job %d did not finish in time", jobID)
}

// ProviderConfig returns HCL for the provider block used in acceptance tests.
func ProviderConfig() string {
	return fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
}
`, Endpoint())
}

// Ensure resource package is importable
var _ resource.TestCheckFunc = nil
