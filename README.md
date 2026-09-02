# Tuinear

Tuinear is a fast, keyboard-first terminal client for browsing and safely
editing Linear issues. It combines a focused three-pane browser with guarded
field editing and recoverable issue archiving.

## MVP

- Three-pane team, issue, and detail layout
- Vim and arrow-key navigation
- Team filtering with `tab`, `[` and `]`
- Incremental identifier/title search with `/`
- Composable assignee, status, priority, and project filters via `f`
- Loading, empty, and actionable error states
- Manual refresh with `r`
- Demo and snapshot modes that need no Linear account
- Browser-based OAuth login with PKCE and automatic token refresh
- Multiple simultaneous workspace/user profiles for work and personal accounts
- Instant cached startup with background refresh and offline fallback
- Press `space` while browsing to open the selected issue's Linear URL in your default browser
- Press `enter` to discover every action available for the selected issue
- Edit the selected issue's title with `e`, including optimistic feedback and exact rollback on failure
- Change the selected issue's team-specific status with `s`
- Change the selected issue's priority with `p`
- Assign or unassign the selected issue with `u`
- Move the selected issue into or out of a team project with `P`
- Apply or clear multiple labels with `l`
- Edit multiline descriptions with `d`
- Archive the selected issue with `x` after a focused confirmation that
  defaults to cancellation

The editing and archive backlog is complete. Archive is recoverable from Linear;
Tuinear deliberately does not expose permanent issue deletion. The
implementation record lives in [`.tickets`](.tickets/README.md).

## OAuth setup

Tuinear uses Linear OAuth with PKCE and requests the standard `read,write`
scopes. Tokens are stored in your operating system's credential store; a
client secret is neither needed nor accepted.

Tuinear ships with its OAuth client ID, so no environment variable is needed.
The registered application uses:

1. Use `http://127.0.0.1:14565/oauth/callback` as the redirect URL.
2. Enable the authorization-code grant.
3. The configured public client ID is `3c2a2e12d13e32eaaa0a3d69de27aa61`.

Public distribution is needed for the same client ID to connect both your work
and personal workspaces. A managed work workspace may require administrator
approval before it can authorize the app.

Profiles connected by an older read-only Tuinear build must be connected again
with `tuinear --login` before editing. Browsing continues to work without
reconnecting.

The client ID is public configuration, not a secret. Developers can override it
with `TUINEAR_OAUTH_CLIENT_ID` or with
`-ldflags "-X main.oauthClientID=YOUR_CLIENT_ID"`.

## Run

Requires Go 1.26 or newer.

```sh
go run ./cmd/tuinear
```

The first run opens Linear in your browser. Every additional login is saved as
a separate workspace/user profile and becomes active:

```sh
go run ./cmd/tuinear --login
go run ./cmd/tuinear --accounts
go run ./cmd/tuinear --profile personal
go run ./cmd/tuinear --profile jamie@company.test
go run ./cmd/tuinear --logout --profile personal
```

`--accounts` prints stable profile IDs for the rare case where a workspace or
user name is ambiguous. The chosen profile remains active on later launches.
Connected accounts also appear beneath the team list in the TUI. Press `a` to
switch to the next account or `A` to switch to the previous account; tickets
reload in place.

## Cache and offline use

Tuinear stores normalized ticket data in a local SQLite database under your
operating system's user cache directory. OAuth tokens and saved account
profiles remain in the credential store and are never copied into SQLite.

On later launches, cached tickets appear immediately while Tuinear refreshes in
the background. The header marks cached or offline data and shows its age. A
successful refresh atomically replaces that account's snapshot; a failed
refresh leaves the last-known-good tickets available. Corrupt or incompatible
cache files are quarantined with a `.broken-<timestamp>` suffix and rebuilt.

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
| `a` / `A` | Next/previous account |
| `enter` | Open the selected issue's action menu |
| `e` | Edit the selected issue's title |
| `s` | Change the selected issue's status |
| `p` | Change the selected issue's priority |
| `u` | Change the selected issue's assignee |
| `P` | Change the selected issue's project |
| `l` | Edit the selected issue's labels |
| `d` | Edit the selected issue's multiline description |
| `x` | Confirm and archive the selected issue (recoverable) |
| `/` | Incremental search by identifier or title |
| `f` / `ctrl+f` | Open the discoverable filter palette |
| `space` | Open the selected issue in the default browser |
| `esc` | Clear an active search/filter, or close search/palette |
| `r` | Refresh |
| `q` / `ctrl+c` | Quit |

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/tuinear
```
