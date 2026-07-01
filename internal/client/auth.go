package client

import (
	"context"
	"encoding/json"
	"fmt"
)

// AuthAPIKey authenticates using a TrueNAS API key.
// The API key format is "<id>-<secret>", e.g. "1-abcdef1234".
func AuthAPIKey(ctx context.Context, c *Client, apiKey string) error {
	raw, err := c.Call(ctx, "auth.login_with_api_key", apiKey)
	if err != nil {
		return fmt.Errorf("auth.login_with_api_key: %w", err)
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}
	if !ok {
		return fmt.Errorf("authentication rejected: invalid API key")
	}
	return nil
}

// AuthPassword authenticates using a username and password.
func AuthPassword(ctx context.Context, c *Client, username, password string) error {
	raw, err := c.Call(ctx, "auth.login", username, password)
	if err != nil {
		return fmt.Errorf("auth.login: %w", err)
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}
	if !ok {
		return fmt.Errorf("authentication rejected: invalid username or password")
	}
	return nil
}
