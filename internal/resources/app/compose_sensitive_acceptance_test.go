// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccApp_composeSensitivePlan checks Terraform's rendered plan, not just
// sensitivity metadata. The CLI is required on PATH or TF_ACC_TERRAFORM_PATH.
// Like the other app acceptance tests, this uses a real disposable app and is
// gated by TF_ACC and TRUENAS_APPS. JSON plans/state contain plaintext by design;
// this test deliberately never prints them, even on failure.
func TestAccApp_composeSensitivePlan(t *testing.T) {
	acctest.AppsCheck(t)
	terraform := os.Getenv("TF_ACC_TERRAFORM_PATH")
	if terraform == "" {
		var err error
		terraform, err = exec.LookPath("terraform")
		if err != nil {
			t.Skip("install Terraform on PATH or set TF_ACC_TERRAFORM_PATH for rendered-plan coverage")
		}
	}
	dir := t.TempDir()
	binDir := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "terraform-provider-truenas"), ".")
	build.Dir = "../../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build provider: %v\n%s", err, out)
	}
	write := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("dev.tfrc", fmt.Sprintf(`provider_installation {
  dev_overrides { "truenas/truenas" = %q }
  direct {}
}
`, binDir))
	var env []string
	for _, entry := range os.Environ() {
		// Isolate CLI settings, logging, workspace, and reattach configuration.
		if !strings.HasPrefix(entry, "TF_") {
			env = append(env, entry)
		}
	}
	env = append(env, "TF_CLI_CONFIG_FILE="+filepath.Join(dir, "dev.tfrc"), "TF_IN_AUTOMATION=1")
	run := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, terraform, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	name := acctest.RandName("tf-acc-secret")
	original := acctest.RandName("synthetic-original")
	rotated := acctest.RandName("synthetic-rotated")
	drifted := acctest.RandName("synthetic-drifted")
	config := func(token string) string {
		return `terraform {
  required_providers { truenas = { source = "truenas/truenas" } }
}
` + acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app" "test" {
  name = %q
  custom_app = true
  running = true
  custom_compose_config_string = jsonencode({
    services = { test = {
      image = "busybox:1.37.0"
      command = ["sleep", "infinity"]
      network_mode = "none"
      environment = { TOKEN = sensitive(%q) }
    } }
  })
}
`, name, token)
	}
	assertRedacted := func(rendered string) {
		t.Helper()
		for _, token := range []string{original, rotated, drifted} {
			if strings.Contains(rendered, token) {
				t.Fatal("sensitive Compose value leaked into rendered plan")
			}
		}
		if !strings.Contains(rendered, "custom_compose_config_string") || !strings.Contains(rendered, "(sensitive value)") {
			t.Fatal("rendered plan did not show a redacted Compose change")
		}
		if !strings.Contains(rendered, name) {
			t.Fatal("rendered plan lost the public app name")
		}
	}
	plan := func() {
		t.Helper()
		out, err := run("plan", "-no-color", "-input=false", "-out=compose.tfplan")
		if err != nil {
			t.Fatalf("terraform plan failed: %v (output withheld)", err)
		}
		assertRedacted(out)
		out, err = run("show", "-no-color", "compose.tfplan")
		if err != nil {
			t.Fatalf("terraform show failed: %v (output withheld)", err)
		}
		assertRedacted(out)
	}
	write("main.tf", config(original))
	plan()
	// Register cleanup before apply, including partial-create failures.
	t.Cleanup(func() {
		if _, err := run("destroy", "-auto-approve", "-input=false", "-no-color"); err != nil {
			t.Errorf("destroy disposable app %s: %v (output withheld)", name, err)
		}
		if err := testAccCheckAppDestroyed(name)(nil); err != nil {
			t.Error(err)
		}
	})
	if _, err := run("apply", "-input=false", "-no-color", "compose.tfplan"); err != nil {
		t.Fatalf("apply disposable app: %v (output withheld)", err)
	}
	write("main.tf", config(rotated))
	plan()
	// app.config now returns a different plaintext token. Refresh must retain
	// the input's sensitivity when displaying the proposed correction.
	external := strings.ReplaceAll(testAccCompose("busybox:1.37.0", "external"), "synthetic-token", drifted)
	if _, err := acctest.Client().CallJob(context.Background(), "app.update", name,
		map[string]any{"custom_compose_config_string": external}); err != nil {
		t.Fatal("out-of-band update of the disposable app failed")
	}
	plan()
	// Prove read-back actually observed the drift, rather than a stale state
	// accidentally making the redaction assertions pass. JSON is intentionally
	// unredacted and must not be used as evidence of safe human-readable output.
	raw, err := run("show", "-json", "compose.tfplan")
	if err != nil {
		t.Fatalf("read JSON plan: %v (output withheld)", err)
	}
	var saved struct {
		Changes []struct {
			Address string                                 `json:"address"`
			Change  struct{ Before, After map[string]any } `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		t.Fatal("invalid JSON plan")
	}
	for _, change := range saved.Changes {
		if change.Address != "truenas_app.test" {
			continue
		}
		before, _ := change.Change.Before["custom_compose_config_string"].(string)
		after, _ := change.Change.After["custom_compose_config_string"].(string)
		if !strings.Contains(before, drifted) || !strings.Contains(after, rotated) {
			t.Fatal("plan did not reconcile the live token with the configured secret")
		}
		return
	}
	t.Fatal("app change missing from plan")
}
