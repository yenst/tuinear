# Tuinear development guide

Keep the application read-only until every MVP ticket is complete. Features
are implemented as vertical slices: domain/API behavior, UI behavior, failure
states, and tests land together.

- UI state changes only inside Bubble Tea's `Update` path.
- Background commands return messages; they never mutate UI state directly.
- Keep Linear-specific GraphQL types inside `internal/linear`.
- Keep files focused. Split a file before it grows beyond roughly 500 lines.
- Every ticket must define acceptance criteria and tests.
- Editing, archiving, and deletion remain last in the roadmap.
