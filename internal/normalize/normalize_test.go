// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type pair struct {
	prior, server string
	want          bool
}

// checkPairs runs every pair through both the exported Equal function and the
// type's semantic equality (server value as receiver, as the framework does).
func checkPairs[K kind](t *testing.T, eq func(a, b string) bool, pairs []pair) {
	t.Helper()
	ctx := context.Background()
	for _, p := range pairs {
		if got := eq(p.prior, p.server); got != p.want {
			t.Errorf("Equal(%q, %q) = %v, want %v", p.prior, p.server, got, p.want)
		}
		got, diags := newValue[K](p.server).StringSemanticEquals(ctx, newValue[K](p.prior))
		if diags.HasError() {
			t.Fatalf("diags: %v", diags)
		}
		if got != p.want {
			t.Errorf("%T semantic equals(prior %q, server %q) = %v, want %v", newValue[K](""), p.prior, p.server, got, p.want)
		}
	}
}

func TestIPAddress(t *testing.T) {
	checkPairs[ipKind](t, EqualIP, []pair{
		{"fe80::e21f:2bff:fe46:7882", "fe80:0000:0000:0000:e21f:2bff:fe46:7882", true}, // B-076
		{"FE80::1", "fe80:0000:0000:0000:0000:0000:0000:0001", true},
		{"10.0.0.1", "10.0.0.1", true},
		{"10.0.0.1", "10.0.0.2", false},
		{"fe80::1", "fe80::2", false},
		{"not-an-ip", "NOT-AN-IP", false},
	})
}

func TestCIDR(t *testing.T) {
	checkPairs[cidrKind](t, EqualCIDR, []pair{
		{"2001:db8::/64", "2001:0db8:0000:0000:0000:0000:0000:0000/64", true}, // B-076
		{"198.51.100.7", "198.51.100.7/32", true},                             // B-076, B-075
		{"198.51.100.0/255.255.255.0", "198.51.100.0/24", true},               // B-076
		{"2001:db8::1", "2001:db8::1/128", true},                              // B-075 IPv6
		{"FDD0:0::/64", "fdd0::/64", true},                                    // B-079
		{"10.0.0.0/8", "10.0.0.0/255.0.0.0", true},
		{"0.0.0.0/0.0.0.0", "0.0.0.0/0", true},
		{"10.0.0.5/24", "10.0.0.0/24", false}, // host bits matter
		{"10.0.0.0/24", "10.0.0.0/25", false},
		{"10.0.0.1", "10.0.0.1/31", false},
		{"10.0.0.0/255.0.255.0", "10.0.0.0/255.0.255.0", true}, // identical text, even if invalid
		{"10.0.0.0/255.0.255.0", "10.0.0.0/8", false},
		{"bogus", "bogus/32", false},
	})
}

func TestNetwork(t *testing.T) {
	checkPairs[networkKind](t, EqualNetwork, []pair{
		{"172.17.0.0/12", "172.16.0.0/12", true},        // B-078
		{"192.168.100.10/24", "192.168.100.0/24", true}, // #13 NFS
		{"192.168.1.5", "192.168.1.5/32", true},         // B-075
		{"2001:db8::5/64", "2001:0db8:0000::/64", true}, // host bits + IPv6 spelling
		{"10.1.2.3/255.255.0.0", "10.1.0.0/16", true},   // netmask + host bits
		{"172.17.0.0/12", "172.16.0.0/16", false},
		{"172.17.0.0/12", "172.32.0.0/12", false},
	})
}

func TestParsePrefixErrors(t *testing.T) {
	for _, s := range []string{"", "10.0.0.0/33", "2001:db8::/129", "10.0.0.0/-1", "10.0.0.0/+8", "10.0.0.0/255.0.255.0", "2001:db8::/ffff::", "10.0.0.0/", "host.example.com"} {
		if _, err := ParsePrefix(s); err == nil {
			t.Errorf("ParsePrefix(%q) succeeded, want error", s)
		}
	}
}

