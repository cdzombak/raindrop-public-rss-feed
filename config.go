package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	// defaultTag is the Raindrop tag used when the config omits one.
	defaultTag = "_public"
	// defaultCount is the number of bookmarks included when the config omits one.
	defaultCount = 20
	// defaultFormat is the feed format used when the config omits one.
	defaultFormat = "rss"
	// defaultFeedTitle and defaultFeedLink are fallbacks for an unconfigured feed.
	defaultFeedTitle = "Raindrop Public Bookmarks"
	defaultFeedLink  = "https://raindrop.io/"
)

// feedConfig is the parsed -config YAML: which bookmarks to select, the output
// format, and how to describe the resulting feed.
type feedConfig struct {
	Tag    string   `yaml:"tag"`
	Count  int      `yaml:"count"`
	Format string   `yaml:"format"`
	Feed   feedMeta `yaml:"feed"`
}

// feedMeta is the channel-level metadata written into the output feed.
type feedMeta struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Link        string `yaml:"link"`
	FeedURL     string `yaml:"feed_url"`
	Author      string `yaml:"author"`
	Language    string `yaml:"language"`
}

// loadConfig reads, parses, and validates the feed configuration file. Unknown
// keys are rejected so typos surface as errors rather than being silently
// ignored. Omitted fields fall back to sensible defaults.
func loadConfig(path string) (feedConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return feedConfig{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var cfg feedConfig
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return feedConfig{}, fmt.Errorf("config %q is empty; see config.example.yml", path)
		}
		return feedConfig{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return applyConfigDefaults(cfg, path)
}

// applyConfigDefaults fills in omitted fields and validates the result.
func applyConfigDefaults(cfg feedConfig, path string) (feedConfig, error) {
	if cfg.Tag == "" {
		cfg.Tag = defaultTag
	}
	if cfg.Count == 0 {
		cfg.Count = defaultCount
	}
	if cfg.Count < 1 || cfg.Count > maxBookmarks {
		return feedConfig{}, fmt.Errorf("config %q: count must be between 1 and %d", path, maxBookmarks)
	}
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if !validFormats[cfg.Format] {
		return feedConfig{}, fmt.Errorf("config %q: format must be one of rss, atom, json (got %q)", path, cfg.Format)
	}
	if cfg.Feed.Title == "" {
		cfg.Feed.Title = defaultFeedTitle
	}
	if cfg.Feed.Link == "" {
		cfg.Feed.Link = defaultFeedLink
	}
	if cfg.Feed.Description == "" {
		cfg.Feed.Description = fmt.Sprintf("Bookmarks tagged %q", cfg.Tag)
	}
	return cfg, nil
}
