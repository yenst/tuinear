# 007 — Safe issue editing

Status: DONE

## User goal

Update common issue fields after the read experience is proven reliable.

## Planned behavior

- [x] Edit title.
- [x] Edit status.
- [x] Edit priority.
- [x] Edit assignee.
- [x] Edit project.
- [x] Edit labels.
- [x] Edit description.
- Apply optimistic updates with visible pending state and rollback on failure.
- Prevent accidental double submission.

## Acceptance

- Every mutation has contract and rollback tests.
- Failed edits never leave the cache claiming success.
- Keyboard commands are discoverable through the command palette.

## Dependency

Tickets 001–006 must be complete first.

## Delivered foundation

Tickets 007a–007h added the authenticated `issueUpdate` contract, optimistic
title, team-specific status, priority, assignee, project, and multi-select
label editors plus a multiline description editor, exact rollback,
double-submit protection, and confirmed-only cache updates. The choice picker
is shared across single-value fields, and the issue action menu makes every
available operation discoverable from `enter`.
