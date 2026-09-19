// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// SCRAM-SHA-512 authentication for TrueNAS API keys (TrueNAS 26.0+).
//
// The exchange runs over auth.login_ex per RFC 5802, with TrueNAS
// specifics (docs/source/accounts/scram_authentication.rst in the
// middleware tree):
//
//   - hash is SHA-512; SaltedPassword = PBKDF2-HMAC-SHA512(secret, salt, i)
//     where secret is the API key's random part (after the "<id>-" prefix)
//   - the SCRAM username is "<username>:<api_key_id>"
//   - the exchange is unbound (GS2 "n,,"): TrueNAS negotiates channel
//     binding per exchange and accepts unbound clients
//   - the server's final message carries v=<ServerSignature>, verified here
//     so a server that doesn't know the key is rejected (mutual auth)
//
// The raw key never crosses the wire — only proof of possession does.

// scramLoginResponse is the auth.login_ex result shape for SCRAM steps.
type scramLoginResponse struct {
	ResponseType string `json:"response_type"`
	ScramType    string `json:"scram_type"`
	RFCStr       string `json:"rfc_str"`
}

// scramConversation holds the state of one SCRAM exchange.
type scramConversation struct {
	clientFirstBare string
	clientNonceRaw  []byte // decoded form; the wire carries base64
	serverFirst     string
	serverNonce     string // combined nonce, base64 as sent by the server
	saltedPassword  []byte
}

// newScramNonce returns a 32-byte random nonce (the size the TrueNAS
// server requires after base64-decoding the r= attribute).
func newScramNonce() ([]byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("scram nonce: %w", err)
	}
	return buf, nil
}

// clientFirstMessage builds the full client-first message ("n,,n=...,r=...")
// and records the bare form for the later AuthMessage. nonceRaw must be 32
// bytes: the server base64-decodes the r= value and rejects any other size
// (truenas_scram scram_parse_nonce).
func (sc *scramConversation) clientFirstMessage(username string, apiKeyID int, nonceRaw []byte) string {
	sc.clientNonceRaw = nonceRaw
	sc.clientFirstBare = fmt.Sprintf("n=%s:%d,r=%s", username, apiKeyID, base64.StdEncoding.EncodeToString(nonceRaw))
	return "n,," + sc.clientFirstBare
}

// processServerFirst parses "r=...,s=...,i=..." and derives SaltedPassword
// from the API key secret.
func (sc *scramConversation) processServerFirst(serverFirst, secret string) error {
	sc.serverFirst = serverFirst

	var salt []byte
	var iterations int
	for _, part := range strings.Split(serverFirst, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "r":
			sc.serverNonce = kv[1]
		case "s":
			var err error
			salt, err = base64.StdEncoding.DecodeString(kv[1])
			if err != nil {
				return fmt.Errorf("scram: invalid salt encoding: %w", err)
			}
		case "i":
			var err error
			iterations, err = strconv.Atoi(kv[1])
			if err != nil {
				return fmt.Errorf("scram: invalid iteration count %q", kv[1])
			}
		}
	}

	// The combined nonce is binary: the server decodes the client's
	// base64 nonce, appends its own 32 bytes, and re-encodes all 64. The
	// base64 strings therefore diverge at the last encoding group — the
	// prefix check must happen on the decoded bytes.
	combined, err := base64.StdEncoding.DecodeString(sc.serverNonce)
	if err != nil {
		return fmt.Errorf("scram: invalid server nonce encoding: %w", err)
	}
	if len(combined) <= len(sc.clientNonceRaw) || !bytes.HasPrefix(combined, sc.clientNonceRaw) {
		return fmt.Errorf("scram: server nonce does not extend client nonce")
	}
	// Mirror the truenas_scram server-side limits (SCRAM_DEFAULT_SALT_SZ,
	// SCRAM_MIN_ITERS/SCRAM_MAX_ITERS): a 16-byte salt and a sane
	// iteration range. The floor also stops a tampering middlebox from
	// handing the client a trivially cheap PBKDF2 target.
	if len(salt) != 16 {
		return fmt.Errorf("scram: salt must be 16 bytes, got %d", len(salt))
	}
	if iterations < 50000 || iterations > 5000000 {
		return fmt.Errorf("scram: iteration count %d outside 50000..5000000", iterations)
	}

	salted, err := pbkdf2.Key(sha512.New, secret, salt, iterations, sha512.Size)
	if err != nil {
		return fmt.Errorf("scram: pbkdf2: %w", err)
	}
	sc.saltedPassword = salted
	return nil
}

// authMessage is the RFC 5802 AuthMessage both proofs are computed over.
func (sc *scramConversation) authMessage() string {
	return sc.clientFirstBare + "," + sc.serverFirst + "," + sc.clientFinalWithoutProof()
}

func (sc *scramConversation) clientFinalWithoutProof() string {
	gs2 := base64.StdEncoding.EncodeToString([]byte("n,,")) // "biws"
	return fmt.Sprintf("c=%s,r=%s", gs2, sc.serverNonce)
}

