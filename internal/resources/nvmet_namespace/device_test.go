// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace_test

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// nsFixtures declares a subsystem, two zvols (a, b) and a dataset (d) for a
// namespace to switch between.
func nsFixtures(base string) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "s" {
  name = "%[2]s"
}
resource "truenas_zvol" "a" {
  name    = "%[1]s/%[2]s-a"
  volsize = 67108864
}
resource "truenas_zvol" "b" {
  name    = "%[1]s/%[2]s-b"
  volsize = 67108864
}
resource "truenas_dataset" "d" {
  name = "%[1]s/%[2]s-d"
}
`, acctest.TestPool(), base)
}

func nsDeviceCfg(base, dev, extra string) string {
	return acctest.ProviderConfig() + nsFixtures(base) + fmt.Sprintf(`
resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.s.id
  device_path = %s
%s
}
`, dev, extra)
}

// checkNSServerEnabled asserts the server-side namespace is enabled, so a
// device switch that disables the namespace first also re-enables it.
func checkNSServerEnabled(t *testing.T) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources["truenas_nvmet_namespace.test"].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		inst := acctest.GetInstance(t, "nvmet.namespace", id)
		if inst["enabled"] != true {
			return fmt.Errorf("server namespace enabled = %v, want true", inst["enabled"])
		}
		return nil
	}
}

// deviceSwitchSteps re-points an enabled namespace at another zvol, then at
// a file, checking each change applies in place (device_uuid kept, namespace
// re-enabled) and re-plans empty. extraZvol/extraFile are the device_type
// lines, empty to exercise the derived device_type.
func deviceSwitchSteps(t *testing.T, base, extraZvol, extraFile string) []resource.TestStep {
	var uuid string
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	update := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction("truenas_nvmet_namespace.test", plancheck.ResourceActionUpdate),
	}}
	sameUUID := resource.TestCheckResourceAttrWith("truenas_nvmet_namespace.test", "device_uuid", func(v string) error {
		if v != uuid {
			return fmt.Errorf("device_uuid = %q, want %q (namespace must be updated in place)", v, uuid)
		}
		return nil
	})
	zvolA := nsDeviceCfg(base, `"zvol/${truenas_zvol.a.name}"`, extraZvol)
	zvolB := nsDeviceCfg(base, `"zvol/${truenas_zvol.b.name}"`, extraZvol)
	file := nsDeviceCfg(base, `"${truenas_dataset.d.mountpoint}/ns.img"`, extraFile+"\n  filesize = 67108864")
	return []resource.TestStep{
		{
			Config: zvolA,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_type", "ZVOL"),
				resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "true"),
				resource.TestCheckResourceAttrWith("truenas_nvmet_namespace.test", "device_uuid", func(v string) error {
					uuid = v
					return nil
				}),
			),
		},
		{Config: zvolA, ConfigPlanChecks: empty},
		{ResourceName: "truenas_nvmet_namespace.test", ImportState: true, ImportStateVerify: true},
		{Config: zvolA, ConfigPlanChecks: empty},
		{
			Config:           zvolB,
			ConfigPlanChecks: update,
			Check: resource.ComposeTestCheckFunc(
				resource.TestMatchResourceAttr("truenas_nvmet_namespace.test", "device_path", regexp.MustCompile(`^zvol/.*-b$`)),
				sameUUID,
				checkNSServerEnabled(t),
			),
		},
		{Config: zvolB, ConfigPlanChecks: empty},
		{
			Config:           file,
			ConfigPlanChecks: update,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_type", "FILE"),
				sameUUID,
				checkNSServerEnabled(t),
			),
		},
		{Config: file, ConfigPlanChecks: empty},
	}
}

// TestAccNVMetNamespace_changeDevice omits device_type. The API requires it
// (no server default), so create failed; the provider now derives it from
// device_path, including when device_path switches from a zvol to a file.
// The switches run on an enabled namespace, which TrueNAS only allows after
// disabling it ("Disable first to allow change").
func TestAccNVMetNamespace_changeDevice(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    deviceSwitchSteps(t, acctest.RandName("tffixns"), "", ""),
	})
}

// TestAccNVMetNamespace_changeDeviceTyped is changeDevice with device_type
// stated, so the zvol-to-file switch changes device_type and device_path
// together on an enabled namespace.
func TestAccNVMetNamespace_changeDeviceTyped(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    deviceSwitchSteps(t, acctest.RandName("tffixnst"), `  device_type = "ZVOL"`, `  device_type = "FILE"`),
	})
}

// TestAccNVMetNamespace_devZvolForm: a /dev/zvol/... device_path without
// device_type is derived as ZVOL, so the server reports the path-form error
// instead of a misleading FILE error.
func TestAccNVMetNamespace_devZvolForm(t *testing.T) {
	base := acctest.RandName("tffixnsdev")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      nsDeviceCfg(base, `"/dev/zvol/${truenas_zvol.a.name}"`, ""),
				ExpectError: regexp.MustCompile(`must start with "zvol/"`),
			},
		},
	})
}
