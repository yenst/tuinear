# 005 — Persistent cache and synchronization

Status: PROPOSED

## User goal

Open Tuinear instantly and retain a useful read-only view while offline.

## Planned behavior

- Store normalized workspace data in SQLite.
- Render cached data immediately, then refresh in the background.
- Mark stale data visibly.
- Reconcile remote changes without moving the current selection unexpectedly.

## Acceptance

- Warm startup does not wait for the network.
- Corrupt or incompatible cache data can be rebuilt safely.
- Synchronization and migration behavior are integration tested.
