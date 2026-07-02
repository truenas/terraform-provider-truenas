package zvol

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// TestZvolSchema verifies the resource schema declares volsize as Required.
func TestZvolSchema(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["volsize"]
	if !ok {
		t.Fatal("schema missing 'volsize' attribute")
	}

	i64, ok := attr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("volsize attribute is %T, want schema.Int64Attribute", attr)
	}
	if !i64.IsRequired() {
		t.Error("volsize attribute should be Required")
	}
}

// TestZvolAPIPayload verifies the apiPayload helper produces the correct structure.
func TestZvolAPIPayload(t *testing.T) {
	m := &ZvolModel{
		ID:           types.StringValue("tank/myvol"),
		Name:         types.StringValue("tank/myvol"),
		VolSize:      types.Int64Value(1073741824),
		VolBlockSize: types.Int64Null(),
		Compression:  types.StringValue("lz4"),
		Sync:         types.StringValue("standard"),
		Dedup:        types.StringValue("off"),
		Sparse:       types.BoolNull(),
		Comments:     types.StringNull(),
		Pool:         types.StringNull(),
		Encrypted:    types.BoolNull(),
	}

	p := m.apiPayload()

	// Must always include type=VOLUME.
	if v, ok := p["type"]; !ok || v != "VOLUME" {
		t.Errorf("payload type = %v, want VOLUME", p["type"])
	}

	// Must include volsize.
	if v, ok := p["volsize"]; !ok || v != int64(1073741824) {
		t.Errorf("payload volsize = %v, want 1073741824", p["volsize"])
	}

	// Sparse must not appear when it is null.
	if _, ok := p["sparse"]; ok {
		t.Error("payload must not include sparse when it is null")
	}

	// volblocksize must not appear when null.
	if _, ok := p["volblocksize"]; ok {
		t.Error("payload must not include volblocksize when null")
	}

	// Compression must be uppercased in the payload.
	if v, ok := p["compression"]; !ok || v != "LZ4" {
		t.Errorf("payload compression = %v, want LZ4", p["compression"])
	}

	// Must use "deduplication" key (not "dedup").
	if _, ok := p["dedup"]; ok {
		t.Error("payload must use 'deduplication' key, not 'dedup'")
	}
	if v, ok := p["deduplication"]; !ok || v != "OFF" {
		t.Errorf("payload deduplication = %v, want OFF", p["deduplication"])
	}
}

// TestAccZvol_basic is an acceptance test; skipped unless TF_ACC=1 and TRUENAS_API_KEY is set.
func TestAccZvol_basic(t *testing.T) {
	name := "tank/tf-test-zvol"
	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []tfresource.TestStep{
			{
				Config: testAccZvolConfig(name, 1073741824, "lz4"),
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "name", name),
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "volsize", "1073741824"),
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "compression", "lz4"),
					tfresource.TestCheckResourceAttrSet("truenas_zvol.test", "pool"),
				),
			},
			// Update compression.
			{
				Config: testAccZvolConfig(name, 1073741824, "zstd"),
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("truenas_zvol.test", "compression", "zstd"),
				),
			},
			// Import.
			{
				ResourceName:            "truenas_zvol.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sparse"},
			},
		},
	})
}

func testAccZvolConfig(name string, volsize int64, compression string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name        = %q
  volsize     = %d
  compression = %q
}
`, name, volsize, compression)
}

func testAccCheckZvolDestroyed(name string) tfresource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		_, err := c.Call(context.Background(), "pool.dataset.get_instance", name)
		if err != nil {
			if client.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("error checking zvol %s: %v", name, err)
		}
		return fmt.Errorf("zvol %s still exists", name)
	}
}
