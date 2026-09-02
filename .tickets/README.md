# Tuinear ticket backlog

Tickets are ordered by dependency and MVP value. `DONE` means the behavior and
its automated tests are present. A ticket is only complete when its failure and
empty states are handled.

| Ticket | Status | Outcome |
| --- | --- | --- |
| [001](001-foundation.md) | DONE | Testable Go application foundation |
| [002](002-linear-read-api.md) | DONE | Load a read-only Linear dashboard |
| [003](003-issue-browser.md) | DONE | Browse tickets and inspect details |
| [006](006-oauth-login.md) | DONE | Browser-based sign-in and secure storage |
| [006a](006a-multi-account-profiles.md) | DONE | Keep work/personal workspace-user profiles connected |
| [004](004-search-and-filters.md) | DONE | Quickly narrow a large workspace |
| [005](005-persistent-cache.md) | DONE | Instant and offline startup |
| [009](009-open-issue-browser.md) | DONE | Open the selected issue in the default browser |
| [007](007-editing.md) | IN PROGRESS | Edit safe issue fields |
| [007a](007a-title-editing.md) | DONE | Optimistically edit and safely roll back issue titles |
| [007b](007b-status-editing.md) | DONE | Move issues through team-specific workflow states |
| [008](008-archive-and-delete.md) | DEFERRED | Confirmed destructive operations |

The complete browsing experience includes OAuth profiles, local search,
filters, offline caching, and browser links. Ticket 007 is now landing as
small, independently tested edit slices; destructive ticket 008 remains last.
