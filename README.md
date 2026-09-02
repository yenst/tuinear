# Tuinear

Tuinear is a fast, keyboard-first terminal client for browsing Linear issues.
The MVP is deliberately read-only: it focuses on making tickets pleasant to
scan before adding editing or destructive operations.

## MVP

- Three-pane team, issue, and detail layout
- Vim and arrow-key navigation
- Team filtering with `tab`, `[` and `]`
- Loading, empty, and actionable error states
- Manual refresh with `r`
- Demo and snapshot modes that need no Linear account
- Browser-based OAuth login with PKCE and automatic token refresh

Editing, archiving, and deletion are intentionally deferred. The implementation
plan lives in [`.tickets`](.tickets/README.md).

## OAuth setup

Tuinear uses Linear OAuth with PKCE. Tokens are stored in your operating
system's credential store; a client secret is neither needed nor accepted.

Until Tuinear has a published OAuth application, create a private OAuth app in
[Linear's API settings](https://linear.app/settings/api/applications/new):

1. Use `http://127.0.0.1:14565/oauth/callback` as the redirect URL.
2. Enable the authorization-code grant.
3. Copy its client ID and export it as `TUINEAR_OAUTH_CLIENT_ID`.

The client ID is public configuration, not a secret. A release build can embed
it with `-ldflags "-X main.oauthClientID=YOUR_CLIENT_ID"`.

## Run

Requires Go 1.26 or newer.

```sh
export TUINEAR_OAUTH_CLIENT_ID="YOUR_CLIENT_ID"
go run ./cmd/tuinear
```

The first run opens Linear in your browser. You can also control the session
explicitly:

```sh
go run ./cmd/tuinear --login
go run ./cmd/tuinear --logout
```

For development, `LINEAR_API_KEY` remains available as an explicit override.
Demo mode does not need credentials: `go run ./cmd/tuinear --demo`.

Generate a non-interactive preview:

```sh
go run ./cmd/tuinear --snapshot
```

## Keys

| Key | Action |
| --- | --- |
| `j` / `down` | Next issue |
| `k` / `up` | Previous issue |
| `g` / `home` | First issue |
| `G` / `end` | Last issue |
| `tab` / `]` | Next team |
| `shift+tab` / `[` | Previous team |
| `r` | Refresh |
| `q` / `ctrl+c` | Quit |

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/tuinear
```
