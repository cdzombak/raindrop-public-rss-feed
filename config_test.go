package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigFull(t *testing.T) {
	path := writeConfig(t, `
tag: blog
count: 30
format: atom
feed:
  title: My Links
  description: Stuff I liked
  link: https://example.com/
  feed_url: https://example.com/feed.xml
  author: Jane Doe
  language: en-US
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Tag != "blog" || cfg.Count != 30 || cfg.Format != "atom" {
		t.Errorf("tag/count/format = %q/%d/%q", cfg.Tag, cfg.Count, cfg.Format)
	}
	if cfg.Feed.Title != "My Links" || cfg.Feed.FeedURL != "https://example.com/feed.xml" ||
		cfg.Feed.Author != "Jane Doe" || cfg.Feed.Language != "en-US" {
		t.Errorf("feed metadata = %+v", cfg.Feed)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	// Only a title is set; everything else should fall back to a default.
	path := writeConfig(t, "feed:\n  title: Minimal\n")
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Tag != defaultTag {
		t.Errorf("tag = %q, want default %q", cfg.Tag, defaultTag)
	}
	if cfg.Count != defaultCount {
		t.Errorf("count = %d, want default %d", cfg.Count, defaultCount)
	}
	if cfg.Format != defaultFormat {
		t.Errorf("format = %q, want default %q", cfg.Format, defaultFormat)
	}
	if cfg.Feed.Link != defaultFeedLink {
		t.Errorf("link = %q, want default %q", cfg.Feed.Link, defaultFeedLink)
	}
	// The description default mentions the (defaulted) tag.
	if !strings.Contains(cfg.Feed.Description, defaultTag) {
		t.Errorf("description = %q, want it to mention %q", cfg.Feed.Description, defaultTag)
	}
}

func TestLoadConfigUnknownKeyRejected(t *testing.T) {
	path := writeConfig(t, "tittle: typo\n")
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected an error for an unknown key, got nil")
	}
}

func TestLoadConfigBadCount(t *testing.T) {
	path := writeConfig(t, "count: 99\n")
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected an error for out-of-range count, got nil")
	}
}

func TestLoadConfigBadFormat(t *testing.T) {
	path := writeConfig(t, "format: xml\n")
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected an error for an invalid format, got nil")
	}
}

func TestLoadConfigEmpty(t *testing.T) {
	path := writeConfig(t, "")
	if _, err := loadConfig(path); err == nil {
		t.Fatal("expected an error for an empty config, got nil")
	}
}

func TestLoadConfigMissing(t *testing.T) {
	if _, err := loadConfig(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Fatal("expected an error for a missing config, got nil")
	}
}
