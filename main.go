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

	exitcode "github.com/cdzombak/exitcode_go"
)

const (
	// appName is the program name, used in the feed's generator field.
	appName = "raindrop-public-rss-feed"
	// maxBookmarks is the most bookmarks the feed can include (Raindrop's
	// per-page maximum).
	maxBookmarks = 50
)

// version is the program version, injected at build time via
// -ldflags="-X main.version=...". It defaults to a placeholder for `go run`
// and un-stamped builds.
var version = "<dev>"

// validFormats is the set of accepted -format values.
var validFormats = map[string]bool{"rss": true, "atom": true, "json": true}

// appConfig holds the parsed command-line configuration.
type appConfig struct {
	statePath    string
	configPath   string
	login        bool
	clientID     string
	clientSecret string
	redirectURI  string
	outFile      string
}

func main() {
	var cfg appConfig
	var showHelp, showVersion bool
	flag.BoolVar(&showHelp, "help", false, "Show this help and exit.")
	flag.BoolVar(&showVersion, "version", false, "Print the version and exit.")
	flag.StringVar(&cfg.statePath, "oauth-state", "", "Path to the JSON file storing OAuth state (refresh token, app credentials). Required. Created by -login if missing.")
	flag.StringVar(&cfg.configPath, "config", "", "Path to the YAML feed configuration file. Required unless -login is given. See config.example.yml.")
	flag.BoolVar(&cfg.login, "login", false, "Run the interactive OAuth login flow and persist the result to the -oauth-state file.")
	flag.StringVar(&cfg.clientID, "client-id", "", "Raindrop app client ID (login only; defaults to $RAINDROP_CLIENT_ID).")
	flag.StringVar(&cfg.clientSecret, "client-secret", "", "Raindrop app client secret (login only; defaults to $RAINDROP_CLIENT_SECRET).")
	flag.StringVar(&cfg.redirectURI, "redirect-uri", "http://localhost:8080/oauth", "OAuth redirect URI; must match your Raindrop app settings (login only).")
	flag.StringVar(&cfg.outFile, "out-file", "", "Path to write the output feed to. Required unless -login is given.")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(exitcode.Success)
	}

	if showHelp {
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
		os.Exit(exitcode.Success)
	}

	if err := validateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		os.Exit(exitcode.InvalidArgument)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if err := run(cfg, logger); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitcode.Failure)
	}
}

// validateConfig checks the argument combination. Errors here are usage errors.
func validateConfig(cfg appConfig) error {
	if cfg.statePath == "" {
		return errors.New("-oauth-state is required")
	}
	if cfg.login {
		// The feed flags and config don't apply to the login flow.
		return nil
	}
	if cfg.configPath == "" {
		return errors.New("-config is required unless -login is given")
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

	fc, err := loadConfig(cfg.configPath)
	if err != nil {
		return err
	}

	state, err := loadState(cfg.statePath)
	if err != nil {
		if errors.Is(err, errStateMissing) {
			return fmt.Errorf("OAuth state file %q is missing or empty; run once with -login to authenticate", cfg.statePath)
		}
		return err
	}
	return runSearch(cfg, fc, state, logger)
}
