# 002 — Read-only Linear API

Status: DONE

## User goal

Authenticate with a personal API key and load the tickets visible to me.

## Behavior

- Read `LINEAR_API_KEY`; never persist it.
- Fetch the viewer, teams, and 100 most recently updated issues.
- Apply a request timeout and expose useful API errors.
- Keep GraphQL transport details outside the UI package.

## Acceptance

- Request headers and GraphQL variables are covered by tests.
- GraphQL and non-2xx errors retain useful context without exposing the token.
- Loading, failure, and empty results are distinct UI states.

