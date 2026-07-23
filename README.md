# raindrop-public-rss-feed

A small Go program that finds your most recently created [Raindrop.io](https://raindrop.io)
bookmarks tagged `_public` (or any tag you configure) and writes them out as an
RSS, Atom, or JSON feed.

Each item carries the bookmark's title, URL, description, and cover image (if
any). The feed is written atomically, so a web server never serves a
half-written file.

You authenticate via OAuth **once**, interactively. After that the refresh token
is stored and the program renews access tokens on its own, so it can run
non-interactively (e.g. from cron).

## Installation

### macOS via Homebrew

```shell
brew install cdzombak/oss/raindrop-public-rss-feed
```

### Debian via Apt repository

Install my Debian repository if you haven't already:

```shell
sudo apt-get install ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://dist.cdzombak.net/deb.key | sudo gpg --dearmor -o /etc/apt/keyrings/dist-cdzombak-net.gpg
sudo chmod 0644 /etc/apt/keyrings/dist-cdzombak-net.gpg
echo -e "deb [signed-by=/etc/apt/keyrings/dist-cdzombak-net.gpg] https://dist.cdzombak.net/deb/oss any oss\n" | sudo tee -a /etc/apt/sources.list.d/dist-cdzombak-net.list > /dev/null
sudo apt update
```

Then install `raindrop-public-rss-feed` via `apt`:

```shell
sudo apt install raindrop-public-rss-feed
```

### Manual installation from build artifacts

