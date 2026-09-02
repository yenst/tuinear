# 007d — Issue action menu

Status: DONE

## User goal

Discover and launch every action available for the selected issue without
memorizing its keyboard shortcut.

## Behavior

- `enter` opens a focused menu for the selected issue.
- The menu lists only actions supported by the current data source: title,
  status, priority, and assignee editing, plus opening the issue in Linear when
  a URL is available.
- Each row shows the action's direct keyboard shortcut and current field value
  where useful.
- Arrow keys or `j`/`k` move through the menu; `enter` launches the selected
  action and `esc` cancels.
- `e`, `s`, `p`, `u`, and `space` remain direct shortcuts and also work while
  the action menu is open.

## Acceptance

- [x] The menu opens only from normal issue browsing mode.
- [x] Selecting an edit action opens the existing editor without duplicating
  mutation logic.
- [x] Read-only data sources expose only the browser action.
- [x] A background refresh closes the menu safely if its issue disappears.
- [x] Search and filter interactions keep ownership of their `enter` key.
