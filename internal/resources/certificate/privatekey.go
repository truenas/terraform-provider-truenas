// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TrueNAS re-encodes every imported private key before storing it: PKCS#1
// RSA ("RSA PRIVATE KEY", the tls_private_key default) and SEC1 EC ("EC
// PRIVATE KEY") keys come back as PKCS#8, passphrase-protected keys come back
// decrypted, and a missing trailing newline is added (probed on 26.0 and
// 28.0). Comparing the PEM text therefore reported a different key for the
// same key, failing every such import with "inconsistent result after apply"
// (B-013). The helpers below compare key material instead.

// parsePrivateKeyPEM decodes the first private-key block of a PEM string
// (skipping a leading "EC PARAMETERS" block, as `openssl ecparam -genkey`
// writes). encrypted reports a passphrase-protected key, which cannot be
// parsed without decrypting it.
func parsePrivateKeyPEM(s string) (key crypto.Signer, encrypted bool) {
	rest := []byte(s)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, false
		}
		if strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return nil, true
		}
		var k any
		var err error
		switch block.Type {
		case "ENCRYPTED PRIVATE KEY":
			return nil, true
		case "PRIVATE KEY":
			k, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		case "RSA PRIVATE KEY":
			k, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		case "EC PRIVATE KEY":
			k, err = x509.ParseECPrivateKey(block.Bytes)
		default:
			continue
		}
		if err != nil {
			return nil, false
		}
		signer, ok := k.(crypto.Signer)
		if !ok {
			return nil, false
		}
		return signer, false
	}
}

// boundPublicKey returns the public key of the first certificate or CSR in
// a PEM string, or nil.
func boundPublicKey(s string) crypto.PublicKey {
	rest := []byte(s)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		switch block.Type {
		case "CERTIFICATE":
			if c, err := x509.ParseCertificate(block.Bytes); err == nil {
				return c.PublicKey
			}
			return nil
		case "CERTIFICATE REQUEST", "NEW CERTIFICATE REQUEST":
			if c, err := x509.ParseCertificateRequest(block.Bytes); err == nil {
				return c.PublicKey
			}
			return nil
		}
	}
}

// publicKeysEqual compares two public keys by value (rsa, ecdsa and ed25519
// public keys all implement Equal).
func publicKeysEqual(a, b crypto.PublicKey) bool {
	ea, ok := a.(interface{ Equal(crypto.PublicKey) bool })
	return ok && b != nil && ea.Equal(b)
}

// samePrivateKey reports whether configured and server hold the same private
// key, whatever their PEM encodings. A passphrase-protected configured key
// cannot be parsed, so it counts as the same key when the server's key
// belongs to bound (the certificate or CSR the key was imported with): the
// server only accepts a key that matches it. Text equality short-circuits.
func samePrivateKey(configured, server, bound string) bool {
	if configured == server {
		return true
	}
	serverKey, _ := parsePrivateKeyPEM(server)
	if serverKey == nil {
		return false
	}
	cfgKey, encrypted := parsePrivateKeyPEM(configured)
	if cfgKey != nil {
		return publicKeysEqual(cfgKey.Public(), serverKey.Public())
	}
	if encrypted {
		return publicKeysEqual(serverKey.Public(), boundPublicKey(bound))
	}
	return false
}

// replaceIfPrivatekeyChanged requires replacement only when the planned
// privatekey is a different key from the one in state. A re-encoding of the
// same key (after import, state holds the server's PKCS#8 form while the
// configuration holds PKCS#1/SEC1 or an encrypted key) is an in-place update
// that only records the configured text; certificate.update never sends the
// key.
func replaceIfPrivatekeyChanged() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			if req.StateValue.IsNull() || req.PlanValue.IsUnknown() || req.PlanValue.IsNull() {
				resp.RequiresReplace = !req.StateValue.Equal(req.PlanValue)
				return
			}
			var cert, csr types.String
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("certificate"), &cert)...)
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("csr"), &csr)...)
			bound := cert.ValueString()
			if bound == "" {
				bound = csr.ValueString()
			}
			resp.RequiresReplace = !samePrivateKey(req.PlanValue.ValueString(), req.StateValue.ValueString(), bound)
		},
		"Replaced only when the private key itself changes; a different PEM encoding of the same key is recorded in place.",
		"Replaced only when the private key itself changes; a different PEM encoding of the same key is recorded in place.",
	)
}