Pre-built binaries for Linux and macOS on multiple architectures are attached to every [GitHub Release](https://github.com/cdzombak/raindrop-public-rss-feed/releases). Debian packages for each release are published as well.

### Build and install locally

```shell
git clone https://github.com/cdzombak/raindrop-public-rss-feed.git
cd raindrop-public-rss-feed
make build

cp out/raindrop-public-rss-feed $INSTALL_DIR
```

### Docker image

Multi-architecture images are published to [Docker Hub](https://hub.docker.com/r/cdzombak/raindrop-public-rss-feed) and [GHCR](https://github.com/cdzombak/raindrop-public-rss-feed/pkgs/container/raindrop-public-rss-feed), built `FROM scratch` (just the binary plus CA certificates).

The typical use is non-interactive feed generation, after authenticating once.
Keep your [config file](#configuration) alongside the state file in the mounted
directory. Both mounts must be writable: the feed is written atomically into
`/out`, and the state file is rewritten in place on each token refresh.

```shell
docker run --rm \
  -v /home/cdzombak/.config/raindrop-public-rss-feed:/state \
  -v /var/www/feeds:/out \
  cdzombak/raindrop-public-rss-feed:1 \
  -oauth-state /state/oauth.json \
  -config /state/config.yml \
  -out-file /out/public.xml
```

You can also run the one-time `-login` in the container. It prints the
authorization URL for you to open on your host, so no in-container browser is
needed. The image binds the OAuth callback server to all interfaces, so just
publish the callback port with `-p` (matching your `-redirect-uri`, default port
8080):

```shell
docker run --rm -it \
  -p 8080:8080 \
  -e RAINDROP_CLIENT_ID -e RAINDROP_CLIENT_SECRET \
  -v /home/cdzombak/.config/raindrop-public-rss-feed:/state \
  cdzombak/raindrop-public-rss-feed:1 \
  -oauth-state /state/oauth.json -login
```

Open the printed URL on the host; the redirect to `http://localhost:8080/oauth`
is forwarded into the container. (On Linux you can instead use `--network host`.)

## Setup

1. Create an app at <https://app.raindrop.io/settings/integrations> to get a
   **client ID** and **client secret**, and set its **Redirect URI** to
   `http://localhost:8080/oauth`.

   This must match the `-redirect-uri` flag **exactly** — the `-login` flow runs
   a temporary local server at that address to receive the OAuth callback. To use
   a different address, change both.

2. Authenticate once. Credentials are read from the environment (or
   `-client-id` / `-client-secret`) and saved to the state file:

   ```sh
   export RAINDROP_CLIENT_ID=...
   export RAINDROP_CLIENT_SECRET=...
   raindrop-public-rss-feed -oauth-state .oauth.json -login
   ```

   This opens Raindrop's authorization page in your browser and writes the
   resulting tokens to the state file.

## Configuration

The feed is described by a YAML file, passed with `-config` (required when
generating; not needed for `-login`). A minimal example:

```yaml
tag: _public
count: 20
format: rss
feed:
  title: "Chris Dzombak • Public Bookmarks"
  description: "Interesting links I've shared publicly."
  link: "https://www.dzombak.com/"
  feed_url: "https://www.dzombak.com/feeds/bookmarks.rss.xml"
  author: "Chris Dzombak"
  language: "en-US"
```

Every field is optional and falls back to a default. See
[`config.example.yml`](config.example.yml) for the full, commented reference.

| Key                | Default                     | Description                                          |
| ------------------ | --------------------------- | ---------------------------------------------------- |
| `tag`              | `_public`                   | Raindrop tag that marks a bookmark for the feed.     |
| `count`            | `20`                        | Number of bookmarks to include (1–50).               |
| `format`           | `rss`                       | Output feed format: `rss`, `atom`, or `json`.        |
| `feed.title`       | `Raindrop Public Bookmarks` | Feed title.                                          |
| `feed.description` | `Bookmarks tagged "<tag>"`  | Feed description / subtitle.                         |
| `feed.link`        | `https://raindrop.io/`      | The website the feed represents (home page).         |
| `feed.feed_url`    | —                           | Canonical URL of the feed itself (`rel="self"`). Rendered in Atom and JSON output only, not RSS. |
| `feed.author`      | —                           | Feed author.                                         |
| `feed.language`    | —                           | Feed language, as a BCP 47 code (e.g. `en-US`).      |

Unknown keys are rejected, so a typo fails loudly instead of being ignored.

## Usage

Once authenticated, run with `-config` and `-out-file` to generate the feed. The
format is set in the config file:

```sh
raindrop-public-rss-feed -oauth-state .oauth.json -config config.yml -out-file public.xml
```

This is the form to put in cron: the state and config files are self-sufficient,
so no environment variables are needed.

### Flags

`-help` prints usage and exits; `-version` prints the version and exits.

| Flag           | Required          | Description                                                        |
| -------------- | ----------------- | ----------------------------------------------------------------- |
| `-oauth-state` | yes               | Path to the JSON OAuth state file. Created by `-login` if missing. |
| `-config`      | unless `-login`   | Path to the YAML [feed configuration](#configuration).            |
| `-verbose`     | no                | Enable verbose (debug) logging to stderr.                         |

#### Login flags (`-login`)

Used only when authenticating with `-login`:

| Flag             | Required | Description                                                              |
| ---------------- | -------- | ------------------------------------------------------------------------ |
| `-login`         | no       | Run the interactive OAuth login flow and persist the result.             |
| `-client-id`     | no       | App client ID for `-login` (defaults to `$RAINDROP_CLIENT_ID`).          |
| `-client-secret` | no       | App client secret for `-login` (defaults to `$RAINDROP_CLIENT_SECRET`).  |
| `-redirect-uri`  | no       | OAuth redirect URI; must match your app settings. Default `http://localhost:8080/oauth`. |

#### Generate flags

Used when generating the feed (i.e. without `-login`):

| Flag        | Required | Description                                         |
| ----------- | -------- | --------------------------------------------------- |
| `-out-file` | yes      | Path to write the output feed to (written atomically), or `-` for stdout. |

The tag, item count, output format, and all feed metadata live in the
[config file](#configuration).

## Building from source

```sh
make build          # build for the current platform to ./out
make all            # cross-compile for macOS and Linux (amd64/arm64/armv7/armv6)
make package        # build binaries + .deb packages (requires fpm)
make test           # run the test suite
```

The build stamps the version (from `.version.sh`) into the binary, reported by
`-version`. `go run .` and un-stamped builds report `<dev>`.

## License

GPL-3.0; see [LICENSE](LICENSE).
