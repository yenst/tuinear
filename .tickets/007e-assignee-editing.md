# 007e — Safe assignee editing

Status: DONE

## User goal

Assign or unassign an issue from the keyboard without leaving Tuinear.

## Behavior

- `u` opens a picker containing Unassigned and the workspace's available
  members.
- The `enter` issue action menu exposes Change assignee with the current value.
- The current assignee is selected when the picker opens; choosing it again
  sends no mutation.
- Confirming a different member updates the issue immediately while Linear is
  contacted in the background.
- A successful mutation uses Linear's canonical issue and updates the cache;
  a failure restores the exact previous issue.
- Cached startup retains the workspace member list, so assignee editing remains
  available while the background refresh runs.

## Acceptance

- [x] Workspace members are loaded through Linear's users connection.
- [x] Assignee-only mutations omit unrelated fields and support explicit null
  for Unassigned.
- [x] Workspace members round-trip through the existing normalized cache.
- [x] Optimistic assignment, unassignment rollback, refresh rebasing, and
  unchanged selection are tested.
- [x] Demo mode uses the same picker and mutation path.