func TestEmail(t *testing.T) {
	checkPairs[emailKind](t, EqualEmail, []pair{
		{"Admin@Example.COM", "Admin@example.com", true}, // B-077, B-127
		{"A@Example.COM", "A@example.com", true},
		{"admin@example.com", "Admin@example.com", false}, // local part kept verbatim
		{"admin", "ADMIN", false},
		{"a@b@Example.com", "a@b@example.com", true},
	})
}

func TestCaseInsensitive(t *testing.T) {
	checkPairs[caseInsensitiveKind](t, caseInsensitiveKind{}.equal, []pair{
		{"s-1-5-21-1-2-3-1001", "S-1-5-21-1-2-3-1001", true}, // B-074
		{"00:A0:98:12:34:56", "00:a0:98:12:34:56", true},     // B-082, B-083
		{"1m", "1M", true},       // B-126
		{"nfsv4", "NFSV4", true}, // B-002
		{"FILESYSTEM", "VOLUME", false},
	})
}

func TestTimeOfDay(t *testing.T) {
	checkPairs[timeOfDayKind](t, EqualTimeOfDay, []pair{
		{"8:00", "08:00", true}, // B-088
		{"8:5", "08:05", true},  // B-088 resilver
		{"08:00", "08:00:00", true},
		{"0:0", "00:00", true},
		{"8:00", "20:00", false},
		{"8:00", "08:01", false},
		{"24:00", "00:00", false},
	})
	for _, s := range []string{"8", "24:00", "12:60", "1:2:3:4", "aa:bb", "123:00", "-1:00", "12:00:60"} {
		if (timeOfDayKind{}).validate(s) == nil {
			t.Errorf("validate(%q) succeeded, want error", s)
		}
	}
	for _, s := range []string{"0:00", "23:59", "8:5", "12:30:15"} {
		if err := (timeOfDayKind{}).validate(s); err != nil {
			t.Errorf("validate(%q): %v", s, err)
		}
	}
}

func TestRFC3339(t *testing.T) {
	checkPairs[rfc3339Kind](t, EqualRFC3339, []pair{
		{"2035-01-01T02:00:00+02:00", "2035-01-01T00:00:00Z", true}, // B-059
		{"2035-01-01T00:00:00.123Z", "2035-01-01T00:00:00Z", true},  // B-059 fractional
		{"2034-12-31T19:00:00-05:00", "2035-01-01T00:00:00Z", true},
		{"2035-01-01T00:00:01Z", "2035-01-01T00:00:00Z", false},
		{"2035-01-01", "2035-01-01T00:00:00Z", false},
	})
	if (rfc3339Kind{}).validate("2035-01-01") == nil {
		t.Error("validate accepted a date without a time")
	}
}

func TestURL(t *testing.T) {
	checkPairs[urlKind](t, EqualURL, []pair{
		{"https://mirror.gcr.io", "https://mirror.gcr.io/", true},                     // B-080
		{"https://example.com", "https://example.com/", true},                         // B-085 HTTP/WEBDAV
		{"https://Example.COM:443/Files", "https://example.com/Files", true},          // B-085
		{"http://h:80/x", "http://h/x", true},                                         // B-085
		{"https://s3.example.com:443", "https://s3.example.com/", true},               // B-085 S3
		{"https://s3.example.com:443/", "https://s3.example.com/", true},              // B-085 S3
		{"https://S3.EXAMPLE.com/", "https://s3.example.com/", true},                  // B-085
		{"https://Gateway.storjshare.io:443", "https://gateway.storjshare.io/", true}, // B-085 STORJ
		{"https://ghcr.io", "https://ghcr.io/", true},                                 // B-122
		{"HTTPS://ghcr.io/v2/", "https://ghcr.io/v2", true},
		{"http://[FE80::1]:80/", "http://[fe80::1]/", true},
		{"https://h:8443/", "https://h:8443", true},
		{"https://h:8443/", "https://h/", false},
		{"http://h:443/", "http://h/", false}, // 443 is not http's default
		{"https://h/Files", "https://h/files", false},
		{"https://h/?a=1", "https://h/?a=2", false},
		{"https://h/", "http://h/", false},
		{"docker.io", "docker.io/", true}, // not absolute: trailing slash only
		{"docker.io", "Docker.io", false},
		{"https://h/", "h/", false},
	})
}

