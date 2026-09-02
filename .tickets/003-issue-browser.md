# 003 — Issue browser MVP

Status: DONE

## User goal

Scan Linear tickets quickly and inspect the selected ticket.

## Behavior

- Show teams, issues, and selected-ticket details in three panes.
- Navigate issues with Vim keys or arrows.
- Cycle team filters with `tab`, `[` and `]`.
- Display identifier, status, priority, assignee, project, labels, and
  description where available.
- Adapt to narrow terminals instead of overflowing.

## Acceptance

- Selection never moves out of bounds.
- Changing teams resets selection and never shows a ticket from another team.
- The screen identifies the active team and total visible issue count.
- Demo data exercises completed, started, backlog, urgent, and unassigned states.
