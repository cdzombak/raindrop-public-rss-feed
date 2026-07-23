package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rd "github.com/cdzombak/raindrop-io-api-client/pkg/raindrop"
)

// errStateMissing indicates the OAuth state file does not exist or is empty.
var errStateMissing = errors.New("oauth state missing or empty")

// OAuthState is everything needed to make authenticated, non-interactive
// Raindrop API calls: the app credentials plus the current token set. Storing
// the client credentials alongside the tokens keeps the state file
// self-sufficient, so cron runs need nothing but this file.
type OAuthState struct {
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// loadState reads and parses the OAuth state file. A missing or empty file is
// reported as errStateMissing so callers can distinguish it from a parse error.
func loadState(path string) (OAuthState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return OAuthState{}, errStateMissing
		}
		return OAuthState{}, fmt.Errorf("read oauth state: %w", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return OAuthState{}, errStateMissing
	}
	var s OAuthState
	if err := json.Unmarshal(b, &s); err != nil {
		return OAuthState{}, fmt.Errorf("parse oauth state %q: %w", path, err)
	}
	return s, nil
}

// loadStateOptional returns a zero OAuthState when the file is missing or empty,
// and an error only for genuine read/parse failures.
func loadStateOptional(path string) (OAuthState, error) {
	s, err := loadState(path)
	if errors.Is(err, errStateMissing) {
		return OAuthState{}, nil
	}
	return s, err
}

// saveState writes the state file, creating parent directories as needed. The
// file may contain a client secret and refresh token, so it is written 0600.
func saveState(path string, s OAuthState) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create oauth state dir: %w", err)
		}
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write oauth state: %w", err)
	}
	return nil
}

// applyToken updates the token portion of the state from an API token response.
func (s *OAuthState) applyToken(t *rd.AccessTokenResponse) {
	s.AccessToken = t.AccessToken
	if t.RefreshToken != "" {
		s.RefreshToken = t.RefreshToken
	}
	if t.TokenType != "" {
		s.TokenType = t.TokenType
	}
	if t.ExpiresIn > 0 {
		s.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	} else {
		s.ExpiresAt = time.Time{}
	}
}