func TestTrimmed(t *testing.T) {
	checkPairs[trimmedKind](t, EqualTrimmed, []pair{
		{"ssh-ed25519 AAAA user@host\n", "ssh-ed25519 AAAA user@host", true},                           // B-053
		{"-----BEGIN X-----\nabc\n-----END X-----", "-----BEGIN X-----\nabc\n-----END X-----\n", true}, // B-054
		{"a\r\nb\r\n", "a\nb", true},
		{"a b", "a  b", false},
	})
}

func pemOf(t *testing.T, typ string, der []byte) string {
	t.Helper()
	return string(pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}))
}

func TestPrivateKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8 := func(k any) string {
		der, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			t.Fatal(err)
		}
		return pemOf(t, "PRIVATE KEY", der)
	}
	sec1DER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}
	ecParams := pemOf(t, "EC PARAMETERS", []byte{0x06, 0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07})
	pkcs1 := pemOf(t, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey))
	server := pkcs8(rsaKey)
	checkPairs[privateKeyKind](t, EqualPrivateKey, []pair{
		{pkcs1, server, true}, // B-013 PKCS#1 (tls_private_key RSA)
		{pemOf(t, "EC PRIVATE KEY", sec1DER), pkcs8(ecKey), true}, // B-013 SEC1 (tls_private_key ECDSA)
		{ecParams + pemOf(t, "EC PRIVATE KEY", sec1DER), pkcs8(ecKey), true},
		{server[:len(server)-1], server, true}, // B-013 no trailing newline
		{pkcs8(edKey), pkcs8(edKey), true},
		{pkcs1, pkcs8(otherRSA), false},
		{pkcs8(ecKey), server, false},
		{"garbage", "garbage\n", true},
		{"garbage", "other", false},
	})
	encrypted := "-----BEGIN RSA PRIVATE KEY-----\nProc-Type: 4,ENCRYPTED\nDEK-Info: AES-128-CBC,00000000000000000000000000000000\n\nAAAA\n-----END RSA PRIVATE KEY-----\n"
	if EqualPrivateKey(encrypted, server) {
		t.Error("an encrypted key compared equal to a different key")
	}
}

func TestSAN(t *testing.T) {
	checkPairs[sanKind](t, EqualSAN, []pair{
		{"10.0.0.1", "IP Address:10.0.0.1", true}, // B-060
		{"IP:10.0.0.2", "IP Address:10.0.0.2", true},
		{"DNS:c.example.com", "DNS:c.example.com", true},
		{"c.example.com", "DNS:c.example.com", true},
		{"DNS:c.example.com", "c.example.com", true},
		{"2001:db8::1", "IP Address:2001:DB8:0:0:0:0:0:1", true},
		{"email:A@Example.com", "email:A@example.com", true},
		{"a.example.com", "DNS:b.example.com", false},
		{"10.0.0.1", "DNS:10.0.0.1", false},
		{"10.0.0.1", "IP Address:10.0.0.2", false},
	})
}

func TestOctalMode(t *testing.T) {
	checkPairs[octalModeKind](t, EqualOctalMode, []pair{
		{"755", "0755", true}, // B-091
		{"0o755", "0755", true},
		{"1777", "01777", true},
		{"0", "000", true},
		{"755", "0775", false},
		{"789", "0789", false},
	})
	for _, s := range []string{"789", "17777", "rwxr-xr-x", "0x1ed", "-755"} {
		if (octalModeKind{}).validate(s) == nil {
			t.Errorf("validate(%q) succeeded, want error", s)
		}
	}
}

