package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rd "github.com/cdzombak/raindrop-io-api-client/pkg/raindrop"
)

func sampleConfig() feedConfig {
	return feedConfig{
		Tag:    "_public",
		Count:  20,
		Format: "rss",
		Feed: feedMeta{
			Title:       "Test Feed",
			Description: "A test feed.",
			Link:        "https://example.com/",
			FeedURL:     "https://example.com/feed.xml",
			Author:      "Jane Doe",
			Language:    "en-US",
		},
	}
}

func sampleDrops() []rd.Raindrop {
	return []rd.Raindrop{
		{
			Title:   "First Bookmark",
			Link:    "https://example.com/1",
			Excerpt: "Description of the first bookmark.",
			Cover:   "https://example.com/1/cover.jpg",
			Created: "2026-07-20T10:00:00Z",
			Tags:    []string{"_public"},
		},
		{
			// No title: should fall back to the link.
			// No cover: the item should carry no image.
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
	feed := buildFeed(sampleDrops(), sampleConfig(), now)

	if len(feed.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(feed.Items))
	}
	// Channel metadata comes from the config.
	if feed.Title != "Test Feed" || feed.Link != "https://example.com/" ||
		feed.Description != "A test feed." || feed.FeedLink != "https://example.com/feed.xml" ||
		feed.Language != "en-US" {
		t.Errorf("channel metadata not taken from config: %+v", feed)
	}
	if len(feed.Authors) != 1 || feed.Authors[0].Name != "Jane Doe" {
		t.Errorf("author not taken from config: %+v", feed.Authors)
	}
	if feed.Generator != appName+" "+version {
		t.Errorf("generator = %q, want %q", feed.Generator, appName+" "+version)
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
	if feed.Items[0].Image == nil || feed.Items[0].Image.URL != "https://example.com/1/cover.jpg" {
		t.Errorf("cover should map to the item image, got %+v", feed.Items[0].Image)
	}
	if feed.Items[1].Image != nil {
		t.Errorf("item without a cover should have no image, got %+v", feed.Items[1].Image)
	}
}

func TestWriteFeedRSS(t *testing.T) {
	out := filepath.Join(t.TempDir(), "feed.xml")
	if err := writeFeed(buildFeed(sampleDrops(), sampleConfig(), time.Now()), "rss", out); err != nil {
		t.Fatalf("writeFeed: %v", err)
	}
	s := readFile(t, out)

	for _, want := range []string{
		"<title>First Bookmark</title>",
		"<link>https://example.com/1</link>",
		`<guid isPermaLink="true">https://example.com/1</guid>`,
		"<description>Description of the first bookmark.</description>",
		`<enclosure url="https://example.com/1/cover.jpg"`, // cover image
		"<language>en-US</language>",                       // from config
		"<managingEditor>Jane Doe</managingEditor>",        // author, from config
	} {
		if !strings.Contains(s, want) {
			t.Errorf("RSS output missing %q\n---\n%s", want, s)
		}
	}
}

func TestWriteFeedAtom(t *testing.T) {
	out := filepath.Join(t.TempDir(), "feed.atom")
	if err := writeFeed(buildFeed(sampleDrops(), sampleConfig(), time.Now()), "atom", out); err != nil {
		t.Fatalf("writeFeed: %v", err)
	}
	s := readFile(t, out)

	for _, want := range []string{
		"<id>https://example.com/1</id>", // GUID becomes the Atom entry id
		"Description of the first bookmark.",
		`href="https://example.com/1/cover.jpg" rel="enclosure"`, // cover image
		`href="https://example.com/feed.xml" rel="self"`,         // feed_url -> rel=self
		"Jane Doe", // author name
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Atom output missing %q\n---\n%s", want, s)
		}
	}
}

func TestWriteFeedJSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "feed.json")
	if err := writeFeed(buildFeed(sampleDrops(), sampleConfig(), time.Now()), "json", out); err != nil {
		t.Fatalf("writeFeed: %v", err)
	}

	var doc struct {
		Title       string `json:"title"`
		HomePageURL string `json:"home_page_url"`
		FeedURL     string `json:"feed_url"`
		Items       []struct {
			ID          string `json:"id"`
			URL         string `json:"url"`
			Title       string `json:"title"`
			ContentHTML string `json:"content_html"`
			Image       string `json:"image"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(readFile(t, out)), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// Channel metadata from the config.
	if doc.Title != "Test Feed" || doc.HomePageURL != "https://example.com/" ||
		doc.FeedURL != "https://example.com/feed.xml" {
		t.Errorf("JSON feed metadata = %q / %q / %q", doc.Title, doc.HomePageURL, doc.FeedURL)
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
	if it.Image != "https://example.com/1/cover.jpg" {
		t.Errorf("image = %q, want the cover URL", it.Image)
	}
	if doc.Items[1].Image != "" {
		t.Errorf("item without a cover should have no image, got %q", doc.Items[1].Image)
	}
}

func TestWriteFeedStdout(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	writeErr := writeFeed(buildFeed(sampleDrops(), sampleConfig(), time.Now()), "rss", "-")
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = orig

	if writeErr != nil {
		t.Fatalf("writeFeed: %v", writeErr)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "<rss") || !strings.Contains(string(out), "First Bookmark") {
		t.Errorf("stdout output doesn't look like the RSS feed:\n%s", out)
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
