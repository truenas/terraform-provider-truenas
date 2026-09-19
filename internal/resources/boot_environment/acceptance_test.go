// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// bootEnvSummary is the subset of boot.environment.query fields this test
// package needs directly (outside of the provider's own resource code).
type bootEnvSummary struct {
	ID     string `json:"id"`
	Active bool   `json:"active"`
}

// activeBootEnvironmentName looks up the name of the boot environment that is
// currently booted (active). Tests clone from this BE rather than ever
// mutating or destroying it, so the host running the acceptance suite is
// never left with a missing or deactivated boot environment.
func activeBootEnvironmentName(t *testing.T) string {
	t.Helper()

	c := acctest.Client()
	raw, err := c.Call(context.Background(), "boot.environment.query", [][]any{{"active", "=", true}})
	if err != nil {
		t.Fatalf("failed to query active boot environment: %v", err)
	}

	var results []bootEnvSummary
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("failed to parse boot.environment.query response: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no active boot environment found; cannot determine clone source")
	}
	return results[0].ID
}

// TestAccBootEnvironment_basic clones the currently active boot environment,
// exercises the keep attribute, imports, and finally destroys only the
// clone. It never activates or destroys the boot environment it was cloned
// from.
func TestAccBootEnvironment_basic(t *testing.T) {
	// Resolve early and unconditionally skip (before any live API call) when
	// TF_ACC isn't set, so plain `go test` never dials a real TrueNAS host.
	acctest.PreCheck(t)

	sourceName := activeBootEnvironmentName(t)
	cloneName := acctest.RandName("tf-acc-be")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBootEnvironmentDestroyed(cloneName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccBootEnvironmentConfig(cloneName, sourceName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_boot_environment.test", "name", cloneName),
					resource.TestCheckResourceAttr("truenas_boot_environment.test", "source", sourceName),
					resource.TestCheckResourceAttr("truenas_boot_environment.test", "id", cloneName),
					resource.TestCheckResourceAttr("truenas_boot_environment.test", "keep", "true"),
					resource.TestCheckResourceAttr("truenas_boot_environment.test", "active", "false"),
					resource.TestCheckResourceAttrSet("truenas_boot_environment.test", "dataset"),
					resource.TestCheckResourceAttrSet("truenas_boot_environment.test", "used_bytes"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccBootEnvironmentConfig(cloneName, sourceName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_boot_environment.test", "keep", "false"),
				),
			},
			{
				ResourceName: "truenas_boot_environment.test",
				ImportState:  true,
				// "source" is client-side only (the clone origin) and is
				// never returned by the API, so it cannot be recovered by
				// import; state will be empty for it post-import.
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source"},
			},
		},
	})
}

// TestAccBootEnvironment_datasource verifies that the truenas_boot_environment
// datasource can look up a boot environment cloned by the resource in the
// same config.
func TestAccBootEnvironment_datasource(t *testing.T) {
	acctest.PreCheck(t)

	sourceName := activeBootEnvironmentName(t)
	cloneName := acctest.RandName("tf-acc-be-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckBootEnvironmentDestroyed(cloneName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccBootEnvironmentConfig(cloneName, sourceName, false) + `
data "truenas_boot_environment" "lookup" {
  name = truenas_boot_environment.test.name
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_boot_environment.lookup", "name", cloneName),
					resource.TestCheckResourceAttrSet("data.truenas_boot_environment.lookup", "dataset"),
				),
			},
		},
	})
}

func testAccBootEnvironmentConfig(name, source string, keep bool) string {
	return fmt.Sprintf(`
resource "truenas_boot_environment" "test" {
  name   = %q
  source = %q
  keep   = %v
}
`, name, source, keep)
}

// testAccCheckBootEnvironmentDestroyed verifies the clone was actually
// removed after the test finishes. It only ever queries/destroys the clone
// by name, never the boot environment it was cloned from.
func testAccCheckBootEnvironmentDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "boot.environment.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking boot environment %s: %v", name, err)
		}
		var results []bootEnvSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing boot.environment.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("boot environment %s still exists", name)
		}
		return nil
	}
}
