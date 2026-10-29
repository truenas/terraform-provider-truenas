// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// noDisplayName fails when any result of the list block has the given name.
type noDisplayName struct{ list, name string }

func (c noDisplayName) CheckQuery(_ context.Context, req querycheck.CheckQueryRequest, resp *querycheck.CheckQueryResponse) {
	for _, r := range req.Query {
		if strings.TrimPrefix(r.Address, "list.") == c.list && r.DisplayName == c.name {
			resp.Error = fmt.Errorf("%s lists %q", c.list, c.name)
			return
		}
	}
}

// TestAccCertificate_listQuery: the list resource gave a CSR create_type =
// null (Required, so the generated configuration was invalid) and listed the
// system's truenas_default certificate (B-138).
func TestAccCertificate_listQuery(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-csr")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0), // list/query support requires Terraform 1.14.0+
		},
		CheckDestroy: testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + testAccCertificateCSRConfig(name)},
			{
				// No provider block: the prior step's .tf file is still on
				// disk (see TestAccUser_basic's Query step).
				Query: true,
				Config: `
list "truenas_certificate" "test" {
  provider         = truenas
  include_resource = true
  config {}
}
`,
				QueryResultChecks: []querycheck.QueryResultCheck{
					noDisplayName{list: "truenas_certificate.test", name: "truenas_default"},
					querycheck.ExpectResourceKnownValues("truenas_certificate.test",
						queryfilter.ByDisplayName(knownvalue.StringExact(name)),
						[]querycheck.KnownValueCheck{{
							Path:       tfjsonpath.New("create_type"),
							KnownValue: knownvalue.StringExact("CERTIFICATE_CREATE_CSR"),
						}}),
				},
			},
		},
	})
}
