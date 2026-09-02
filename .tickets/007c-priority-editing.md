# 007c — Safe priority editing

Status: DONE

## User goal

Change an issue's priority without leaving the keyboard-first browser.

## Behavior

- `p` opens a focused picker with Linear's five priority values: no priority,
  urgent, high, medium, and low.
- The issue changes immediately after confirmation while the mutation runs in
  the background.
- A successful mutation replaces the optimistic issue with Linear's canonical
  response and writes only that confirmed response to the cache.
- A failed mutation restores the exact previous issue and keeps the error
  visible.
- Refreshes that finish during an edit preserve the selected choice and use
  fresh server data as the rollback baseline.

## Acceptance

- [x] Priority-only GraphQL mutations omit unrelated fields and reject values
  outside Linear's `0` through `4` range.
- [x] The picker starts on the current priority and an unchanged choice sends
  no mutation.
- [x] Optimistic success and exact failure rollback are tested.
- [x] Only confirmed priority changes are persisted to the local cache.
- [x] Demo mode uses the same priority-editing flow without network access.
