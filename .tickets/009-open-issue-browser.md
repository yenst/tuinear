# 009 — Open selected issue in browser

Status: DONE

## User goal

Open the selected Linear issue's online page without leaving the keyboard-first
issue browser.

## Acceptance criteria

- Pressing `space` in normal issue browsing mode launches the selected issue URL
  asynchronously in the default browser.
- `space` is ignored while search input or the filter palette is active.
- Missing selections, blank URLs, and non-HTTP(S) URLs never launch an external
  process.
- Browser launch failures remain visible while the cached or live dashboard is
  preserved.
- The launcher is injectable so tests do not open a real browser.

## Tests

- Covers successful launch target, empty selection/URL, invalid scheme, failure
  visibility and dashboard preservation, and search/palette mode isolation.
