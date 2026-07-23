package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	rd "github.com/cdzombak/raindrop-io-api-client/pkg/raindrop"
)

const (
	// allRaindropsURL targets collection 0, which spans every collection in the
	// account (excluding Trash).
	allRaindropsURL = "https://api.raindrop.io/rest/v1/raindrops/0"
	// tokenExpiryBuffer refreshes slightly early to avoid using a token that
	// expires mid-request.
	tokenExpiryBuffer = 60 * time.Second
)

// runSearch refreshes the access token if needed, finds the most recent
// bookmarks matching the configured tag, and writes them out as a feed.
func runSearch(cfg appConfig, fc feedConfig, state OAuthState, logger *slog.Logger) error {
	client, err := rd.NewClientWithLogger(state.ClientID, state.ClientSecret, "", logger)
	if err != nil {
		return err
	}

	accessToken, err := ensureAccessToken(client, cfg.statePath, &state)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	drops, err := searchTaggedRaindrops(ctx, accessToken, fc.Tag, fc.Count)
	if err != nil {
		return err
	}

	feed := buildFeed(drops, fc, time.Now())
	if err := writeFeed(feed, cfg.format, cfg.outFile); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote %d item(s) as %s to %s\n", len(feed.Items), cfg.format, cfg.outFile)
	return nil
}

// ensureAccessToken returns a valid access token, refreshing and persisting a new
// one when the stored token is missing or (about to be) expired.
func ensureAccessToken(client *rd.Client, statePath string, state *OAuthState) (string, error) {
	if state.AccessToken != "" && !state.ExpiresAt.IsZero() && time.Now().Before(state.ExpiresAt.Add(-tokenExpiryBuffer)) {
		return state.AccessToken, nil
	}
	if state.RefreshToken == "" {
		return "", fmt.Errorf("no valid access token and no refresh token; run with -login to re-authenticate")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, err := client.RefreshAccessToken(state.RefreshToken, ctx)
	if err != nil {
		return "", fmt.Errorf("refreshing access token: %w", err)
	}
	if tok.AccessToken == "" {
		msg := firstNonEmpty(tok.ErrorMessage, tok.Error, "unknown error")
		return "", fmt.Errorf("token refresh failed: %s (run with -login to re-authenticate)", msg)
	}

	state.applyToken(tok)
	if err := saveState(statePath, *state); err != nil {
		return "", fmt.Errorf("persisting refreshed token: %w", err)
	}
	return state.AccessToken, nil
}

// searchTaggedRaindrops fetches up to limit most-recently-created raindrops that
// carry tag, across the entire account.
//
// The library's data methods don't expose sort or perpage, so we issue the
// request directly and decode into the library's response model.
func searchTaggedRaindrops(ctx context.Context, accessToken, tag string, limit int) ([]rd.Raindrop, error) {
	u, err := url.Parse(allRaindropsURL)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("search", fmt.Sprintf(`[{"key":"tag","val":%q}]`, tag))
	q.Set("sort", "-created")
	q.Set("perpage", strconv.Itoa(limit))
	q.Set("page", "0")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raindrop search returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out rd.MultiRaindropsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decoding raindrop search response: %w", err)
	}
	if !out.Result {
		return nil, fmt.Errorf("raindrop search was unsuccessful")
	}
	return out.Items, nil
}
