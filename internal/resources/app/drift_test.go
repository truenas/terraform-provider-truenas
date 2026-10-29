// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// deleteAppWithVolumes deletes an app together with its ix-volumes. The
// resource deletes with remove_ix_volumes=false on purpose (user data stays),
// which left an ix-apps/app_mounts/<name> dataset behind after every test run
// that the API cannot delete afterwards (B-174). A not-found app is fine.
func deleteAppWithVolumes(t *testing.T, name string) {
	t.Helper()
	_, err := acctest.Client().CallJob(context.Background(), "app.delete", name, map[string]any{
		"remove_images":     true,
		"remove_ix_volumes": true,
	})
	if err != nil {
		t.Logf("app.delete %s with ix-volumes: %v", name, err)
	}
}

// appCleanupStep is the last step of an app test: it deletes the app with its
// ix-volumes out of band, then applies rest (the config without the app), so
// the refresh drops the app from state and the destroy has nothing left to do.
func appCleanupStep(t *testing.T, name, rest string) resource.TestStep {
	return resource.TestStep{
		PreConfig: func() { deleteAppWithVolumes(t, name) },
		Config:    acctest.ProviderConfig() + rest,
	}
}

// hostPathDataset is a dataset for a flame host_path mount.
func hostPathDataset(name string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "hp" {
  name = "%s/%s"
}
`, acctest.TestPool(), name)
}

// TestAccApp_valuesListOfObjects guards B-094 and B-137: the server fills
// defaults into each item of a list of objects in values (read_only,
// host_path_config.acl_enable), which drifted forever; and a values update
// planned state/human_version/upgrade_available as unknown.
func TestAccApp_valuesListOfObjects(t *testing.T) {
	name := acctest.RandName("tf-acc-applst")
	vals := func(pw string) string {
		return fmt.Sprintf(`jsonencode({
    flame   = { password = %q }
    network = { web_port = { port_number = 30981 } }
    storage = { additional_storage = [{ type = "host_path", mount_path = "/tfh", host_path_config = { path = truenas_dataset.hp.mountpoint } }] }
  })`, pw)
	}
	cfg := func(pw string) string {
		return acctest.ProviderConfig() + hostPathDataset(name) + flameAppConfig(name, "", vals(pw))
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg("x")},
			{Config: cfg("x"), ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// A values update keeps the read-only attributes known.
			{Config: cfg("y"), ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_app.test", plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue("truenas_app.test", tfjsonpath.New("state"), knownvalue.StringExact("RUNNING")),
					plancheck.ExpectKnownValue("truenas_app.test", tfjsonpath.New("human_version"), knownvalue.NotNull()),
					plancheck.ExpectKnownValue("truenas_app.test", tfjsonpath.New("upgrade_available"), knownvalue.NotNull()),
				}}},
			appCleanupStep(t, name, hostPathDataset(name)),
		},
	})
}

// TestAccApp_valuesHeredoc guards B-095: values written as a heredoc (not
// jsonencode output) drifted forever because Read re-marshaled the document.
func TestAccApp_valuesHeredoc(t *testing.T) {
	name := acctest.RandName("tf-acc-apphd")
	vals := "<<EOT\n{\n  \"network\": {\"web_port\": {\"port_number\": 30982}},\n  \"flame\": {\"password\": \"x\"}\n}\nEOT"
	cfg := acctest.ProviderConfig() + flameAppConfig(name, "", vals)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			appCleanupStep(t, name, ""),
		},
	})
}

// TestAccApp_versionLatest guards B-096: version = "latest" (the documented
// default) read back as the resolved version, so the apply failed.
func TestAccApp_versionLatest(t *testing.T) {
	name := acctest.RandName("tf-acc-appvl")
	vals := `jsonencode({ flame = { password = "x" }, network = { web_port = { port_number = 30983 } } })`
	cfg := acctest.ProviderConfig() + flameAppConfig(name, `  version = "latest"`, vals)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_app.test", "version", "latest"),
				resource.TestCheckResourceAttrSet("truenas_app.test", "human_version"),
			)},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			appCleanupStep(t, name, ""),
		},
	})
}

// TestAccApp_composeYAML11 guards B-097: TrueNAS parses Compose with YAML 1.1
// (yes/off are booleans, 0755 is octal, 1:30 is 90), so a Compose using them
// drifted forever when compared with YAML 1.2 rules.
func TestAccApp_composeYAML11(t *testing.T) {
	name := acctest.RandName("tf-acc-appyl")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name       = %q
  custom_app = true
  custom_compose_config_string = <<-EOT
    services:
      a:
        image: busybox:1.36
        command: ["sleep", "100000"]
        environment:
          FOO: yes
          BAR: off
          BAZ: 0755
          QUX: 1:30
  EOT
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			appCleanupStep(t, name, ""),
		},
	})
}

// TestAccApp_dockerStoppedRefresh guards B-044: while Docker is stopped,
// app.get_instance reports every app as not found, and a refresh dropped the
// app from state (the next apply then tried to create it again). Stopping
// Docker stops every app on the box, so the test is disruptive.
func TestAccApp_dockerStoppedRefresh(t *testing.T) {
	name := acctest.RandName("tf-acc-appds")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name       = %q
  custom_app = true
  custom_compose_config_string = <<-EOT
    services:
      a:
        image: busybox:1.36
        command: ["sleep", "100000"]
  EOT
}
`, name)
	docker := func(action string) func() {
		return func() {
			if _, err := acctest.Client().CallJob(context.Background(), "service.control", action, "docker"); err != nil {
				t.Logf("service.control %s docker: %v", action, err)
			}
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.AppsCheck(t)
			acctest.DisruptiveCheck(t)
			t.Cleanup(docker("START"))
		},
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			// Docker stopped: the refresh keeps the app, so the plan is empty.
			{PreConfig: docker("STOP"), Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{PreConfig: docker("START"), Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			appCleanupStep(t, name, ""),
		},
	})
}
