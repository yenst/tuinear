# 007h — Safe description editing

Status: DONE

## User goal

Edit a multiline Markdown issue description without leaving Tuinear.

## Behavior

- `d` opens a full-screen multiline editor for the selected issue.
- `enter` inserts a new line, arrow keys move the cursor, `ctrl+s` saves, and
  `esc` cancels without changing the issue.
- `ctrl+u` clears the draft so an existing description can be removed.
- The `enter` issue action menu exposes Edit description with a compact preview.
- Saving updates the issue immediately while Linear is contacted in the
  background; an unchanged draft sends no mutation.
- A successful mutation uses Linear's canonical issue and updates the cache;
  a failure restores the exact previous issue.

## Acceptance

- [x] Description-only mutations omit unrelated fields and accept an empty
  string to clear the description.
- [x] Multiline insertion, cursor movement, viewport scrolling, clearing,
  cancellation, and unchanged drafts are tested.
- [x] Optimistic display, exact rollback, and draft preservation across a
  background refresh are tested.
- [x] Confirmed descriptions persist through the existing normalized cache.
- [x] Render stored Markdown descriptions with terminal-aware formatting and
  wrapping while keeping the editor's source text exact.
- [x] Demo mode uses the same editor and mutation path.
