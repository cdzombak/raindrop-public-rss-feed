# raindrop-public-rss-feed

A small Go program that finds the most recently created [Raindrop.io](https://raindrop.io)
bookmarks tagged `_public`, across your entire account, and writes them out as an
RSS, Atom, or JSON feed.

Each feed item uses the bookmark's title, URL (as both the link and the GUID),
and description (as the item content). The feed is written atomically to
`-out-file`, so a web server never serves a half-written feed.

It authenticates via OAuth using
[cdzombak/raindrop-io-api-client](https://github.com/cdzombak/raindrop-io-api-client),
and builds the feed with
[cdzombak/gofeed](https://github.com/cdzombak/gofeed) (the `cdz/feed-creation`
branch, which adds feed generation; wired in via a `replace` directive in
`go.mod`).
You authenticate **once** interactively; after that the refresh token is stored
and the program renews access tokens on its own, so it can run non-interactively
(e.g. from cron).

## Setup

1. Create an app at <https://app.raindrop.io/settings/integrations> to get a
   **client ID** and **client secret**.

   In the app settings, set the **Redirect URI** to:

   ```
   http://localhost:8080/oauth
   ```

   This must match the program's `-redirect-uri` flag **exactly** — same scheme,
   host, port, and path — because the `-login` flow starts a temporary local
   server at that address to receive the OAuth callback. `http://localhost` is
   accepted by Raindrop for this purpose. If you want a different address, set it
   in both places (the Raindrop app settings and `-redirect-uri`). Raindrop
   allows multiple redirect URIs per app, so you can register more than one.

2. Authenticate once. The client credentials are read from the environment (or
   `-client-id` / `-client-secret`) and stored in the state file:

   ```sh
   export RAINDROP_CLIENT_ID=...
   export RAINDROP_CLIENT_SECRET=...
   go run . -oauth-state .oauth.json -login
   ```

   This opens Raindrop's authorization page in your browser and writes the
   resulting tokens to the state file.

## Usage

Once authenticated, run without `-login` to generate the feed:

```sh
# 20 most recent, RSS (defaults):
go run . -oauth-state .oauth.json -out-file public.xml

# 50 most recent, JSON Feed:
go run . -oauth-state .oauth.json -out-file public.json -format json -n 50
```

This is the form to put in cron. No environment variables are needed for
non-interactive runs — the state file is self-sufficient.

### Flags

| Flag             | Required            | Description                                                            |
| ---------------- | ------------------- | ---------------------------------------------------------------------- |
| `-oauth-state`   | yes                 | Path to the JSON OAuth state file. Created by `-login` if missing.     |
| `-out-file`      | yes (unless `-login`) | Path to write the output feed to. Written atomically.               |
| `-n`             | no                  | Number of bookmarks to include (1–50; default 20).                    |
| `-format`        | no                  | Feed format: `rss`, `atom`, or `json` (default `rss`).                |
| `-login`         | no                  | Run the interactive OAuth login flow and persist the result.          |
| `-client-id`     | no                  | App client ID for `-login` (defaults to `$RAINDROP_CLIENT_ID`).        |
| `-client-secret` | no                  | App client secret for `-login` (defaults to `$RAINDROP_CLIENT_SECRET`).|
| `-redirect-uri`  | no                  | OAuth redirect URI; must match your app settings. Default `http://localhost:8080/oauth`. |

## The OAuth state file

The file stores your client credentials and the current token set, so keep it
private (it is written with `0600` permissions):

```json
{
  "client_id": "...",
  "client_secret": "...",
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expires_at": "2026-08-06T12:00:00Z"
}
```

Running without `-login` when this file is missing or empty is an error.