func TestJSON(t *testing.T) {
	checkPairs[jsonKind](t, EqualJSON, []pair{
		{`{"a": 1, "b": [true, null]}`, `{"b":[true,null],"a":1}`, true}, // B-061, B-095 heredoc
		{"{\n  \"type\": \"DEFAULT\"\n}\n", `{"type":"DEFAULT"}`, true},  // B-005 idmap spelling
		{`{"n": 1.0}`, `{"n": 1}`, true},
		{`{"n": 12345678901234567890}`, `{"n": 12345678901234567891}`, false},
		{`{"a": 1}`, `{"a": 1, "b": null}`, false}, // strict: added keys matter
		{`[1,2]`, `[2,1]`, false},
		{`{"a": "1"}`, `{"a": 1}`, false},
		{`{bad`, `{bad`, true},
		{`{bad`, `{"bad": 1}`, false},
	})
	if (jsonKind{}).validate(`{"a":1} {}`) == nil {
		t.Error("validate accepted trailing data")
	}
}

func TestJSONSubset(t *testing.T) {
	checkPairs[jsonSubsetKind](t, JSONSubsetOf, []pair{
		// B-094: defaults filled into list items.
		{
			`{"storage":{"additional_storage":[{"type":"host_path","mount_path":"/tfh","host_path_config":{"path":"/mnt/fast/x"}}]}}`,
			`{"storage":{"additional_storage":[{"type":"host_path","mount_path":"/tfh","read_only":false,"host_path_config":{"path":"/mnt/fast/x","acl_enable":false}}]},"run_as":{"user":568}}`,
			true,
		},
		// B-092: partial permission bag.
		{`[{"tag":"owner@","perms":{"BASIC":"FULL_CONTROL"}}]`, `[{"tag":"owner@","id":null,"perms":{"BASIC":"FULL_CONTROL"},"flags":{"BASIC":"INHERIT"}}]`, true},
		// B-120: server adds null keys.
		{`{"api_token":"x"}`, `{"api_token":"x","api_key":null,"email":null}`, true},
		{`{"a":null}`, `{}`, true},
		{`{"a":null}`, `{"a":1}`, false},
		{`{"a":1}`, `{"a":2,"b":3}`, false},
		{`{"a":1,"b":3}`, `{"a":1}`, false}, // configured key missing on the server
		{`{"l":[1,2]}`, `{"l":[1,2,3]}`, false},
		{`{"l":[{"x":1}]}`, `{"l":[{"x":1},{"x":2}]}`, false},
	})
}

