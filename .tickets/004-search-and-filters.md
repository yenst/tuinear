# 004 — Search and richer filters

Status: READY

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

