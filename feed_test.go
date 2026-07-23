package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rd "github.com/cdzombak/raindrop-io-api-client/pkg/raindrop"
)

func sampleDrops() []rd.Raindrop {
	return []rd.Raindrop{
		{
			Title:   "First Bookmark",
			Link:    "https://example.com/1",
			Excerpt: "Description of the first bookmark.",
			Created: "2026-07-20T10:00:00Z",
			Tags:    []string{"_public"},
		},
		{
			// No title: should fall back to the link.
			Title:   "",
			Link:    "https://example.com/2",
			Excerpt: "Description of the second bookmark.",
			Created: "2026-07-19T09:30:00.500Z", // fractional seconds
			Tags:    []string{"_public"},
		},
	}
}

func TestBuildFeed(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	feed := buildFeed(sampleDrops(), now)

	if len(feed.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(feed.Items))
	}
	if feed.Items[1].Title != "https://example.com/2" {
		t.Errorf("missing-title item should fall back to link, got %q", feed.Items[1].Title)
	}
	// Description is used both as content and description so every format carries it.
	if feed.Items[0].Content != "Description of the first bookmark." ||
		feed.Items[0].Description != "Description of the first bookmark." {
		t.Errorf("excerpt not mapped to Content/Description: %+v", feed.Items[0])
	}
	if feed.Items[0].GUID != "https://example.com/1" {
		t.Errorf("GUID = %q, want the bookmark URL", feed.Items[0].GUID)
	}
	if feed.Items[1].PublishedParsed == nil {
		t.Errorf("fractional-second Created should parse")
	}
}

func TestWriteFeedRSS(t *testing.T) {
	out := filepath.Join(t.TempDir(), "feed.xml")
	if err := writeFeed(buildFeed(sampleDrops(), time.Now()), "rss", out); err != nil {
		t.Fatalf("writeFeed: %v", err)
	}
	s := readFile(t, out)

	for _, want := range []string{
		"<title>First Bookmark</title>",
		"<link>https://example.com/1</link>",
		`<guid isPermaLink="true">https://example.com/1</guid>`,
		"<description>Description of the first bookmark.</description>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("RSS output missing %q\n---\n%s", want, s)
		}
	}
}

func TestWriteFeedAtom(t *testing.T) {
	out := filepath.Join(t.TempDir(), "feed.atom")
	if err := writeFeed(buildFeed(sampleDrops(), time.Now()), "atom", out); err != nil {
		t.Fatalf("writeFeed: %v", err)
	}
	s := readFile(t, out)

	for _, want := range []string{
		"<id>https://example.com/1</id>", // GUID becomes the Atom entry id
		"Description of the first bookmark.",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Atom output missing %q\n---\n%s", want, s)
		}
	}
}

func TestWriteFeedJSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "feed.json")
	if err := writeFeed(buildFeed(sampleDrops(), time.Now()), "json", out); err != nil {
		t.Fatalf("writeFeed: %v", err)
	}

	var doc struct {
		Items []struct {
			ID          string `json:"id"`
			URL         string `json:"url"`
			Title       string `json:"title"`
			ContentHTML string `json:"content_html"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(readFile(t, out)), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(doc.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(doc.Items))
	}
	it := doc.Items[0]
	if it.ID != "https://example.com/1" || it.URL != "https://example.com/1" {
		t.Errorf("id/url = %q/%q, want the bookmark URL", it.ID, it.URL)
	}
	if it.Title != "First Bookmark" {
		t.Errorf("title = %q", it.Title)
	}
	if it.ContentHTML != "Description of the first bookmark." {
		t.Errorf("content_html = %q, want the description", it.ContentHTML)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