// clientFinalMessage computes the client proof and returns the final
// message ("c=...,r=...,p=...").
func (sc *scramConversation) clientFinalMessage() string {
	clientKey := hmacSHA512(sc.saltedPassword, []byte("Client Key"))
	storedKey := sha512.Sum512(clientKey)
	clientSignature := hmacSHA512(storedKey[:], []byte(sc.authMessage()))

	proof := make([]byte, len(clientKey))
	for i := range clientKey {
		proof[i] = clientKey[i] ^ clientSignature[i]
	}
	return sc.clientFinalWithoutProof() + ",p=" + base64.StdEncoding.EncodeToString(proof)
}

// verifyServerFinal checks the server's v=<signature> against the locally
// computed ServerSignature (mutual authentication).
func (sc *scramConversation) verifyServerFinal(serverFinal string) error {
	if !strings.HasPrefix(serverFinal, "v=") {
		return fmt.Errorf("scram: malformed server-final message")
	}
	got, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(serverFinal, "v="))
	if err != nil {
		return fmt.Errorf("scram: invalid server signature encoding: %w", err)
	}
	serverKey := hmacSHA512(sc.saltedPassword, []byte("Server Key"))
	want := hmacSHA512(serverKey, []byte(sc.authMessage()))
	if !hmac.Equal(got, want) {
		return fmt.Errorf("scram: server signature verification failed")
	}
	return nil
}

func hmacSHA512(key, msg []byte) []byte {
	h := hmac.New(sha512.New, key)
	h.Write(msg)
	return h.Sum(nil)
}

// splitAPIKey parses the "<id>-<secret>" TrueNAS API key format.
func splitAPIKey(apiKey string) (int, string, error) {
	idStr, secret, ok := strings.Cut(apiKey, "-")
	if !ok {
		return 0, "", fmt.Errorf("invalid API key format (want <id>-<secret>)")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, "", fmt.Errorf("invalid API key ID %q", idStr)
	}
	return id, secret, nil
}

// scramStep sends one auth.login_ex SCRAM message and decodes the response.
func scramStep(ctx context.Context, c *Client, scramType, rfcStr string) (*scramLoginResponse, error) {
	raw, err := c.Call(ctx, "auth.login_ex", map[string]any{
		"mechanism":  "SCRAM",
		"scram_type": scramType,
		"rfc_str":    rfcStr,
	})
	if err != nil {
		return nil, fmt.Errorf("auth.login_ex %s: %w", scramType, err)
	}
	var resp scramLoginResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse auth.login_ex response: %w", err)
	}
	if resp.ResponseType != "SCRAM_RESPONSE" {
		return nil, fmt.Errorf("scram %s rejected: response_type %s", scramType, resp.ResponseType)
	}
	return &resp, nil
}

// AuthAPIKeySCRAM authenticates an API key via SCRAM-SHA-512 (TrueNAS
// 26.0+). username must be the key owner's account; the raw key secret
// never leaves the client.
func AuthAPIKeySCRAM(ctx context.Context, c *Client, username, apiKey string) error {
	keyID, secret, err := splitAPIKey(apiKey)
	if err != nil {
		return err
	}
	nonce, err := newScramNonce()
	if err != nil {
		return err
	}

	sc := &scramConversation{}
	first, err := scramStep(ctx, c, "CLIENT_FIRST_MESSAGE", sc.clientFirstMessage(username, keyID, nonce))
	if err != nil {
		return err
	}
	if err := sc.processServerFirst(first.RFCStr, secret); err != nil {
		return err
	}
	final, err := scramStep(ctx, c, "CLIENT_FINAL_MESSAGE", sc.clientFinalMessage())
	if err != nil {
		return err
	}
	return sc.verifyServerFinal(final.RFCStr)
}

// MechanismChoices returns the login mechanisms the server offers
// (auth.mechanism_choices, callable before authentication). Servers older
// than 26.0 either omit "SCRAM" or fail the call entirely; callers treat
// any error as "legacy mechanisms only".
func MechanismChoices(ctx context.Context, c *Client) ([]string, error) {
	raw, err := c.Call(ctx, "auth.mechanism_choices")
	if err != nil {
		return nil, err
	}
	var choices []string
	if err := json.Unmarshal(raw, &choices); err != nil {
		return nil, err
	}
	return choices, nil
}

// AuthAPIKeyAuto picks the strongest supported API-key mechanism:
// SCRAM-SHA-512 when the server offers it (TrueNAS 26.0+) and the key
// owner's username is known, otherwise the legacy plain login. The
// mechanism probe failing (as it does on 25.10) selects plain.
func AuthAPIKeyAuto(ctx context.Context, c *Client, username, apiKey string) error {
	if username != "" {
		choices, err := MechanismChoices(ctx, c)
		if err == nil && slices.Contains(choices, "SCRAM") {
			return AuthAPIKeySCRAM(ctx, c, username, apiKey)
		}
	}
	return AuthAPIKey(ctx, c, apiKey)
}
