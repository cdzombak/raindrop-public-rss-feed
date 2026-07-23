package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "c"); got != "c" {
		t.Errorf("got %q, want %q", got, "c")
	}
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("got %q, want %q", got, "a")
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := OAuthState{
		ClientID:     "id",
		ClientSecret: "secret",
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		ExpiresAt:    time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
	}
	if err := saveState(path, want); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	// File is created (including parent dirs) with 0600 permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}

	got, err := loadState(path)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if got.ClientID != want.ClientID || got.ClientSecret != want.ClientSecret ||
		got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken ||
		got.TokenType != want.TokenType || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", got, want)
	}
}

func TestLoadStateMissingOrEmpty(t *testing.T) {
	if _, err := loadState(filepath.Join(t.TempDir(), "does-not-exist.json")); !errors.Is(err, errStateMissing) {
		t.Errorf("missing file: got %v, want errStateMissing", err)
	}

	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadState(empty); !errors.Is(err, errStateMissing) {
		t.Errorf("empty file: got %v, want errStateMissing", err)
	}
}
