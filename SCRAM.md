# SCRAM-SHA-512 Client Implementation

This document describes how the Terraform provider authenticates TrueNAS API
keys with SCRAM-SHA-512, and how that implementation is tested. It is written
for review by the TrueNAS middleware team: every protocol decision below is
traced to the middleware's published behavior — `auth.login_ex` in the
versioned API, `docs/source/accounts/scram_authentication.rst` in the
middleware tree, and the `truenas/truenas_scram` C library that backs
server-side parsing.

Implementation: `internal/client/scram.go`
Tests: `internal/client/scram_internal_test.go`, `internal/client/scram_live_test.go`

## Scope

- SCRAM-SHA-512 for **API keys** over `auth.login_ex` on the versioned
  JSON-RPC 2.0 endpoint (`/api/current`).
- Unbound exchanges only (GS2 header `n,,`). Channel binding
  (`tls-server-end-point`, SCRAM-PLUS) is not yet implemented; see
  *Known limitations*.
- Password logins continue to use `auth.login`; API-key logins on servers
  without SCRAM use `auth.login_with_api_key`.

## Mechanism selection

The provider selects the mechanism by capability, not by version string:

1. If the operator supplied the key owner's `username`, the client calls
   `auth.mechanism_choices` before authenticating (the method is callable
   pre-auth).
2. If the response lists `"SCRAM"`, the client runs the SCRAM exchange.
3. Otherwise — including when `auth.mechanism_choices` itself fails, which
   is the observed behavior on 25.10
   (`'NoneType' object has no attribute 'may_create_auth_token'`) — the
   client falls back to `auth.login_with_api_key`.

Two properties worth noting:

- **No downgrade after selection.** Once SCRAM is selected, a failed
  exchange is a failed login. The client never retries the same
  credentials over the PLAIN mechanism after a SCRAM failure, so an active
  attacker cannot force a downgrade by breaking the SCRAM exchange.
- **PLAIN is never reached implicitly on a SCRAM-capable server** when a
  username is configured. Without a username the client cannot build the
  SCRAM identity (see below) and uses the legacy login; this is an explicit
  operator choice, and the raw key is then protected by TLS only.

## The exchange

All messages travel as `auth.login_ex` calls:

```json
{"mechanism": "SCRAM", "scram_type": "<step>", "rfc_str": "<RFC 5802 message>"}
```

A response whose `response_type` is not `SCRAM_RESPONSE` (e.g. `AUTH_ERR`)
aborts the exchange immediately.

### Client-first message

```
n,,n=<username>:<api_key_id>,r=<base64(32 random bytes)>
```

- **GS2 header** `n,,`: no channel binding requested. The middleware's
  API-key PAM stack runs `channel_binding=negotiate`, which accepts unbound
  clients (verified against `test__scram_unbound_login_allowed_with_server_binding`
  in the middleware test suite).
