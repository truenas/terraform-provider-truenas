// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// This test drives the types through real Terraform with an in-memory
// "server" that rewrites values the way TrueNAS does. It needs the terraform
// binary, so it runs under TF_ACC like the acceptance tests (no TrueNAS box
// is used).

type thingModel struct {
	ID    types.String              `tfsdk:"id"`
	CIDR  normalize.CIDR            `tfsdk:"cidr"`
	Time  normalize.TimeOfDay       `tfsdk:"time"`
	Doc   normalize.JSONSubset      `tfsdk:"doc"`
	Users normalize.UnorderedList   `tfsdk:"users"`
	Nets  types.List                `tfsdk:"nets"`
	Pools []poolModel               `tfsdk:"pools"`
	Kind  normalize.CaseInsensitive `tfsdk:"kind"`
}

type poolModel struct {
	Base normalize.Network `tfsdk:"base"`
}

// fakeServer stores the normalized spelling, like TrueNAS.
type fakeServer struct {
	mu   sync.Mutex
	objs map[string]map[string]any
}

func explode(s string) string {
	p, err := normalize.ParsePrefix(s)
	if err != nil {
		return s
	}
	a := p.Addr()
	if a.Is4() {
		return p.String()
	}
	b := a.As16()
	parts := make([]string, 8)
	for i := range parts {
		parts[i] = fmt.Sprintf("%02x%02x", b[2*i], b[2*i+1])
	}
	return strings.Join(parts, ":") + fmt.Sprintf("/%d", p.Bits())
}

func (s *fakeServer) put(id string, m thingModel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var users []string
	_ = m.Users.ElementsAs(context.Background(), &users, false)
	sort.Strings(users)
	var nets []string
	for _, e := range m.Nets.Elements() {
		nets = append(nets, explode(e.(normalize.CIDR).ValueString()))
	}
	var pools []string
	for _, p := range m.Pools {
		pf, _ := normalize.ParsePrefix(p.Base.ValueString())
		pools = append(pools, pf.Masked().String())
	}
	t, _ := normalize.ParseTimeOfDay(m.Time.ValueString())
	var doc map[string]any
	_ = json.Unmarshal([]byte(m.Doc.ValueString()), &doc)
	doc["server_default"] = true
	if items, ok := doc["items"].([]any); ok {
		for _, it := range items {
			it.(map[string]any)["read_only"] = false
		}
	}
	docJSON, _ := json.Marshal(doc)
	s.objs[id] = map[string]any{
		"cidr":  explode(m.CIDR.ValueString()),
		"time":  fmt.Sprintf("%02d:%02d", t/3600, t/60%60),
		"doc":   string(docJSON),
		"users": users,
		"nets":  nets,
		"pools": pools,
		"kind":  strings.ToUpper(m.Kind.ValueString()),
	}
}

func (s *fakeServer) read(ctx context.Context, id string, m *thingModel) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objs[id]
	if !ok {
		return false
	}
	m.ID = types.StringValue(id)
	m.CIDR = normalize.NewCIDRValue(o["cidr"].(string))
	m.Time = normalize.NewTimeOfDayValue(o["time"].(string))
	m.Doc = normalize.NewJSONSubsetValue(o["doc"].(string))
	m.Users, _ = normalize.NewUnorderedListValueFrom(ctx, types.StringType, o["users"])
	var nets []attr.Value
	for _, n := range o["nets"].([]string) {
		nets = append(nets, normalize.NewCIDRValue(n))
	}
	m.Nets, _ = types.ListValue(normalize.CIDRType{}, nets)
	m.Pools = nil
	for _, p := range o["pools"].([]string) {
		m.Pools = append(m.Pools, poolModel{Base: normalize.NewNetworkValue(p)})
	}
	m.Kind = normalize.NewCaseInsensitiveValue(o["kind"].(string))
	return true
}

type thingResource struct{ srv *fakeServer }

func (r *thingResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "normtest_thing"
}

func (r *thingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"cidr": schema.StringAttribute{Required: true, CustomType: normalize.CIDRType{},
			PlanModifiers: []planmodifier.String{normalize.KeepStateIfEquivalent(normalize.CIDRType{})}},
		"time": schema.StringAttribute{Required: true, CustomType: normalize.TimeOfDayType{},
			PlanModifiers: []planmodifier.String{normalize.KeepStateIfEquivalent(normalize.TimeOfDayType{})}},
		"doc": schema.StringAttribute{Required: true, CustomType: normalize.JSONSubsetType{},
			PlanModifiers: []planmodifier.String{normalize.KeepStateIfEquivalent(normalize.JSONSubsetType{})}},
		"users": schema.ListAttribute{Required: true, ElementType: types.StringType, CustomType: normalize.UnorderedListOf(types.StringType),
			PlanModifiers: []planmodifier.List{normalize.KeepStateIfEquivalentList(normalize.UnorderedListOf(types.StringType))}},
		"nets": schema.ListAttribute{Required: true, ElementType: normalize.CIDRType{},
			PlanModifiers: []planmodifier.List{normalize.KeepStateIfEquivalentList(types.ListType{ElemType: normalize.CIDRType{}})}},
		"pools": schema.ListNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"base": schema.StringAttribute{Required: true, CustomType: normalize.NetworkType{},
					PlanModifiers: []planmodifier.String{normalize.KeepStateIfEquivalent(normalize.NetworkType{})}},
			},
		}},
		"kind": schema.StringAttribute{Required: true, CustomType: normalize.CaseInsensitiveType{},
			PlanModifiers: []planmodifier.String{
				normalize.KeepStateIfEquivalent(normalize.CaseInsensitiveType{}),
				stringplanmodifier.RequiresReplace(),
			}},
	}}
}

