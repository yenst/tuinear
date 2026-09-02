# 007g — Safe label editing

Status: DONE

## User goal

Apply or remove several issue labels from the keyboard without leaving
Tuinear.

## Behavior

- `l` opens a multi-select editor containing workspace labels and labels for
  the selected issue's team.
- `space` toggles the highlighted label, `enter` applies the complete
  selection, and `esc` cancels without changing the issue.
- The `enter` issue action menu exposes Edit labels with the current values.
- Confirming a changed set updates the issue immediately while Linear is
  contacted in the background; confirming the unchanged set sends no mutation.
- A successful mutation uses Linear's canonical issue and updates the cache;
  a failure restores the exact previous issue.
- Cached startup retains workspace and team label metadata.

## Acceptance

- [x] Workspace and team labels are loaded and merged without duplicates.
- [x] Label-only mutations omit unrelated fields, normalize duplicate IDs, and
  send an empty list to clear every label.
- [x] Label metadata and its workspace/team relationships round-trip through
  the normalized cache; schema versions 1–3 migrate safely.
- [x] Optimistic selection, clearing rollback, refresh rebasing, unchanged
  selection, team isolation, and missing metadata are tested.
- [x] Demo mode uses the same multi-select editor and mutation path.
