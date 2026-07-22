package directoryservices_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// dsPreCheck runs acctest.DSCheck (which itself skips unless TRUENAS_DS=1
// and enforces the disposable-VM TRUENAS_DS_ALLOWED_ENDPOINT guard), then
// requires the domain/user/password env vars a live Active Directory join
// needs.
//
// It does NOT verify DNS is pointed at the domain controller server-side:
// probing core.get_methods for a dnsclient/dns forward-lookup-or-SRV method
// (SCALE 25.10, live) found only dns.query, which lists locally configured
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
	domain = os.Getenv("TRUENAS_DS_DOMAIN")
	user = os.Getenv("TRUENAS_DS_USER")
	password = os.Getenv("TRUENAS_DS_PASSWORD")
	if domain == "" || user == "" || password == "" {
		t.Fatal("TRUENAS_DS=1 requires TRUENAS_DS_DOMAIN, TRUENAS_DS_USER, and TRUENAS_DS_PASSWORD to be set")
	}
	return domain, user, password
}

// TestAccDirectoryServices_ActiveDirectory drives the singleton
// truenas_directoryservices resource through a full Active Directory join
// lifecycle against the disposable TrueNAS SCALE 25.10 test VM: join
// (enable=true, waiting for directoryservices.status to report HEALTHY),
// update a non-join field (timeout) in place, then destroy — which disables
// directory services (enable=false) WITHOUT leaving the domain — and
// verifies the box-side config shows disabled afterward.
//
// Requires TF_ACC=1, TRUENAS_DS=1, TRUENAS_DS_ALLOWED_ENDPOINT (must equal
// TRUENAS_ENDPOINT — see acctest.DSCheck), and TRUENAS_DS_DOMAIN/USER/
// PASSWORD for a real, reachable Active Directory domain controller. This
// test makes real changes to that domain (creates and later leaves in
// place, but disables, a TrueNAS computer account) and must only be run
// against a disposable domain/box.
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

// testAccCheckDirectoryServicesDisabled is the CheckDestroy function: after
// the resource is destroyed, directoryservices.config must show enable=false
// (Delete calls directoryservices.update with enable=false only) while the
// domain join itself is left in place (never directoryservices.leave).
func testAccCheckDirectoryServicesDisabled(s *terraform.State) error {
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
	// service_type is expected to remain ACTIVEDIRECTORY (destroy disables,
	// it does not leave the domain / wipe the join config).
	if cfg.ServiceType == nil || *cfg.ServiceType != "ACTIVEDIRECTORY" {
		st := "null"
		if cfg.ServiceType != nil {
			st = *cfg.ServiceType
		}
		return fmt.Errorf("directoryservices.config service_type = %s, want ACTIVEDIRECTORY to remain "+
			"(destroy must disable, not leave, the domain)", st)
	}
	return nil
}
