// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

type snmpCommunityLocation struct {
	Community string `json:"community"`
	Location  string `json:"location"`
}

func readSNMPCommunityLocation() (snmpCommunityLocation, error) {
	var got snmpCommunityLocation
	raw, err := acctest.Client().Call(context.Background(), "snmp.config")
	if err != nil {
		return got, err
	}
	err = json.Unmarshal(raw, &got)
	return got, err
}

func checkSNMPCommunity(want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := readSNMPCommunityLocation()
		if err != nil {
			return err
		}
		if got.Community != want {
			return fmt.Errorf("stored community is %q, want %q", got.Community, want)
		}
		return nil
	}
}

// TestAccSNMPConfig_communitySentOnlyOnVersionChange: an unrelated update with
// a changed community_wo and the same community_wo_version leaves the stored
// community alone; bumping the version sends it. (B-141)
func TestAccSNMPConfig_communitySentOnlyOnVersionChange(t *testing.T) {
	acctest.DisruptiveCheck(t)
	orig, err := readSNMPCommunityLocation()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "snmp.update", map[string]any{
			"community": orig.Community, "location": orig.Location,
		}); err != nil {
			t.Errorf("restore snmp community/location: %v", err)
		}
		if got, err := readSNMPCommunityLocation(); err != nil || got != orig {
			t.Errorf("snmp not restored: %+v (%v), want %+v", got, err, orig)
		}
	})
	cfg := func(location, community string, version int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_snmp_config" "test" {
  location             = %q
  community_wo         = %q
  community_wo_version = %d
}
`, location, community, version)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg("tf-acc-wo-1", "tfwocommA", 1), Check: checkSNMPCommunity("tfwocommA")},
			{
				Config: cfg("tf-acc-wo-2", "tfwocommB", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_snmp_config.test", "location", "tf-acc-wo-2"),
					checkSNMPCommunity("tfwocommA"),
				),
			},
			{Config: cfg("tf-acc-wo-2", "tfwocommB", 2), Check: checkSNMPCommunity("tfwocommB")},
		},
	})
}
