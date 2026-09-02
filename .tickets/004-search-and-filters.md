# 004 — Search and richer filters

Status: DONE

## User goal

Reach a known issue in seconds even in a large workspace.

## Planned behavior

- `/` opens incremental local search over identifier and title.
- Filters cover assignee, status, priority, and project.
- The command palette exposes every filter without memorized keys.
- The active filter is always visible and can be cleared with `esc`.

## Acceptance

- Search remains responsive with at least 10,000 cached issues.
- Filter composition has table-driven tests.
- Empty results preserve the query and explain how to clear it.

## Delivered

- `/` opens incremental local search over issue identifiers and titles.
- `f` opens a discoverable palette for assignee, status, priority, and project;
  selected values compose with one another.
- The search query and active filters remain visible, and `esc` clears an
  active query/filter state before closing search. Empty results explain how
  to clear the view.