func TestYAML11(t *testing.T) {
	checkPairs[yamlKind](t, EqualYAML11, []pair{
		// B-097: what app.config returns for "FOO: yes / BAR: off / BAZ: 0755 / QUX: 1:30".
		{"FOO: yes\nBAR: off\nBAZ: 0755\nQUX: 1:30\n", `{"FOO": true, "BAR": false, "BAZ": 493, "QUX": 90}`, true},
		{"FOO: yes\n", `{"FOO": "yes"}`, false},
		{"FOO: 'yes'\n", `{"FOO": "yes"}`, true},
		{"a: 0x1F\nb: 0b101\nc: 1_000\nd: -0o7\ne: 08\nf: 1.5\ng: 1e5\nh: 1.0e+5\ni: ~\nj: \nk: On\nl: y\n",
			`{"a":31,"b":5,"c":1000,"d":"-0o7","e":"08","f":1.5,"g":"1e5","h":100000,"i":null,"j":null,"k":true,"l":"y"}`, true},
		{"t: !!str yes\nn: !!int \"12\"\nf: !!float 1\n", `{"t":"yes","n":12,"f":1.0}`, true},
		{"1: a\nyes: b\n~: c\n", `{"1":"a","true":"b","null":"c"}`, true},
		{"base: &b {x: 1, y: 2}\nsvc:\n  <<: *b\n  y: 3\n", `{"base":{"x":1,"y":2},"svc":{"x":1,"y":3}}`, true},
		{"a: &a {x: 1}\nb: &b {x: 2, z: 2}\nc:\n  <<: [*a, *b]\n", `{"a":{"x":1},"b":{"x":2,"z":2},"c":{"x":1,"z":2}}`, true},
		{"services:\n  web:\n    image: nginx\n", "services: {web: {image: nginx}}", true},
		{"services:\n  web:\n    image: nginx\n", "services: {web: {image: httpd}}", false},
		{"t: 1:30.5\nneg: -1:30\n", `{"t":90.5,"neg":-90}`, true},
		{"x: .inf\n", "x: .Inf\n", true},
	})
	if _, err := DecodeYAML11("a: 1\n---\nb: 2\n"); err == nil {
		t.Error("multi-document YAML accepted")
	}
	if _, err := DecodeYAML11("a: [1\n"); err == nil {
		t.Error("broken YAML accepted")
	}
	if (yamlKind{}).validate("a: !!int abc\n") == nil {
		t.Error("!!int abc accepted")
	}
	bomb := "a: &a [x,x,x,x,x,x,x,x,x,x]\nb: &b [*a,*a,*a,*a,*a,*a,*a,*a,*a,*a]\nc: &c [*b,*b,*b,*b,*b,*b,*b,*b,*b,*b]\nd: &d [*c,*c,*c,*c,*c,*c,*c,*c,*c,*c]\ne: &e [*d,*d,*d,*d,*d,*d,*d,*d,*d,*d]\nf: &f [*e,*e,*e,*e,*e,*e,*e,*e,*e,*e]\ng: [*f,*f,*f,*f,*f,*f,*f,*f,*f,*f]\n"
	if _, err := DecodeYAML11(bomb); err == nil {
		t.Error("alias bomb was expanded")
	}
	if v, err := DecodeYAML11(""); err != nil || v != nil {
		t.Errorf("empty document = %v, %v", v, err)
	}
}

func TestNullUnknownNeverSemanticallyEqual(t *testing.T) {
	ctx := context.Background()
	known := NewCIDRValue("10.0.0.1")
	for _, other := range []CIDR{NewCIDRNull(), NewCIDRUnknown()} {
		if got, _ := known.StringSemanticEquals(ctx, other); got {
			t.Errorf("known vs %v compared equal", other)
		}
		if got, _ := other.StringSemanticEquals(ctx, known); got {
			t.Errorf("%v vs known compared equal", other)
		}
		if got, _ := other.StringSemanticEquals(ctx, other); got {
			t.Errorf("%v vs itself compared equal", other)
		}
	}
	// A different normalize type never matches.
	if got, _ := NewCIDRValue("10.0.0.1").StringSemanticEquals(ctx, NewIPAddressValue("10.0.0.1")); got {
		t.Error("CIDR compared equal to an IPAddress value")
	}
	if got, _ := NewCIDRValue("10.0.0.1").StringSemanticEquals(ctx, basetypes.NewStringValue("10.0.0.1")); got {
		t.Error("CIDR compared equal to a plain string value")
	}
	if !NewCIDRPointerValue(nil).IsNull() {
		t.Error("NewCIDRPointerValue(nil) is not null")
	}
	s := "x"
	if NewCIDRPointerValue(&s).ValueString() != "x" {
		t.Error("NewCIDRPointerValue lost the value")
	}
}

