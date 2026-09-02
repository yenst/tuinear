# 007a — Safe title editing

Status: DONE

## User goal

Rename the selected Linear issue without leaving the terminal.

## Behavior

- `e` opens a focused title editor for the selected issue.
- Arrow, Home/End, Backspace/Delete, and Unicode input edit the value; Ctrl+U
  clears it.
- Enter applies an optimistic title immediately and starts one background
  mutation. A second edit cannot start while that mutation is pending.
- Linear's canonical issue replaces the optimistic issue after confirmation.
- A failed mutation restores the exact prior issue and shows an actionable
  error without replacing the dashboard.
- Only confirmed mutations update the local cache.
- OAuth login requests `read,write`; an existing read-only profile is told to
  reconnect before an edit is attempted.

## Acceptance

- [x] The GraphQL mutation contract validates the issue ID, non-empty title,
  GraphQL errors, success flag, and canonical response.
- [x] Optimistic success and exact rollback are covered by UI tests.
- [x] Empty titles and double submission are blocked.
- [x] Cache tests prove success is persisted and failure leaves prior data.
- [x] Search and filter modes retain ownership of their input keys.
- [x] Demo mode exercises the same title-editing UI without network access.

## Deferred to the parent ticket

Status, assignee, priority, labels, project, and description editors, followed
by a discoverable command palette.
