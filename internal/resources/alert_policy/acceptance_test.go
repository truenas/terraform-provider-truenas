// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// alertClassesAPI mirrors the JSON object returned by alertclasses.config.
type alertClassesAPI struct {
	Classes map[string]any `json:"classes"`
}

// readAlertPolicyOriginal reads the box's current classes map via
// alertclasses.config, so the test can restore it exactly afterward.
func readAlertPolicyOriginal(t *testing.T) map[string]any {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "alertclasses.config")
	if err != nil {
		t.Fatalf("error reading current alert policy: %v", err)
	}
	var orig alertClassesAPI
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing alertclasses.config response: %v", err)
	}
	if orig.Classes == nil {
		orig.Classes = map[string]any{}
	}
	return orig.Classes
}

// restoreAlertPolicy sends the original classes map back via
// alertclasses.update. It runs from t.Cleanup, so it restores the box even
// if the Terraform steps themselves fail partway through. Unlike the other
// singletons in this suite, truenas_alert_policy owns the WHOLE classes
// object, so this restores every class override the box had before the
// test ran, not just the one class this test touches.
func restoreAlertPolicy(t *testing.T, origClasses map[string]any) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "alertclasses.update", map[string]any{
		"classes": origClasses,
	}); err != nil {
		t.Fatalf("error restoring alert policy classes: %v", err)
	}
}

// classesToHCLString marshals a classes map to its canonical JSON form and
// escapes double quotes for embedding in an HCL string literal.
func classesToHCLString(t *testing.T, classes map[string]any) string {
	t.Helper()
	b, err := json.Marshal(classes)
	if err != nil {
		t.Fatalf("error marshaling classes: %v", err)
	}
	return strings.ReplaceAll(string(b), `"`, `\"`)
}

// classesJSONString marshals a classes map to its canonical (unescaped)
// JSON string, matching what truenas_alert_policy stores in state.
func classesJSONString(t *testing.T, classes map[string]any) string {
	t.Helper()
	b, err := json.Marshal(classes)
	if err != nil {
		t.Fatalf("error marshaling classes: %v", err)
	}
	return string(b)
}

// TestAccAlertPolicy_setAndRestore drives the singleton
// truenas_alert_policy resource's "classes" field through a one-class
// override (UPSBatteryLow: WARNING/DAILY) layered on top of the box's
// existing classes, and back to the exact classes map read from the box
// before the test ran, then imports it. It requires TF_ACC=1 and
// TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), since it mutates the
// box's live alert policy; a t.Cleanup-registered API restore is the
// safety net if the Terraform steps fail.
//
// Because truenas_alert_policy owns the WHOLE classes object (any class
// omitted from the JSON reverts to its default), the test steps carry
// forward the box's other existing class overrides unchanged rather than
// setting classes to a single-key object, so no other alert class is ever
// reset to its TrueNAS default by this test.
func TestAccAlertPolicy_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	origClasses := readAlertPolicyOriginal(t)
	t.Cleanup(func() { restoreAlertPolicy(t, origClasses) })

	testClasses := make(map[string]any, len(origClasses)+1)
	for k, v := range origClasses {
		testClasses[k] = v
	}
	testClasses["UPSBatteryLow"] = map[string]any{"level": "WARNING", "policy": "DAILY"}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAlertPolicyConfig(classesToHCLString(t, testClasses)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "id", "alert_policy"),
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "classes", classesJSONString(t, testClasses)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAlertPolicyConfig(classesToHCLString(t, origClasses)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "classes", classesJSONString(t, origClasses)),
				),
			},
			{
				ResourceName:      "truenas_alert_policy.test",
				ImportState:       true,
				ImportStateId:     "alert_policy",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAlertPolicyConfig(classes string) string {
	return fmt.Sprintf(`
resource "truenas_alert_policy" "test" {
  classes = "%s"
}
`, classes)
}
