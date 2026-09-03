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
| [004](004-search-and-filters.md) | DONE | Persist inclusive and negative filters per profile |
| [005](005-persistent-cache.md) | DONE | Instant and offline startup |
| [009](009-open-issue-browser.md) | DONE | Open the selected issue in the default browser |
| [007](007-editing.md) | DONE | Edit safe issue fields |
| [007a](007a-title-editing.md) | DONE | Optimistically edit and safely roll back issue titles |
| [007b](007b-status-editing.md) | DONE | Move issues through team-specific workflow states |
| [007c](007c-priority-editing.md) | DONE | Change issue priority with optimistic rollback |
| [007d](007d-issue-action-menu.md) | DONE | Discover issue actions from the Enter key |
| [007e](007e-assignee-editing.md) | DONE | Assign or unassign issues safely |
| [007f](007f-project-editing.md) | DONE | Move issues into or out of team projects safely |
| [007g](007g-label-editing.md) | DONE | Apply or clear issue labels safely |
| [007h](007h-description-editing.md) | DONE | Edit multiline issue descriptions safely |
| [008](008-archive-and-delete.md) | DONE | Confirmed, recoverable issue archiving |
| [010](010-interface-and-refresh-polish.md) | DONE | Compact help, reliable refresh/account switching, and terminal-native styling |
| [011](011-issue-creation.md) | DONE | Optimistically create tickets in an explicitly selected team |

The complete browsing and editing experience includes OAuth profiles, local
search, filters, offline caching, browser links, guarded mutations, and
confirmed recoverable archiving. Permanent deletion is deliberately not
exposed.