func TestValidateAttribute(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		v    xattr.ValidateableAttribute
		fail bool
	}{
		{NewCIDRValue("10.0.0.0/24"), false},
		{NewCIDRValue("10.0.0.0/33"), true},
		{NewCIDRValue(""), false}, // "" is the clear sentinel
		{NewCIDRNull(), false},
		{NewCIDRUnknown(), false},
		{NewNetworkValue("example.com"), true},
		{NewIPAddressValue("10.0.0.0/24"), true},
		{NewIPAddressValue("fe80::1%eth0"), false},
		{NewTimeOfDayValue("25:00"), true},
		{NewRFC3339Value("tomorrow"), true},
		{NewOctalModeValue("0755"), false},
		{NewJSONValue("{"), true},
		{NewJSONSubsetValue("{}"), false},
		{NewYAMLValue("a: [1"), true},
		{NewURLValue("http://[::1"), true},
		{NewEmailValue("not an email"), false},
	}
	for _, c := range cases {
		resp := &xattr.ValidateAttributeResponse{}
		c.v.ValidateAttribute(ctx, xattr.ValidateAttributeRequest{Path: path.Root("x")}, resp)
		if resp.Diagnostics.HasError() != c.fail {
			t.Errorf("validate %v: errors=%v, want fail=%v", c.v, resp.Diagnostics, c.fail)
		}
	}
}

