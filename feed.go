package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	rd "github.com/cdzombak/raindrop-io-api-client/pkg/raindrop"
	"github.com/mmcdole/gofeed"
	"github.com/mmcdole/gofeed/rss"
)

const (
	feedTitle       = "Raindrop Public Bookmarks"
	feedLink        = "https://raindrop.io/"
	feedDescription = `Bookmarks tagged "_public"`
)

// buildFeed constructs a universal gofeed.Feed from the given raindrops, which
// are expected to be ordered newest-first.
//
// Each bookmark's description (Raindrop's "excerpt") is stored in both the
// universal Content and Description fields: the RSS converter renders
// Description as <description>, while the Atom and JSON converters render
// Content as the entry content. Setting both makes every output format carry
// the description.
func buildFeed(drops []rd.Raindrop, now time.Time) *gofeed.Feed {
	feed := &gofeed.Feed{
		Title:       feedTitle,
		Link:        feedLink,
		Description: feedDescription,
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

// writeFeed renders the feed in the requested format and writes it atomically to
// outFile, so a reader (or web server) never observes a partially written feed.
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
	return atomicWriteFile(outFile, buf.Bytes(), 0o644)
}

// atomicWriteFile writes data to a temp file in the destination directory, then
// renames it over path.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".feed-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing feed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing feed: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("setting feed permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("moving feed into place: %w", err)
	}
	return nil
}