- **Identity**: `<username>:<api_key_id>`, the concatenation documented in
  `scram_authentication.rst` ("the username field must be formatted as
  `{username}:{api_key_id}`"). The key ID is parsed from the raw key's
  `<id>-<secret>` form.
- **Nonce**: 32 bytes from `crypto/rand`, standard base64. The size is not
  arbitrary: `truenas_scram`'s `scram_parse_nonce()` base64-decodes the
  `r=` value and rejects any decoded size other than `SCRAM_NONCE_SIZE`
  (32) for a client nonce. This was confirmed empirically — a
  non-32-byte nonce is answered with `AUTH_ERR` at the first message.

### Server-first message

```
r=<base64(client 32 bytes ++ server 32 bytes)>,s=<base64 salt>,i=<iterations>
```

Client-side validation before any key derivation:

- **Nonce continuity is checked on decoded bytes, not strings.** The
  server decodes the client nonce, appends its own 32 bytes, and re-encodes
  all 64 (`scram_parse_nonce` accepts `SCRAM_NONCE_SIZE * 2` for the
  combined form). Because 32 is not a multiple of the base64 3-byte group,
  the combined value's base64 form is *not* a string extension of the
  client's — a naive `strings.HasPrefix` check on the base64 forms fails
  against a genuine server. The client decodes the combined nonce and
  requires the decoded value to be a strict byte-prefix extension of its
  own 32 bytes.
- **Salt** must decode to exactly 16 bytes (`SCRAM_DEFAULT_SALT_SZ`).
- **Iterations** must lie in 50,000..5,000,000 — the same
  `SCRAM_MIN_ITERS`/`SCRAM_MAX_ITERS` bounds the server enforces. The
  client-side floor also means a tampering middlebox cannot hand the client
  a trivially cheap PBKDF2 target.

### Key derivation and client-final message

Per RFC 5802 §3 with SHA-512, exactly as specified in
`scram_authentication.rst`:

```
SaltedPassword  = PBKDF2-HMAC-SHA512(secret, salt, iterations, 64)
ClientKey       = HMAC-SHA512(SaltedPassword, "Client Key")
StoredKey       = SHA512(ClientKey)
AuthMessage     = client-first-bare "," server-first "," client-final-without-proof
ClientSignature = HMAC-SHA512(StoredKey, AuthMessage)
ClientProof     = ClientKey XOR ClientSignature
```

- `secret` is the API key's random part — everything after the `<id>-`
  prefix — matching the usage example in the middleware documentation.
- PBKDF2 comes from the Go standard library (`crypto/pbkdf2`,
  FIPS 140-3 module material in Go ≥ 1.24), not a hand-rolled loop.
- `client-first-bare` and `server-first` enter `AuthMessage` verbatim as
  transmitted/received (no re-serialization), so attribute ordering or
  encoding differences cannot corrupt the signature.

The client-final message is:

```
c=biws,r=<server combined nonce, verbatim>,p=<base64 ClientProof>
```

`biws` is base64 of the GS2 header `n,,`, consistent with the unbound
exchange (RFC 5802 requires the `c=` attribute to carry the GS2 header the
client actually sent).

### Server-final message and mutual authentication

```
v=<base64 ServerSignature>
```

The client computes

```
ServerKey       = HMAC-SHA512(SaltedPassword, "Server Key")
ServerSignature = HMAC-SHA512(ServerKey, AuthMessage)
```

and compares against the server's value with `crypto/hmac.Equal`
(constant-time). A mismatch fails the login even though the server has
already accepted the session — a server that cannot produce the signature
does not know the key, and the client refuses to proceed. This is the
mutual-authentication property SCRAM exists for.

## Security properties claimed

| Property | How |
|---|---|
| Raw key never transmitted (SCRAM path) | Only nonces, salt parameters, and HMAC-derived proofs cross the wire |
| Mutual authentication | `v=` signature verified constant-time; failure aborts |
| Replay resistance | 32-byte `crypto/rand` nonce per exchange; server extends it; continuity verified on decoded bytes |
| No mechanism downgrade | SCRAM failure is terminal — no silent PLAIN retry |
| Parameter tampering bounded | Client rejects out-of-range iterations (50k floor), wrong-size salt, malformed nonces |
| No secret in error paths | Errors carry protocol context only; the secret and derived keys never appear in messages or logs |

Not claimed: resistance to a TLS-terminating man-in-the-middle. That
requires channel binding (see below).

## Known limitations

- **No channel binding (SCRAM-PLUS).** The client always sends `n,,` and
  relies on the server's negotiate mode. Binding to
  `tls-server-end-point` per RFC 5929 is the natural next step; the client
  has access to the peer certificate via the TLS connection state, and the
  middleware already publishes the binding for `pam_truenas`. Until then,
  TLS itself is the only defense against a TLS-terminating proxy — the
  same position as every pre-SCRAM client.
- **No precomputed-key path.** TrueNAS 26 returns
  `client_key`/`stored_key`/`server_key` at key creation and offers
  `api_key.convert_raw_key`; using them would skip the 500k-iteration
  PBKDF2 (~1s) per login. The provider currently accepts only the raw
  `<id>-<secret>` form and derives keys per session.
- **API keys only.** Password SCRAM (if/when offered) is not implemented.

## Testing

The project's testing policy is **no mocks**: correctness of the exchange
is judged by the real middleware, never by a simulated server. Test code
divides accordingly into pure-function input tests (no server involved,
simulated or otherwise) and live tests against real TrueNAS boxes.

### Unit: pure-function input validation

`scram_internal_test.go` feeds crafted strings and bytes into the client's
message builders and parsers — no server, real or simulated, participates:

| Test | Asserts |
|---|---|
| `TestScramConversation_ClientFirstWireForm` | Exact client-first bytes: GS2 `n,,` header, `username:key_id` identity, base64 nonce; `client-first-bare` retained for `AuthMessage` |
| `TestScramConversation_BadServerSignatureRejected` | A syntactically valid but wrong `v=`, and a malformed server-final, are both rejected (mutual-auth enforcement) |
| `TestScramConversation_NonceMismatchRejected` | A combined nonce that does not byte-extend the client's own is rejected |
| `TestScramConversation_BadServerFirstRejected` | Iterations below 50k / above 5M, non-16-byte salt, non-base64 salt or nonce — each rejected before key derivation |
| `TestSplitAPIKey` | `<id>-<secret>` parsing, malformed-key rejection |

### Live: the real middleware is the referee

`scram_live_test.go` (env-gated: `TF_ACC`, `TRUENAS_ENDPOINT`,
`TRUENAS_API_KEY`, `TRUENAS_USERNAME`) runs full exchanges against a real
box:

- `TestLiveSCRAM` — the positive path: mechanism discovery, full
  exchange, server-signature verification, then an authenticated
  `system.version_short` call. On **TrueNAS 26.0.0-BETA.2**
  (`auth.mechanism_choices` = `[API_KEY_PLAIN, TOKEN_PLAIN,
  PASSWORD_PLAIN, SCRAM]`) this passes; on **25.10.3.1** the mechanism
  probe fails pre-auth and the test skips.
- `TestLiveSCRAM_WrongCredentialsRejected` — the negative half, judged by
  the server itself: a proof built from a corrupted secret (same key ID)
  is rejected, and the correct key presented under a wrong username is
  rejected. Both verified against 26.0.0-BETA.2.

The cryptographic pipeline (PBKDF2, ClientKey/StoredKey, proof, server
signature) therefore has its correctness established end-to-end against
the production implementation: a genuine exchange succeeds *and* a
single-character perturbation of the secret fails.

### Acceptance: provider end-to-end

With `TRUENAS_USERNAME` set, every provider `Configure` in the acceptance
suite authenticates via the auto-selection path. Verified runs:

- `truenas_dataset` full create/update/import cycle over SCRAM against
  26.0.0-BETA.2 (each Terraform test step opens a fresh session, so the
  exchange runs several times per test);
- the same test against 25.10.3.1 over the PLAIN fallback, confirming the
  selection logic degrades cleanly on servers without SCRAM.

### What testing does not cover

- Channel-binding exchanges (not implemented).
- A live server that *lies* in its final message (wrong `v=`): a real
  middleware never produces one, so client-side rejection of a bad server
  signature is covered by the pure input test above rather than live.

## Empirical findings fed back

Two behaviors surfaced during implementation that the middleware team may
want to note:

1. `auth.mechanism_choices` raises
   `'NoneType' object has no attribute 'may_create_auth_token'` when called
   unauthenticated on 25.10.3.1. Harmless for us (we treat any failure as
   "no SCRAM"), but it looks unintended for a method that is callable
   pre-auth on 26.0.
2. The documentation example in `scram_authentication.rst` (Go section)
   validates the server nonce with a string `HasPrefix` on base64 forms.
   Against the real server this always fails once the client nonce's byte
   length (32) is not a multiple of 3 — the combined nonce re-encodes
   differently. The check must be performed on decoded bytes (as
   `truenas_scram` itself defines the nonce as binary data).
