// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccApp_basic installs the "syncthing" catalog app with default values,
// verifies computed attributes are populated, stops it via the "running"
// attribute, and finally destroys it.
//
// Gated behind TRUENAS_APPS=1 (acctest.AppsCheck) since it pulls container
// images from the network and can be slow/bandwidth-heavy, in addition to
// TF_ACC=1 and credentials via TRUENAS_API_KEY or
// TRUENAS_USERNAME+TRUENAS_PASSWORD.
func TestAccApp_basic(t *testing.T) {
	// App names may have length limits; keep the RandName-suffixed name
	// comfortably under 40 chars.
	name := acctest.RandName("tf-acc-app")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: append([]resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAppConfig(name, "syncthing", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "name", name),
					resource.TestCheckResourceAttr("truenas_app.test", "catalog_app", "syncthing"),
					resource.TestCheckResourceAttr("truenas_app.test", "id", name),
					resource.TestCheckResourceAttr("truenas_app.test", "running", "true"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "state"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "version"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAppConfig(name, "syncthing", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_app.test", "state", "STOPPED"),
				),
			},
			// Import then plan the same config: catalog_app is read from metadata.name, so the plan is empty (BUGS B-006).
		}, acctest.ImportReapplyNoop("truenas_app.test", acctest.ProviderConfig()+testAccAppConfig(name, "syncthing", false))...),
	})
}

// TestAccApp_datasource verifies that the truenas_app datasource can look up
// an app created by the resource in the same config. Gated behind
// TRUENAS_APPS=1 like TestAccApp_basic, since it also installs a real app.
func TestAccApp_datasource(t *testing.T) {
	name := acctest.RandName("tf-acc-app-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAppConfig(name, "syncthing", true) + `
data "truenas_app" "lookup" {
  name = truenas_app.test.name
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_app.lookup", "name", name),
					resource.TestCheckResourceAttrSet("data.truenas_app.lookup", "state"),
				),
			},
		},
	})
}

func testAccAppConfig(name, catalogApp string, running bool) string {
	return fmt.Sprintf(`
resource "truenas_app" "test" {
  name        = %q
  catalog_app = %q
  running     = %v
}
`, name, catalogApp, running)
}

func testAccCheckAppDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "app.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking app %s: %v", name, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing app.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("app %s still exists", name)
		}
		return nil
	}
}

func testAccAppValuesConfig(name, tz string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name        = %q
  catalog_app = "syncthing"
  running     = true
  values      = jsonencode({ TZ = %q })
}
`, name, tz)
}

// TestAccApp_valuesDriftDetection guards GH #33: `values` is reconciled from the
// live config on read, projected onto the keys the user set, so drift in a
// user-set key is detected — while chart defaults and server-managed ix_* keys
// do NOT show as drift. Gated behind TRUENAS_APPS=1 (installs a real app).
func TestAccApp_valuesDriftDetection(t *testing.T) {
	name := acctest.RandName("tf-acc-appdrift")
	cfg := testAccAppValuesConfig(name, "Etc/UTC")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check:  resource.TestCheckResourceAttrSet("truenas_app.test", "values"),
			},
			// No out-of-band change: the plan must be empty. This is the key
			// guard that chart defaults and ix_* keys are not reported as drift.
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Change a user-set key out of band, then re-plan the same config:
			// the drift must be detected as an in-place update back to Etc/UTC.
			{
				PreConfig: func() {
					if _, err := acctest.Client().CallJob(context.Background(), "app.update", name,
						map[string]any{"values": map[string]any{"TZ": "America/New_York"}}); err != nil {
						t.Fatalf("out-of-band app.update failed: %v", err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_app.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func testAccAppComposeConfig(name, image string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name       = %q
  custom_app = true
  running    = true

  custom_compose_config_string = <<-YAML
    services:
      example:
        image: %s
        command: ["sleep", "infinity"]
        environment:
          FOO: bar
          SECRET: s3cr3t
  YAML
}
`, name, image)
}

// TestAccApp_composeConfigDriftDetection guards GH #34: a custom app's
// custom_compose_config_string is reconciled from the live app.config on read,
// so drift (an edit made in the UI/API) is detected, while formatting-only
// differences are not. Gated behind TRUENAS_APPS=1 (installs a real app).
func TestAccApp_composeConfigDriftDetection(t *testing.T) {
	name := "tfacccompose" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	cfg := testAccAppComposeConfig(name, "busybox:1.36")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check:  resource.TestCheckResourceAttrSet("truenas_app.test", "custom_compose_config_string"),
			},
			// No out-of-band change: plan must be empty (formatting is not drift).
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Change the image out of band, then re-plan the same config: the
			// drift must be detected as an in-place update back to busybox:1.36.
			{
				PreConfig: func() {
					newCompose := "services:\n  example:\n    image: busybox:1.35\n    command: [\"sleep\", \"infinity\"]\n"
					if _, err := acctest.Client().CallJob(context.Background(), "app.update", name,
						map[string]any{"custom_compose_config_string": newCompose}); err != nil {
						t.Fatalf("out-of-band app.update failed: %v", err)
					}
				},
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_app.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func testAccAppComposeWOConfig(name, image string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name       = %q
  custom_app = true
  running    = true

  # Base Compose with NO secret; the secret env var is supplied write-only.
  custom_compose_config_string = <<-YAML
    services:
      example:
        image: %s
        command: ["sleep", "infinity"]
        environment:
          FOO: bar
  YAML

  custom_compose_config_string_wo         = jsonencode({ services = { example = { environment = { SECRET = "tfacc-wo-compose-secret" } } } })
  custom_compose_config_string_wo_version = 1
}
`, name, image)
}

// TestAccApp_composeSecretsWO guards the write-only Compose overlay: the secret
// env var is supplied via custom_compose_config_string_wo, deep-merged into the
// Compose server-side, and must NOT appear in state — and the non-secret base
// still plans clean (no drift).
func TestAccApp_composeSecretsWO(t *testing.T) {
	name := "tfacccomposewo" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	cfg := testAccAppComposeWOConfig(name, "busybox:1.36")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("truenas_app.test", "custom_compose_config_string_wo"),
					resource.TestCheckResourceAttrWith("truenas_app.test", "custom_compose_config_string", func(v string) error {
						if strings.Contains(v, "tfacc-wo-compose-secret") {
							return fmt.Errorf("secret leaked into custom_compose_config_string state: %s", v)
						}
						return nil
					}),
				),
			},
			// No change: base plans clean (the secret, present live, is projected out).
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
