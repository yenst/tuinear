# Tuinear ticket backlog

Tickets are ordered by dependency and MVP value. `DONE` means the behavior and
its automated tests are present. A ticket is only complete when its failure and
empty states are handled.

| Ticket | Status | Outcome |
| --- | --- | --- |
| [001](001-foundation.md) | DONE | Testable Go application foundation |
| [002](002-linear-read-api.md) | DONE | Load a read-only Linear dashboard |
| [003](003-issue-browser.md) | DONE | Browse tickets and inspect details |
| [004](004-search-and-filters.md) | READY | Quickly narrow a large workspace |
| [005](005-persistent-cache.md) | PROPOSED | Instant and offline startup |
| [006](006-oauth-login.md) | DONE | Browser-based sign-in and secure storage |
| [007](007-editing.md) | DEFERRED | Edit safe issue fields |
| [008](008-archive-and-delete.md) | DEFERRED | Confirmed destructive operations |

The MVP boundary ends after ticket 003. Tickets 007 and 008 must remain last.
