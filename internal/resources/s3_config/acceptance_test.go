// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package s3_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/resources/s3_config"
)

// s3Check skips when the connected release has no S3 API (s3.* exists from
// TrueNAS 27.0; of the lab boxes only 28.0 has it).
func s3Check(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	if _, err := acctest.Client().CallRead(context.Background(), "s3.config"); err != nil {
		if s3_config.IsUnavailable(err) {
			t.Skip("s3.config does not exist on this box: the S3 API requires TrueNAS 27.0 or later")
		}
		t.Fatalf("s3.config: %v", err)
	}
}

// restoreS3Config reads s3.config now and writes the listed keys back when
// the test ends, so the box's S3 service (which may be in use: on .70 it
// serves space/s3demo) is left as found. Only the keys a test writes are
// restored: default_audit is an Enterprise-licensed feature that s3.update
// rejects on an unlicensed system even unchanged.
func restoreS3Config(t *testing.T, keys ...string) map[string]any {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "s3.config")
	if err != nil {
		t.Fatalf("read s3.config: %v", err)
	}
	var orig map[string]any
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("parse s3.config: %v", err)
	}
	t.Cleanup(func() {
		p := map[string]any{}
		for _, k := range keys {
			if v, ok := orig[k]; ok {
				p[k] = v
			}
		}
		// global_grants rows carry a read-only "name"; strip it.
		if gs, ok := p["global_grants"].([]any); ok {
			for _, g := range gs {
				if gm, ok := g.(map[string]any); ok {
					delete(gm, "name")
				}
			}
		}
		if _, err := acctest.RestoreCall(context.Background(), "s3.update", p); err != nil {
			t.Errorf("restore s3.config %v: %v", p, err)
		}
	})
	return orig
}

// TestAccS3ConfigDataSource_basic reads the configuration only.
func TestAccS3ConfigDataSource_basic(t *testing.T) {
	s3Check(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + `data "truenas_s3_config" "test" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.truenas_s3_config.test", "id", "s3_config"),
				resource.TestCheckResourceAttrSet("data.truenas_s3_config.test", "servers"),
				resource.TestCheckResourceAttrSet("data.truenas_s3_config.test", "log_level"),
			),
		}},
	})
}

// TestAccS3Config_setAndRestore writes every attribute (listeners on a test
// port, grants for a test group, a test managed_root_dataset), updates them,
// imports, and restores the original configuration afterwards.
func TestAccS3Config_setAndRestore(t *testing.T) {
	s3Check(t)
	acctest.DisruptiveCheck(t) // listeners restart the service
	restoreS3Config(t, "listeners", "servers", "certificate", "region", "log_level", "global_grants", "managed_root_dataset")
	ds := acctest.TestPool() + "/" + acctest.RandName("tfaccs3root")
	group := acctest.RandName("tfaccs3g")
	cfg := func(port int, region, logLevel, access string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
data "truenas_certificate" "ui" {
  name = "truenas_default"
}
resource "truenas_dataset" "root" {
  name = %q
}
resource "truenas_group" "g" {
  name = %q
}
resource "truenas_s3_config" "test" {
  listeners   = [{ address = "0.0.0.0", port = %d, tls = false }]
  servers     = 2
  certificate = data.truenas_certificate.ui.id
  region      = %q
  log_level   = %q
  global_grants = [
    { principal_type = "GROUP", xid = truenas_group.g.gid, access = %q },
    { principal_type = "EVERYONE", access = "READONLY" },
  ]
  managed_root_dataset = truenas_dataset.root.name
}
`, ds, group, port, region, logLevel, access)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(9001, "eu-west-1", "INFO", "READWRITE"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_s3_config.test", "id", "s3_config"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "listeners.0.port", "9001"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "servers", "2"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "region", "eu-west-1"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "log_level", "INFO"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "global_grants.#", "2"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "global_grants.1.principal_type", "EVERYONE"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "managed_root_dataset", ds),
				),
			},
			{Config: cfg(9001, "eu-west-1", "INFO", "READWRITE"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config: cfg(9002, "", "NOTICE", "DENY"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_s3_config.test", "listeners.0.port", "9002"),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "region", ""),
					resource.TestCheckResourceAttr("truenas_s3_config.test", "global_grants.0.access", "DENY"),
				),
			},
			{Config: cfg(9002, "", "NOTICE", "DENY"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: cfg(9002, "", "NOTICE", "DENY"),
				ResourceName: "truenas_s3_config.test", ImportState: true, ImportStateId: "s3_config", ImportStateVerify: true},
		},
	})
}

// s3AuditLicensed reports whether this system's license includes S3 audit
// logging: an empty default_audit is always accepted, a non-empty one is
// refused on an unlicensed system, so the probe sets one action and puts the
// original back when that succeeded.
func s3AuditLicensed(t *testing.T) bool {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "s3.config")
	if err != nil {
		t.Fatalf("s3.config: %v", err)
	}
	var cur struct {
		DefaultAudit json.RawMessage `json:"default_audit"`
	}
	if err := json.Unmarshal(raw, &cur); err != nil {
		t.Fatal(err)
	}
	_, err = acctest.RestoreCall(context.Background(), "s3.update", map[string]any{"default_audit": []string{"ListBucket"}})
	if err != nil && strings.Contains(err.Error(), "license") {
		return false
	}
	if err != nil {
		t.Fatalf("s3.update default_audit probe: %v", err)
	}
	var v any
	_ = json.Unmarshal(cur.DefaultAudit, &v)
	if _, err := acctest.RestoreCall(context.Background(), "s3.update", map[string]any{"default_audit": v}); err != nil {
		t.Fatalf("restore default_audit after probe: %v", err)
	}
	return true
}

// TestAccS3Config_audit exercises default_audit / default_audit_overflow,
// including ["ALL"] (the API's "ALL" string). S3 audit logging is an
// Enterprise-licensed feature: on an unlicensed system the apply is refused
// with the license error and nothing else is touched.
func TestAccS3Config_audit(t *testing.T) {
	s3Check(t)
	cfg := func(audit, overflow string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_s3_config" "test" {
  default_audit          = %s
  default_audit_overflow = %q
}
`, audit, overflow)
	}
	if !s3AuditLicensed(t) {
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
			Steps: []resource.TestStep{{
				Config:      cfg(`["PutObject", "DeleteObject"]`, "DROP"),
				ExpectError: regexp.MustCompile(`license does not include the S3 audit`),
			}},
		})
		return
	}
	restoreS3Config(t, "default_audit", "default_audit_overflow")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`["PutObject", "DeleteObject"]`, "DROP"), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_s3_config.test", "default_audit.#", "2"),
				resource.TestCheckResourceAttr("truenas_s3_config.test", "default_audit_overflow", "DROP"))},
			{Config: cfg(`["ALL"]`, "BACKPRESSURE"), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_s3_config.test", "default_audit.#", "1"),
				resource.TestCheckResourceAttr("truenas_s3_config.test", "default_audit.0", "ALL"),
				resource.TestCheckResourceAttr("truenas_s3_config.test", "default_audit_overflow", "BACKPRESSURE"))},
			{Config: cfg(`["ALL"]`, "BACKPRESSURE"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
