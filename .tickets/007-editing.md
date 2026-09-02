# 007 — Safe issue editing

Status: IN PROGRESS

## User goal

Update common issue fields after the read experience is proven reliable.

## Planned behavior

- [x] Edit title.
- [x] Edit status.
- [ ] Edit assignee, priority, labels, project, and description.
- Apply optimistic updates with visible pending state and rollback on failure.
- Prevent accidental double submission.

## Acceptance

- Every mutation has contract and rollback tests.
- Failed edits never leave the cache claiming success.
- Keyboard commands are discoverable through the command palette.

## Dependency

Tickets 001–006 must be complete first.

## Delivered foundation

Tickets 007a–007b added the authenticated `issueUpdate` contract, optimistic
title and team-specific status editors, exact rollback, double-submit
protection, and confirmed-only cache updates. The remaining field editors will
reuse this mutation pipeline.
