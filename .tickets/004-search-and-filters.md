# 004 — Search and richer filters

Status: DONE

## User goal

Reach a known issue in seconds even in a large workspace.

## Planned behavior

- `/` opens incremental local search over identifier and title.
- Filters cover assignee, status, priority, and project.
- The command palette exposes every filter without memorized keys.
- The active filter is always visible and can be cleared with `esc`.
- Filters persist independently for each connected Linear profile.
- Positive and negative values compose, including multiple `NOT` values for a
  field.

## Acceptance

- Search remains responsive with at least 10,000 cached issues.
- Filter composition has table-driven tests.
- Empty results preserve the query and explain how to clear it.
- Saved filters survive dashboard refreshes and application restarts.
- Account switching never applies one profile's filters to another profile.

## Delivered

- `/` opens incremental local search over issue identifiers and titles.
- `f` opens a discoverable palette for assignee, status, priority, and project;
  selected values compose with one another.
- `enter` includes the selected value and `!` toggles it as a `NOT` value while
  leaving the palette open for additional exclusions.
- The palette always offers `Me (<viewer>)` and an **Active** preset that hides
  completed and canceled workflow states.
- Filter state is saved in the local cache per profile, restored on startup and
  account switching, and kept separate from replaceable dashboard snapshots.
- The search query and active filters remain visible, and `esc` clears an
  active query/filter state before closing search. Empty results explain how
  to clear the view.
