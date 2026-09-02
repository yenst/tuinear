# 007f — Safe project editing

Status: DONE

## User goal

Move an issue into or out of one of its team's projects without leaving
Tuinear.

## Behavior

- `P` opens a picker containing No project and the selected issue team's
  available projects.
- The `enter` issue action menu exposes Change project with the current value.
- The current project is selected when the picker opens; choosing it again
  sends no mutation.
- Confirming another project updates the issue immediately while Linear is
  contacted in the background.
- A successful mutation uses Linear's canonical issue and updates the cache;
  a failure restores the exact previous issue.
- Cached startup retains each team's project list, so project editing remains
  available while the background refresh runs.

## Acceptance

- [x] Team projects are loaded through each Linear team's projects connection.
- [x] Project-only mutations omit unrelated fields and support explicit null
  for No project.
- [x] Team/project relationships round-trip through the normalized cache and
  schema versions 1 and 2 migrate safely.
- [x] Optimistic changes, clearing rollback, refresh rebasing, unchanged
  selection, team isolation, and missing metadata are tested.
- [x] Demo mode uses the same picker and mutation path.
