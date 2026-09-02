# 008 — Archive and destructive operations

Status: DEFERRED

## User goal

Archive or delete an issue without risking an accidental destructive action.

## Planned behavior

- Prefer archive over delete wherever Linear supports it.
- Require a focused confirmation that names the issue.
- Never bind permanent deletion to a single unmodified key.
- Explain whether the action is recoverable before execution.

## Acceptance

- Cancellation is the default path.
- Repeated keys and stale selections cannot target a different issue.
- Destructive behavior has end-to-end keyboard tests.

## Dependency

This is intentionally the final feature ticket.

