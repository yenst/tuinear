# 006a — Multiple workspace and user profiles

Status: DONE

## User goal

Keep work and personal Linear accounts connected at the same time without
copying tokens or repeatedly signing out in the browser.

## Behavior

- Every OAuth login is identified by both workspace ID and user ID.
- Each profile has an isolated access/refresh token in the OS credential store.
- A new login uses `prompt=consent` so Linear offers workspace selection.
- `--accounts` lists connected profiles and marks the active one.
- `--profile` selects by workspace name/key, user name/email, or stable ID.
- `--logout --profile ...` revokes only the selected profile.
- The selected profile remains active for the next launch.
- The TUI header shows the workspace and user returned by Linear.
- Accounts render beneath teams; `a`/`A` switches forward/backward and reloads
  tickets in place.

## Acceptance

- Two users in one workspace and one user in two workspaces remain distinct.
- Refreshing one account cannot overwrite another account's token.
- Ambiguous human-readable selectors require the stable profile ID.
- Removing the active profile selects a remaining profile safely.
- Account metadata and tokens remain in the OS credential store.
