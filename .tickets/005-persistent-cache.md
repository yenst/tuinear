# 005 — Persistent cache and synchronization

Status: DONE

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

## Delivered

- Each OAuth profile and API-key identity has an isolated normalized SQLite
  snapshot in the operating system's user cache directory.
- Startup reads SQLite first and renders it before beginning the Linear request.
- The header distinguishes cached refresh, offline cached data, and live data.
- Successful refreshes replace a snapshot atomically; failed refreshes retain
  the last-known-good dashboard.
- Team and issue selections are reconciled by stable ID when refreshed data is
  reordered.
- Schema migration, account isolation, synchronization, corruption recovery,
  and newer-schema recovery have integration coverage.
