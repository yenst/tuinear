# 011 — Create issues

Status: DONE

## Outcome

Create a complete ticket from the keyboard in an explicitly selected team.
Collect all editable fields in one form, show the finished draft
optimistically, reconcile it with Linear's canonical response, and keep the
whole draft retryable if creation fails.

## Acceptance criteria

- `n` opens one creation form only when a specific team is selected and names
  that team in the form.
- The form supports title, Markdown description, team status, priority,
  assignee, team project, and workspace/team labels without leaving the flow.
- Keyboard navigation distinguishes editing a field from submitting the form;
  `Ctrl+S` or the final action submits one mutation.
- Submitting sends Linear one `issueCreate` mutation containing the trimmed
  title, team ID, and configured optional fields.
- A temporary ticket appears immediately and survives a dashboard refresh
  without duplicate placeholders.
- Success replaces the temporary ticket with Linear's canonical issue and the
  cache stores that canonical issue.
- Failure removes the temporary ticket, displays the error, and restores the
  complete draft for retry.
- Empty titles never invoke Linear.

## Tests

- Linear full-field request shape, canonical decoding, validation, and
  unconfirmed result.
- Cached-loader insertion on success and no cache mutation on failure.
- UI team requirement, optimistic insertion, refresh reconciliation, canonical
  replacement, full-field form submission, navigation/cancellation, empty-title
  validation, and retryable rollback.