func (r *thingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m thingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.srv.put("one", m)
	r.srv.read(ctx, "one", &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *thingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m thingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.srv.read(ctx, m.ID.ValueString(), &m) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *thingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m thingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.srv.put(m.ID.ValueString(), m)
	r.srv.read(ctx, m.ID.ValueString(), &m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *thingResource) Delete(_ context.Context, req resource.DeleteRequest, _ *resource.DeleteResponse) {
	r.srv.mu.Lock()
	defer r.srv.mu.Unlock()
	delete(r.srv.objs, "one")
}

func (r *thingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

type testProvider struct{ srv *fakeServer }

func (p *testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "normtest"
}
func (p *testProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (p *testProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (p *testProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{func() resource.Resource { return &thingResource{srv: p.srv} }}
}
func (p *testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

const e2eConfig = `
resource "normtest_thing" "t" {
  cidr  = "%s"
  time  = "%s"
  doc   = <<-EOT
    {"name": "x",
     "items": [{"path": "/a"}]}
  EOT
  users = ["carol", "alice", "bob"]
  nets  = ["2001:db8::1", "10.0.0.1"]
  pools = [{ base = "172.17.0.0/12" }]
  kind  = "%s"
}
`

func TestAccNormalize_endToEnd(t *testing.T) {
	srv := &fakeServer{objs: map[string]map[string]any{}}
	factories := map[string]func() (tfprotov6.ProviderServer, error){
		"normtest": providerserver.NewProtocol6WithError(&testProvider{srv: srv}),
	}
	cfg := fmt.Sprintf(e2eConfig, "2001:db8::/64", "8:00", "filesystem")
	tfresource.Test(t, tfresource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []tfresource.TestStep{
			{
				// Create: every attribute is rewritten by the server; the
				// configured spellings stay in state and the re-plan is empty.
				Config: cfg,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("normtest_thing.t", "cidr", "2001:db8::/64"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "time", "8:00"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "users.0", "carol"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "nets.0", "2001:db8::1"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "pools.0.base", "172.17.0.0/12"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "kind", "filesystem"),
				),
			},
			{
				// A spelling-only config edit plans no change at all, not an
				// update and not a replacement.
				Config: fmt.Sprintf(e2eConfig, "2001:0DB8::/64", "08:00", "FileSystem"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				// A real change still plans: the RequiresReplace attribute
				// replaces.
				Config: fmt.Sprintf(e2eConfig, "2001:db8::/64", "8:00", "volume"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("normtest_thing.t", plancheck.ResourceActionReplace)},
				},
			},
			{
				// Forget the object (the fake server keeps it).
				Config: "removed {\n  from = normtest_thing.t\n  lifecycle {\n    destroy = false\n  }\n}\n",
			},
			{
				// Import stores the server spellings.
				ResourceName:       "normtest_thing.t",
				ImportState:        true,
				ImportStateId:      "one",
				ImportStatePersist: true,
				Config:             fmt.Sprintf(e2eConfig, "2001:db8::/64", "8:00", "volume"),
			},
			{
				// The same config after import: the string and list types
				// plan nothing, but JSONSubset is strict at plan time (so a
				// key removal is never hidden), so the server-added keys in
				// doc plan one in-place update. RequiresReplace on kind
				// does not fire.
				Config: fmt.Sprintf(e2eConfig, "2001:db8::/64", "8:00", "volume"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("normtest_thing.t", plancheck.ResourceActionUpdate)},
				},
			},
			{
				Config: fmt.Sprintf(e2eConfig, "2001:db8::/64", "8:00", "volume"),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				// State holds the server spellings from the import.
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("normtest_thing.t", "cidr", "2001:0db8:0000:0000:0000:0000:0000:0000/64"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "time", "08:00"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "users.0", "alice"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "pools.0.base", "172.16.0.0/12"),
					tfresource.TestCheckResourceAttr("normtest_thing.t", "kind", "VOLUME"),
				),
			},
		},
	})
}
