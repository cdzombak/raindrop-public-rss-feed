// Command raindrop-public-rss-feed authenticates to Raindrop.io, finds the most
// recently created bookmarks carrying a configured tag ("_public" by default)
// across the whole account, and writes them out as an RSS, Atom, or JSON feed.
// The tag, item count, output format, and feed metadata are read from a YAML
// file given with -config.
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

// cliArgs holds the parsed command-line configuration.
type cliArgs struct {
	statePath    string
	configPath   string
	login        bool
	clientID     string
	clientSecret string
	redirectURI  string
	outFile      string
	verbose      bool
}

func main() {
	var args cliArgs
	var showHelp, showVersion bool
	flag.BoolVar(&showHelp, "help", false, "Show this help and exit.")
	flag.BoolVar(&showVersion, "version", false, "Print the version and exit.")
	flag.StringVar(&args.statePath, "oauth-state", "", "Path to the JSON file storing OAuth state (refresh token, app credentials). Required. Created by -login if missing.")
	flag.StringVar(&args.configPath, "config", "", "Path to the YAML feed configuration file. Required unless -login is given. See config.example.yml.")
	flag.BoolVar(&args.login, "login", false, "Run the interactive OAuth login flow and persist the result to the -oauth-state file.")
	flag.StringVar(&args.clientID, "client-id", "", "Raindrop app client ID (login only; defaults to $RAINDROP_CLIENT_ID).")
	flag.StringVar(&args.clientSecret, "client-secret", "", "Raindrop app client secret (login only; defaults to $RAINDROP_CLIENT_SECRET).")
	flag.StringVar(&args.redirectURI, "redirect-uri", "http://localhost:8080/oauth", "OAuth redirect URI; must match your Raindrop app settings (login only).")
	flag.StringVar(&args.outFile, "out-file", "", "Path to write the output feed to. Required unless -login is given.")
	flag.BoolVar(&args.verbose, "verbose", false, "Enable verbose (debug) logging to stderr.")
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

	if err := validateConfig(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		os.Exit(exitcode.InvalidArgument)
	}

	logLevel := slog.LevelWarn
	if args.verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	if err := run(args, logger); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitcode.Failure)
	}
}

// validateConfig checks the argument combination. Errors here are usage errors.
func validateConfig(args cliArgs) error {
	if args.statePath == "" {
		return errors.New("-oauth-state is required")
	}
	if args.login {
		// The feed flags and config don't apply to the login flow.
		return nil
	}
	if args.configPath == "" {
		return errors.New("-config is required unless -login is given")
	}
	if args.outFile == "" {
		return errors.New("-out-file is required unless -login is given")
	}
	return nil
}

func run(args cliArgs, logger *slog.Logger) error {
	if args.login {
		return runLogin(args, logger)
	}

	fc, err := loadConfig(args.configPath)
	if err != nil {
		return err
	}

	state, err := loadState(args.statePath)
	if err != nil {
		if errors.Is(err, errStateMissing) {
			return fmt.Errorf("OAuth state file %q is missing or empty; run once with -login to authenticate", args.statePath)
		}
		return err
	}
	return runSearch(args, fc, state, logger)
}
