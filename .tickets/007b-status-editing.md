# 007b — Team-specific status editing

Status: DONE

## User goal

Move the selected issue through its team's workflow without leaving the
terminal.

## Behavior

- `s` opens a status picker for the selected issue.
- Only workflow states belonging to that issue's team are offered, in workflow
  order.
- `j`/`k`, arrows, Tab/Shift+Tab, and Home/End navigate; Enter saves and Escape
  cancels.
- The selected status is applied optimistically and replaced by Linear's
  canonical issue after confirmation.
- Failure restores the exact prior issue. A background refresh that arrives
  during editing becomes the fresh rollback baseline.
- Account switching and manual refresh cannot start during a pending mutation.
- Team workflow metadata is retained in the per-account SQLite cache through a
  versioned, non-destructive schema migration.

## Acceptance

- [x] The dashboard query returns all team workflow states with stable IDs.
- [x] Status-only `issueUpdate` requests have a GraphQL contract test.
- [x] Cross-team states never appear in the picker.
- [x] Optimistic success, failure rollback, and refresh rebasing are tested.
- [x] Confirmed status changes update the cache; failed changes do not.
- [x] Existing version-1 cache databases migrate without quarantine or data
  loss.
- [x] Search and filter modes retain ownership of the `s` key.

## Dependency

Ticket 007a provides the shared optimistic mutation transaction.
