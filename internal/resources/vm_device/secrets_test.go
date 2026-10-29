// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// checkDisplayPassword checks the DISPLAY password TrueNAS has stored for the
// device (vm.device.get_instance returns it to an admin session).
func checkDisplayPassword(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_vm_device.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().CallRead(context.Background(), "vm.device.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Attributes map[string]any `json:"attributes"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Attributes["password"] != want {
			return fmt.Errorf("stored password = %v, want %q", got.Attributes["password"], want)
		}
		return nil
	}
}

// noPasswordIn fails when attributes in state (or an import) carries a
// password key or the given secret.
func noPasswordIn(secret string) func(string) error {
	return func(v string) error {
		if strings.Contains(v, `"password"`) || strings.Contains(v, secret) {
			return fmt.Errorf("secret in attributes: %s", v)
		}
		return nil
	}
}

func testAccVMDeviceSecretConfig(vmName, resolution, attrsExtra, secret string, version int) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  vcpus     = 1
  autostart = false
  running   = false
}

resource "truenas_vm_device" "test" {
  vm = truenas_vm.test.id
  attributes = jsonencode({
    dtype      = "DISPLAY"
    type       = "SPICE"
    bind       = "0.0.0.0"
    resolution = %q
    wait       = false
    web        = false
    port       = 15940
    web_port   = 15941
%s  })
  attributes_secrets_wo         = jsonencode({ password = %q })
  attributes_secrets_wo_version = %d
}
`, vmName, resolution, attrsExtra, secret, version)
}

// TestAccVMDevice_secretsOutOfAttributes covers B-159 and B-141 for
// truenas_vm_device: a password inside attributes is a plan-time error;
// attributes (no longer Sensitive) never holds the stored password, not even
// after import; an update that does not change attributes_secrets_wo_version
// leaves the stored password alone (a changed overlay is not applied); bumping
// the version sends it.
func TestAccVMDevice_secretsOutOfAttributes(t *testing.T) {
	vmName := "tfaccvmdevsec" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	cfg := func(res, extra, secret string, version int) string {
		return acctest.ProviderConfig() + testAccVMDeviceSecretConfig(vmName, res, extra, secret, version)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckVMDeviceDestroyed,
		Steps: []resource.TestStep{
			{
				Config:      cfg("1024x768", "    password = \"tfwo-inline\"\n", "tfwo-pw-a", 1),
				ExpectError: regexp.MustCompile(`Secret in attributes`),
			},
			{
				Config: cfg("1024x768", "", "tfwo-pw-a", 1),
				Check: resource.ComposeTestCheckFunc(
					checkDisplayPassword("tfwo-pw-a"),
					resource.TestCheckResourceAttrWith("truenas_vm_device.test", "attributes", noPasswordIn("tfwo-pw-a")),
				),
			},
			{
				Config: cfg("1280x720", "", "tfwo-pw-b", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrWith("truenas_vm_device.test", "attributes", func(v string) error {
						if !strings.Contains(v, "1280x720") {
							return fmt.Errorf("resolution not updated: %s", v)
						}
						return nil
					}),
					checkDisplayPassword("tfwo-pw-a"),
				),
			},
			{Config: cfg("1280x720", "", "tfwo-pw-b", 2), Check: checkDisplayPassword("tfwo-pw-b")},
			{
				ResourceName: "truenas_vm_device.test",
				ImportState:  true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("imported %d states", len(states))
					}
					return noPasswordIn("tfwo-pw-b")(states[0].Attributes["attributes"])
				},
			},
		},
	})
}