func TestTypeRoundTrip(t *testing.T) {
	ctx := context.Background()
	typ := CIDRType{}
	v, err := typ.ValueFromTerraform(ctx, tftypes.NewValue(tftypes.String, "10.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	c, ok := v.(CIDR)
	if !ok || c.ValueString() != "10.0.0.1" {
		t.Fatalf("ValueFromTerraform = %#v", v)
	}
	if !c.Type(ctx).Equal(typ) || typ.Equal(URLType{}) || typ.Equal(basetypes.StringType{}) {
		t.Error("type equality is wrong")
	}
	if typ.String() != "normalize.CIDRType" {
		t.Errorf("String() = %q", typ.String())
	}
	if !c.Equal(NewCIDRValue("10.0.0.1")) || c.Equal(NewCIDRValue("10.0.0.1/32")) || c.Equal(basetypes.NewStringValue("10.0.0.1")) {
		t.Error("value Equal must stay exact")
	}
	sv, diags := typ.ValueFromString(ctx, basetypes.NewStringValue("x"))
	if diags.HasError() || sv.(CIDR).ValueString() != "x" {
		t.Error("ValueFromString")
	}
	if _, ok := typ.ValueType(ctx).(CIDR); !ok {
		t.Error("ValueType")
	}
}

func strList(t *testing.T, elem attr.Type, vals ...string) UnorderedList {
	t.Helper()
	elems := make([]attr.Value, len(vals))
	for i, s := range vals {
		v, d := elem.(basetypes.StringTypable).ValueFromString(context.Background(), basetypes.NewStringValue(s))
		if d.HasError() {
			t.Fatal(d)
		}
		elems[i] = v
	}
	l, d := NewUnorderedListValue(elem, elems)
	if d.HasError() {
		t.Fatal(d)
	}
	return l
}

func TestUnorderedList(t *testing.T) {
	ctx := context.Background()
	str := basetypes.StringType{}
	cases := []struct {
		prior, server UnorderedList
		want          bool
	}{
		{strList(t, str, "a", "b", "c"), strList(t, str, "c", "a", "b"), true},
		{strList(t, str, "a", "a", "b"), strList(t, str, "a", "b", "b"), false},
		{strList(t, str, "a", "b"), strList(t, str, "a", "b", "b"), false},
		{strList(t, str), strList(t, str), true},
		{strList(t, CIDRType{}, "10.0.0.1", "2001:db8::1"), strList(t, CIDRType{}, "2001:0db8::1/128", "10.0.0.1/32"), true},
		{strList(t, CIDRType{}, "10.0.0.1"), strList(t, CIDRType{}, "10.0.0.2/32"), false},
		{NewUnorderedListNull(str), strList(t, str), false},
		{NewUnorderedListUnknown(str), strList(t, str), false},
	}
	for i, c := range cases {
		got, diags := c.server.ListSemanticEquals(ctx, c.prior)
		if diags.HasError() {
			t.Fatal(diags)
		}
		if got != c.want {
			t.Errorf("case %d: got %v, want %v", i, got, c.want)
		}
	}
	ints, d := NewUnorderedListValueFrom(ctx, basetypes.Int64Type{}, []int64{3, 1, 2})
	if d.HasError() {
		t.Fatal(d)
	}
	ints2, _ := NewUnorderedListValueFrom(ctx, basetypes.Int64Type{}, []int64{1, 2, 3})
	ints3, _ := NewUnorderedListValueFrom(ctx, basetypes.Int64Type{}, []int64{1, 2, 4})
	if got, _ := ints2.ListSemanticEquals(ctx, ints); !got {
		t.Error("[3 1 2] != [1 2 3]")
	}
	if got, _ := ints3.ListSemanticEquals(ctx, ints); got {
		t.Error("[3 1 2] == [1 2 4]")
	}

	typ := UnorderedListOf(CIDRType{})
	if !typ.Equal(UnorderedListOf(CIDRType{})) || typ.Equal(UnorderedListOf(str)) || typ.Equal(basetypes.ListType{ElemType: CIDRType{}}) {
		t.Error("UnorderedListType equality is wrong")
	}
	tv := tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{tftypes.NewValue(tftypes.String, "10.0.0.1")})
	v, err := typ.ValueFromTerraform(ctx, tv)
	if err != nil {
		t.Fatal(err)
	}
	ul, ok := v.(UnorderedList)
	if !ok || !ul.Type(ctx).Equal(typ) {
		t.Fatalf("ValueFromTerraform = %#v", v)
	}
	if _, ok := ul.Elements()[0].(CIDR); !ok {
		t.Errorf("element type = %T, want CIDR", ul.Elements()[0])
	}
}

func TestEqualMAC(t *testing.T) {
	for _, p := range []pair{
		{"00:A0:98:12:34:56", "00:a0:98:12:34:56", true}, // B-082, B-083
		{"00-A0-98-12-34-56", "00:a0:98:12:34:56", true},
		{"00:a0:98:12:34:56", "00:a0:98:12:34:57", false},
		{"not-a-mac", "NOT-A-MAC", false},
		{"not-a-mac", "not-a-mac", true},
	} {
		if got := EqualMAC(p.prior, p.server); got != p.want {
			t.Errorf("EqualMAC(%q, %q) = %v", p.prior, p.server, got)
		}
	}
}

func TestEqualTreeFunc(t *testing.T) {
	leaf := KeyEqual(map[string]func(a, b string) bool{"mac": EqualMAC, "ips": EqualIP})
	cfg, _ := DecodeJSON(`{"dtype":"NIC","mac":"00:A0:98:12:34:56","ips":["2001:db8::1"]}`)
	srv, _ := DecodeJSON(`{"dtype":"NIC","mac":"00:a0:98:12:34:56","ips":["2001:0db8::1"],"nic_attach":null,"trust_guest_rx_filters":false}`)
	if !EqualTreeFunc(cfg, srv, true, leaf) {
		t.Error("subset with mac/ip hook should match")
	}
	if EqualTreeFunc(cfg, srv, false, leaf) {
		t.Error("strict compare should see the added keys")
	}
	if EqualTree(cfg, srv, true) {
		t.Error("without the hook the mac case differs")
	}
	other, _ := DecodeJSON(`{"dtype":"NIC","mac":"00:a0:98:12:34:57","ips":["2001:db8::1"]}`)
	if EqualTreeFunc(cfg, other, true, leaf) {
		t.Error("different mac matched")
	}
	num, _ := DecodeJSON(`{"mac": 5}`)
	if !EqualTreeFunc(num, num, true, leaf) {
		t.Error("non-string leaf under a hooked key must fall back to the default comparison")
	}
}
