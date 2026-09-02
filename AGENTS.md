# Tuinear development guide

The read-only MVP is complete. Add editing as narrow vertical slices: domain/API
behavior, optimistic UI behavior, rollback/failure states, cache consistency,
and tests land together.

- UI state changes only inside Bubble Tea's `Update` path.
- Background commands return messages; they never mutate UI state directly.
- Keep Linear-specific GraphQL types inside `internal/linear`.
- Keep files focused. Split a file before it grows beyond roughly 500 lines.
- Every ticket must define acceptance criteria and tests.
- Archive and deletion remain last in the roadmap and always require confirmation.
