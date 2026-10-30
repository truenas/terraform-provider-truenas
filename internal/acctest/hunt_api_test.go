// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt (group "api"), NOT for commit.

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestHuntDSParity_Schema diffs every resource schema against the data source of
// the same type name: attributes only on one side, and attributes whose value
// type differs (a data source that reads the same object must yield the same
// shape). Offline.
func TestHuntDSParity_Schema(t *testing.T) {
	ctx := context.Background()
	p := provider.New("test")()
	pp := p.(interface {
		DataSources(context.Context) []func() datasource.DataSource
	})

	resTypes := map[string]map[string]attr.Type{}
	resAttrs := map[string]map[string]rschema.Attribute{}
	for _, ctor := range providerResources(ctx, p) {
		r := ctor()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sc fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sc)
		m := map[string]attr.Type{}
		for n, a := range sc.Schema.Attributes {
			m[n] = a.GetType()
		}
		for n, b := range sc.Schema.Blocks {
			m[n] = b.Type()
		}
		resTypes[md.TypeName] = m
		resAttrs[md.TypeName] = sc.Schema.Attributes
	}
	var out []string
	matched := 0
	var dsOnlyTypes []string
	for _, ctor := range pp.DataSources(ctx) {
		d := ctor()
		var md datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sc datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &sc)
		rt, ok := resTypes[md.TypeName]
		if !ok {
			dsOnlyTypes = append(dsOnlyTypes, md.TypeName)
			continue
		}
		matched++
		dt := map[string]attr.Type{}
		for n, a := range sc.Schema.Attributes {
			dt[n] = a.GetType()
		}
		for n, b := range sc.Schema.Blocks {
			dt[n] = b.Type()
		}
		var lines []string
		for n, ty := range rt {
			a := resAttrs[md.TypeName][n]
			wo := false
			sens := false
			if a != nil {
				wo = a.IsWriteOnly()
				sens = a.IsSensitive()
			}
			dty, ok := dt[n]
			if !ok {
				if !wo {
					lines = append(lines, fmt.Sprintf("    resource-only: %s (%s)%s", n, ty, map[bool]string{true: " [sensitive]"}[sens]))
				}
				continue
			}
			if !ty.Equal(dty) {
				lines = append(lines, fmt.Sprintf("    TYPE MISMATCH: %s resource=%s datasource=%s", n, ty, dty))
			}
		}
		for n, ty := range dt {
			if _, ok := rt[n]; !ok {
				lines = append(lines, fmt.Sprintf("    datasource-only: %s (%s)", n, ty))
			}
		}
		if len(lines) > 0 {
			sort.Strings(lines)
			out = append(out, md.TypeName+":\n"+strings.Join(lines, "\n"))
		}
	}
	sort.Strings(out)
	sort.Strings(dsOnlyTypes)
	t.Logf("matched resource/datasource pairs: %d; data sources with no same-named resource: %v", matched, dsOnlyTypes)
	t.Logf("\n%s", strings.Join(out, "\n"))
}

// TestHuntDatasetAlgorithmOn27Plus: on TrueNAS 27.0+ the provider silently drops
// encryption_algorithm from pool.dataset.create, but the attribute is
// Optional+Computed and reads back the server's fixed algorithm. A config that
// sets a non-default algorithm must either error clearly at plan/apply or be
// honored; it must not apply and then fail with "inconsistent result".
func TestHuntDatasetAlgorithmOn27Plus(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 27, 0) {
		t.Skip("needs TrueNAS 27.0+")
	}
	name := acctest.TestPool() + "/" + acctest.RandName("tfhuntapialg")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "enc" {
  name                  = %q
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-128-GCM"
  encryption_passphrase = "tfhunt-passphrase-1234"
}
`, name),
			// Provider rejects the field at plan time on 27.0+ (the API dropped it) —
			// the "error clearly" branch of the acceptance criteria above.
			ExpectError: regexp.MustCompile(`encryption_algorithm cannot be chosen on this TrueNAS release`),
		}},
	})
}

func TestHuntZvolAlgorithmOn27Plus(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 27, 0) {
		t.Skip("needs TrueNAS 27.0+")
	}
	name := acctest.TestPool() + "/" + acctest.RandName("tfhuntapizalg")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "enc" {
  name                  = %q
  volsize               = 16777216
  sparse                = true
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-128-GCM"
  encryption_passphrase = "tfhunt-passphrase-1234"
}
`, name),
			// Provider rejects the field at plan time on 27.0+ (the API dropped it) —
			// the "error clearly" branch of the acceptance criteria above.
			ExpectError: regexp.MustCompile(`encryption_algorithm cannot be chosen on this TrueNAS release`),
		}},
	})
}

// TestHuntUserWebshareBelow26: below TrueNAS 26.0 the provider used to drop
// webshare from user.create/update silently while the Optional+Computed
// attribute read back false, ending in "inconsistent result". Fixed: the
// provider now rejects webshare = true at plan time with a version message.
// Guard that fix (2026-10-30: was a stale reproducer expecting success).
func TestHuntUserWebshareBelow26(t *testing.T) {
	acctest.PreCheck(t)
	if acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("needs TrueNAS < 26.0")
	}
	name := acctest.RandName("tfhuntapiws")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_user" "u" {
  username          = %q
  full_name         = "tfhunt webshare"
  password_disabled = true
  home              = "/var/empty"
  smb               = false
  group_create      = true
  webshare          = true
}
`, name),
			ExpectError: regexp.MustCompile(`webshare requires TrueNAS 26\.0`),
		}},
	})
}

func TestHuntDSArgs(t *testing.T) {
	ctx := context.Background()
	p := provider.New("test")()
	pp := p.(interface {
		DataSources(context.Context) []func() datasource.DataSource
	})
	for _, ctor := range pp.DataSources(ctx) {
		d := ctor()
		var md datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sc datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &sc)
		var req, opt []string
		for n, a := range sc.Schema.Attributes {
			if a.IsRequired() {
				req = append(req, n+":"+a.GetType().String())
			} else if a.IsOptional() {
				opt = append(opt, n)
			}
		}
		sort.Strings(req)
		sort.Strings(opt)
		t.Logf("DSARGS %s req=%v opt=%v", md.TypeName, req, opt)
	}
}

// truenas_periodic_snapshot_task data source: used to fail every read with
// "Value Conversion Error" (struct-typed Computed schedule, Config.Get on a
// null schedule) before any API call. Fixed; a lookup of a dataset with no
// task must now be a clean not-found. (2026-10-30: stale reproducer -> guard.)
func TestHuntDSPeriodicSnapshotTask(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + `
data "truenas_periodic_snapshot_task" "d" {
  dataset = "` + acctest.TestPool() + `/tfhuntapi"
}
`,
			ExpectError: regexp.MustCompile(`no task found for dataset`),
		}},
	})
}

func TestHuntDSCloudsyncTask(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{Config: acctest.ProviderConfig() + `
data "truenas_cloudsync_task" "d" {
  description = "tfhuntapi-none"
}
`, ExpectError: regexp.MustCompile(`No cloud sync task with description`)}},
	})
}

func TestHuntDSCloudBackup(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{Config: acctest.ProviderConfig() + `
data "truenas_cloud_backup" "d" {
  description = "tfhuntapi-none"
}
`, ExpectError: regexp.MustCompile(`No cloud backup task with description`)}},
	})
}
