# 007 — Safe issue editing

Status: DEFERRED

## User goal

Update common issue fields after the read experience is proven reliable.

## Planned behavior

- Edit title, status, assignee, priority, labels, project, and description.
- Apply optimistic updates with visible pending state and rollback on failure.
- Prevent accidental double submission.

## Acceptance

- Every mutation has contract and rollback tests.
- Failed edits never leave the cache claiming success.
- Keyboard commands are discoverable through the command palette.

## Dependency

Tickets 001–006 must be complete first.

