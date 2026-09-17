// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Command token_probe tests whether a cached auth.generate_token token
// could replace per-connection API-key logins (to dodge the middleware's
// auth rate limit). It authenticates once with an API key, generates a
// session token, then opens fresh connections in a loop authenticating each
// with the token.
//
// RESULTS (TrueNAS 26.0, 2026-07-07) — token caching is UNWORKABLE:
//
//  1. auth.login_with_token draws from the SAME per-IP rate bucket as
//     auth.login_with_api_key: a 25-iteration loop hit
//     "[EBUSY] Rate Limit Exceeded" (code 16) from iteration 21.
//  2. A token authenticates exactly ONE new session: with the minting
//     session closed OR held open, iteration 1 succeeds and every
//     subsequent login_with_token returns false — despite
//     single_use defaulting to false.
//
// Conclusion: there is no client-side way to amortize logins across
// provider processes; the auth rate limit must be managed by making fewer
// Terraform invocations and pacing test runs (see TESTING.md).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

func main() {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if endpoint == "" || apiKey == "" {
		log.Fatal("set TRUENAS_ENDPOINT and TRUENAS_API_KEY")
	}
	tlsCfg, _ := client.BuildTLSConfig(true, "")

	// 1. One key-authenticated session to mint a token.
	c := client.New(endpoint, tlsCfg)
	if err := c.Connect(context.Background(), func(ctx context.Context) error {
		return client.AuthAPIKey(ctx, c, apiKey)
	}); err != nil {
		log.Fatal("key auth: ", err)
	}
	raw, err := c.Call(context.Background(), "auth.generate_token", 600)
	if err != nil {
		log.Fatal("generate_token: ", err)
	}
	var token string
	if err := json.Unmarshal(raw, &token); err != nil {
		log.Fatal("decode token: ", err)
	}
	// NOTE: keep the minting session OPEN — run 1 showed tokens die with
	// their parent session (iters 2+ returned false after c.Close()).
	defer c.Close()
	fmt.Printf("token minted (len %d)\n", len(token))

	// 2. Hammer: 5 fresh connections, each logging in with the token.
	ok, fail := 0, 0
	start := time.Now()
	for i := 1; i <= 5; i++ {
		cc := client.New(endpoint, tlsCfg)
		err := cc.Connect(context.Background(), func(ctx context.Context) error {
			res, err := cc.Call(ctx, "auth.login_with_token", token)
			if err != nil {
				return err
			}
			var authed bool
			if err := json.Unmarshal(res, &authed); err != nil || !authed {
				return fmt.Errorf("login_with_token returned %s", res)
			}
			return nil
		})
		if err != nil {
			fail++
			fmt.Printf("iter %2d: FAIL: %v\n", i, err)
		} else {
			ok++
			_ = cc.Close()
		}
	}
	fmt.Printf("done in %s: %d ok, %d fail\n", time.Since(start).Round(time.Millisecond), ok, fail)
}
