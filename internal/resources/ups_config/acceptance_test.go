package ups_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccUPSConfigDataSource_basic reads the current TrueNAS UPS
// configuration through the truenas_ups_config datasource only. It never
// writes: UPS is a system-critical power management singleton, and the
// box's real UPS configuration must not be overwritten by an acceptance
// test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccUPSConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_ups_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_ups_config.test", "id", "ups_config"),
					resource.TestCheckResourceAttrSet("data.truenas_ups_config.test", "mode"),
				),
			},
		},
	})
}

// upsConfigOriginal captures the one field TestAccUPSConfig_setAndRestore
// touches.
type upsConfigOriginal struct {
	Description string `json:"description"`
}

// readUPSConfigOriginal reads the box's current description via ups.config,
// so the test can restore it exactly afterward.
func readUPSConfigOriginal(t *testing.T) upsConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "ups.config")
	if err != nil {
		t.Fatalf("error reading current ups config: %v", err)
	}
	var orig upsConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing ups.config response: %v", err)
	}
	return orig
}

// restoreUPSConfig sends only description back to its original value via
// ups.update. It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through.
func restoreUPSConfig(t *testing.T, orig upsConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "ups.update", map[string]any{
		"description": orig.Description,
	}); err != nil {
		t.Fatalf("error restoring ups description: %v", err)
	}
}

// TestAccUPSConfig_setAndRestore drives the singleton truenas_ups_config
// resource's "description" field (a cosmetic, low-risk descriptive string)
// through a test value and back to the value read from the box before the
// test ran, then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live UPS
// configuration; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail.
func TestAccUPSConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readUPSConfigOriginal(t)
	t.Cleanup(func() { restoreUPSConfig(t, orig) })

	testValue := acctest.RandName("tf-acc-description")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUPSConfigConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ups_config.test", "id", "ups_config"),
					resource.TestCheckResourceAttr("truenas_ups_config.test", "description", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccUPSConfigConfig(orig.Description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ups_config.test", "description", orig.Description),
				),
			},
			{
				ResourceName:            "truenas_ups_config.test",
				ImportState:             true,
				ImportStateId:           "ups_config",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"monpwd"},
			},
		},
	})
}

func testAccUPSConfigConfig(description string) string {
	return fmt.Sprintf(`
resource "truenas_ups_config" "test" {
  description = %q
}
`, description)
}
