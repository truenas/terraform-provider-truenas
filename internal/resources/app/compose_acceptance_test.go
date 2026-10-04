// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccApp_composeReadBack uses only a disposable custom app with no ports
// or volumes. app.config returns its complete Compose object, including plain
// environment values (probed on TrueNAS 25.10.6). TF_ACC and TRUENAS_APPS gate
// the real create/update/delete lifecycle and image pulls.
func TestAccApp_composeReadBack(t *testing.T) {
	name := acctest.RandName("tf-acc-compose")
	compose := testAccCompose("busybox:1.37.0", "original")
	// JSON is also YAML. Use it for exact import verification; subsequent
	// steps switch to commented YAML to verify formatting survives refresh.
	cfg := testAccComposeConfig(name, compose)
	yaml := "# preserve my formatting\nservices:\n  test:\n    image: busybox:1.37.0\n    command: [sleep, infinity]\n    network_mode: none\n    environment:\n      TOKEN: synthetic-token\n    labels:\n      example.test/revision: original\n"
	yamlCfg := testAccComposeConfig(name, yaml)
	check := func(expected string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			actual := s.RootModule().Resources["truenas_app.test"].Primary.Attributes["custom_compose_config_string"]
			if actual != expected {
				return fmt.Errorf("Compose state did not match the expected document")
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg, Check: check(compose)},
			{ResourceName: "truenas_app.test", ImportState: true, ImportStateVerify: true},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: yamlCfg, Check: check(yaml)},
			{Config: yamlCfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				PreConfig: func() {
					// Keep the fixture isolated; change both an image and another key to
					// exercise full-document read-back rather than image-only detection.
					if _, err := acctest.Client().CallJob(context.Background(), "app.update", name,
						map[string]any{"custom_compose_config_string": testAccCompose("busybox:1.36.1", "external")}); err != nil {
						t.Fatal("out-of-band update of the disposable app failed")
					}
				},
				Config: yamlCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_app.test", plancheck.ResourceActionUpdate),
				}},
				Check: check(yaml),
			},
			{Config: yamlCfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

func testAccCompose(image, revision string) string {
	document := map[string]any{"services": map[string]any{"test": map[string]any{
		"image": image, "command": []string{"sleep", "infinity"}, "network_mode": "none",
		"environment": map[string]string{"TOKEN": "synthetic-token"},
		"labels":      map[string]string{"example.test/revision": revision},
	}}}
	encoded, _ := json.Marshal(document)
	return string(encoded)
}

func testAccComposeConfig(name, compose string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name = %q
  custom_app = true
  running = true
  custom_compose_config_string = %q
}
`, name, compose)
}
