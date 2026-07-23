// Command raindrop-public-rss-feed authenticates to Raindrop.io, finds the most
// recently created bookmarks tagged "_public" across the whole account, and
// writes them out as an RSS, Atom, or JSON feed.
//
// Authentication uses OAuth via github.com/cdzombak/raindrop-io-api-client. You
// authenticate once interactively with -login; the resulting refresh token (and
// app credentials) are persisted to the -oauth-state file so that later,
// non-interactive runs (e.g. from cron) can refresh access tokens on their own.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

const (
	// publicTag is the Raindrop tag that marks a bookmark as public.
	publicTag = "_public"
	// maxBookmarks is the most bookmarks the feed can include (Raindrop's
	// per-page maximum).
	maxBookmarks = 50
	// defaultBookmarks is the default number of bookmarks in the feed.
	defaultBookmarks = 20
)

// validFormats is the set of accepted -format values.
var validFormats = map[string]bool{"rss": true, "atom": true, "json": true}

// appConfig holds the parsed command-line configuration.
type appConfig struct {
	statePath    string
	login        bool
	clientID     string
	clientSecret string
	redirectURI  string
	n            int
	format       string
	outFile      string
}

func main() {
	var cfg appConfig
	flag.StringVar(&cfg.statePath, "oauth-state", "", "Path to the JSON file storing OAuth state (refresh token, app credentials). Required. Created by -login if missing.")
	flag.BoolVar(&cfg.login, "login", false, "Run the interactive OAuth login flow and persist the result to the -oauth-state file.")
	flag.StringVar(&cfg.clientID, "client-id", "", "Raindrop app client ID (login only; defaults to $RAINDROP_CLIENT_ID).")
	flag.StringVar(&cfg.clientSecret, "client-secret", "", "Raindrop app client secret (login only; defaults to $RAINDROP_CLIENT_SECRET).")
	flag.StringVar(&cfg.redirectURI, "redirect-uri", "http://localhost:8080/oauth", "OAuth redirect URI; must match your Raindrop app settings (login only).")
	flag.IntVar(&cfg.n, "n", defaultBookmarks, "Number of bookmarks to include in the feed (max 50).")
	flag.StringVar(&cfg.format, "format", "rss", "Feed output format: rss, atom, or json.")
	flag.StringVar(&cfg.outFile, "out-file", "", "Path to write the output feed to. Required unless -login is given.")
	flag.Parse()

	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if err := run(cfg, logger); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// validateConfig checks the argument combination. Errors here are usage errors.
func validateConfig(cfg appConfig) error {
	if cfg.statePath == "" {
		return errors.New("-oauth-state is required")
	}
	if cfg.login {
		// The feed flags don't apply to the login flow.
		return nil
	}
	if cfg.n < 1 || cfg.n > maxBookmarks {
		return fmt.Errorf("-n must be between 1 and %d", maxBookmarks)
	}
	if !validFormats[cfg.format] {
		return fmt.Errorf("-format must be one of rss, atom, json (got %q)", cfg.format)
	}
	if cfg.outFile == "" {
		return errors.New("-out-file is required unless -login is given")
	}
	return nil
}

func run(cfg appConfig, logger *slog.Logger) error {
	if cfg.login {
		return runLogin(cfg, logger)
	}

	state, err := loadState(cfg.statePath)
	if err != nil {
		if errors.Is(err, errStateMissing) {
			return fmt.Errorf("OAuth state file %q is missing or empty; run once with -login to authenticate", cfg.statePath)
		}
		return err
	}
	return runSearch(cfg, state, logger)
}
