package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	rd "github.com/cdzombak/raindrop-io-api-client/pkg/raindrop"
	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/rss"
)

// buildFeed constructs a universal gofeed.Feed from the given raindrops, which
// are expected to be ordered newest-first. Channel-level metadata (title, link,
// description, self URL, author, language) comes from the feed configuration.
//
// Each bookmark's description (Raindrop's "excerpt") is stored in both the
// universal Content and Description fields: the RSS converter renders
// Description as <description>, while the Atom and JSON converters render
// Content as the entry content. Setting both makes every output format carry
// the description.
func buildFeed(drops []rd.Raindrop, fc feedConfig, now time.Time) *gofeed.Feed {
	feed := &gofeed.Feed{
		Title:       fc.Feed.Title,
		Link:        fc.Feed.Link,
		Description: fc.Feed.Description,
		Generator:   fmt.Sprintf("%s %s", appName, version),
	}
	// FeedLink is rendered as rel="self" in Atom and as feed_url in JSON Feed.
	// (The RSS converter has no self-link field, so RSS output omits it.)
	if fc.Feed.FeedURL != "" {
		feed.FeedLink = fc.Feed.FeedURL
	}
	if fc.Feed.Language != "" {
		feed.Language = fc.Feed.Language
	}
	if fc.Feed.Author != "" {
		feed.Authors = []*gofeed.Person{{Name: fc.Feed.Author}}
	}

	var newest time.Time
	for _, d := range drops {
		title := d.Title
		if title == "" {
			title = d.Link
		}
		item := &gofeed.Item{
			Title:       title,
			Link:        d.Link,
			GUID:        d.Link,
			Content:     d.Excerpt,
			Description: d.Excerpt,
		}
		// Cover is the bookmark's thumbnail; not every bookmark has one. The
		// converters render Image as an RSS/Atom enclosure and as JSON Feed's
		// item image.
		if d.Cover != "" {
			item.Image = &gofeed.Image{URL: d.Cover}
		}
		if created, err := time.Parse(time.RFC3339, d.Created); err == nil {
			item.Published = d.Created
			item.PublishedParsed = &created
			if created.After(newest) {
				newest = created
			}
		}
		feed.Items = append(feed.Items, item)
	}

	if newest.IsZero() {
		newest = now
	}
	feed.Updated = newest.Format(time.RFC3339)
	feed.UpdatedParsed = &newest

	return feed
}

// permalinkRSSConverter wraps the default RSS converter to mark each item's GUID
// as a permalink. The bookmark URL is used as the GUID, so isPermaLink="true" is
// accurate; the default converter leaves the attribute unset.
type permalinkRSSConverter struct {
	gofeed.DefaultRSSConverter
}

func (c *permalinkRSSConverter) Convert(f *gofeed.Feed) (*rss.Feed, error) {
	rssFeed, err := c.DefaultRSSConverter.Convert(f)
	if err != nil {
		return nil, err
	}
	for _, item := range rssFeed.Items {
		if item.GUID != nil && item.GUID.Value != "" {
			item.GUID.IsPermalink = "true"
		}
	}
	return rssFeed, nil
}

// writeFeed renders the feed in the requested format and writes it to outFile.
// A regular path is written atomically, so a reader (or web server) never
// observes a partially written feed; the special path "-" writes to stdout.
func writeFeed(feed *gofeed.Feed, format, outFile string) error {
	var buf bytes.Buffer
	var err error
	switch format {
	case "rss":
		err = feed.RenderRSS(&buf, &permalinkRSSConverter{})
	case "atom":
		err = feed.RenderAtom(&buf, nil)
	case "json":
		err = feed.RenderJSON(&buf, nil)
	default:
		return fmt.Errorf("unknown feed format %q", format)
	}
	if err != nil {
		return fmt.Errorf("rendering %s feed: %w", format, err)
	}
	if outFile == "-" {
		_, err := os.Stdout.Write(buf.Bytes())
		return err
	}
	return atomicWriteFile(outFile, buf.Bytes(), 0o644)
}
