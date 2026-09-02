# 008 — Archive and destructive operations

Status: DONE

## User goal

Archive an issue without risking an accidental destructive action.

## Delivered behavior

- Press `x`, or choose **Archive issue** from the action menu, to open a
  focused confirmation that names the exact issue.
- Start every confirmation on **Cancel** and keep permanent deletion out of
  Tuinear entirely.
- Explain before execution that archived issues remain recoverable from
  Linear's archive.
- Bind the confirmation to the original issue ID and identifier so navigation,
  refreshes, and repeated keys cannot retarget the action.
- Remove the issue from the dashboard and local cache only after Linear
  confirms the archive mutation.
- Keep the issue visible and show an actionable error when the mutation fails.

## Acceptance

- [x] Cancellation is the default path.
- [x] Repeated keys and stale selections cannot target a different issue.
- [x] Destructive behavior has end-to-end keyboard tests.
- [x] Archive failures leave the dashboard and cache unchanged.

## Dependency

This was intentionally implemented as the final feature ticket. Tuinear offers
the recoverable archive operation and does not expose permanent issue deletion.
