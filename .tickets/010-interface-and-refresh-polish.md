# 010 — Interface and refresh polish

Status: DONE

## User goal

Keep the main screen calm, make shortcuts discoverable on demand, and ensure
refreshing or changing accounts gives visible, reliable feedback.

## Acceptance

- The normal footer stays short and `h` or `?` opens a centered keybinding
  overlay without changing the current selection.
- Enhanced-keyboard representations such as `shift+a` activate their intended
  uppercase shortcuts.
- An account with a cached snapshot can be selected while Linear is offline;
  its remote refresh follows in the background.
- Manual refresh visibly enters a refreshing state and exposes the failure
  reason without discarding cached tickets.
- Dashboard loading stays below Linear's per-request GraphQL complexity limit,
  including workspaces with several teams and their nested metadata.
- Tuinear sets its terminal window title and uses the terminal's ANSI palette
  and default background so live terminal theme changes carry through.

## Delivered

- Added a responsive two-column help overlay with a narrow-screen fallback.
- Centralized key matching across text and physical-keystroke forms.
- Added cache-first account selection with a tested network fallback when the
  target has no snapshot.
- Split the former monolithic dashboard query into bounded workspace, team
  discovery, per-team metadata, and issue requests.
- Reduced the default footer to primary discovery keys and made refresh errors
  actionable.
- Replaced fixed RGB backgrounds with transparent, terminal-native colors and
  set the window/tab title to `Tuinear`.
