# 012 — Code quality and state consistency audit

Status: DONE

## User goal

Audit the project and improve readability, maintainability, and concrete
correctness problems without expanding the editing feature set.

## Scope and findings

Reviewed the CLI/account loader, Linear client, UI event handling and editors,
cache loader/storage, authentication, existing tests, and CI configuration.
The implementation changes focus on the following findings:

| Finding | Impact | Resolution |
| --- | --- | --- |
| Dashboard requests could overlap | Repeated refreshes and account switches could deliver results out of order | Allow one dashboard request at a time; account switching waits for an active refresh |
| Actions remained enabled during account selection | Editors could target the previous dashboard after the loader selected another account | Block dashboard actions until account loading finishes; retain quit shortcuts |
| A failed uncached account switch hid the previous dashboard | Users lost access to otherwise usable data behind a full-screen error | Preserve the existing dashboard and display the failure as a refresh error |
| Filter options could shrink during a background refresh | Enter or `!` could index past the options and panic | Clamp the palette selection before handling input |
| A confirmed mutation marked the whole snapshot fresh | Unchanged issues and metadata appeared newer than they were | Preserve cache timestamps and UI cache indicators after edits, creation, and archive |
| OAuth accepted callback port zero | The listener picked a port that differed from the advertised redirect | Require a numeric callback port between 1 and 65535 |
| UI and OAuth files exceeded the size guide | Unrelated responsibilities made changes harder to review | Split event dispatch, key handling, dashboard loading, browser/clipboard actions, authentication management, and OAuth transport into focused files |

## Acceptance

- UI state changes remain in `Update` and its synchronous helpers; background
  commands only return messages.
- Refresh/account requests cannot overlap through keyboard input, and a
  completed or failed request permits retry.
- Account loading cannot open editors or filters for the old account.
- Filter selection remains safe after refresh removes options.
- Successful issue mutations still update the cache, and cache failures cannot
  turn a successful remote mutation into a failed edit. Only a full dashboard
  refresh advances the snapshot's freshness.
- Existing public interfaces, keyboard actions, optimistic rollback behavior,
  and archive confirmation remain covered by the existing tests.
- Refactored production files stay below roughly 500 lines.

## Tests

- `internal/ui/refresh_safety_test.go`: overlapping requests, actions during
  account selection, failed-switch recovery, a shrinking palette, and quitting
  while loading.
- `internal/ui/mutation_freshness_test.go`: cached/offline indicators after all
  three mutation types.
- `internal/cache/mutation_freshness_test.go`: snapshot age after all three
  mutation types; existing loader tests cover canonical cache updates and
  remote failure behavior.
- `internal/auth/oauth_test.go`: regression for callback port zero, alongside
  existing callback, token refresh, and scope handling tests.

The new UI and cache regressions were reproduced against the previous
implementation before applying their fixes.

Validation on 2026-09-05:

- `go test -race -count=1 ./...` passed across all packages.
- `go vet ./...` passed.
- `go build -o /tmp/tuinear-audit ./cmd/tuinear` passed.
- The built binary's `--snapshot` output contained the expected demo content
  and 32 lines.
- `git diff --check` passed.

Go checks used `GOCACHE=/tmp/tuinear-go-build` because the default build-cache
directory is read-only in this workspace environment.

## Verification boundaries

API behavior is exercised using the project's test doubles. Live Linear
accounts and OS credential stores are not used by this audit. Cross-platform
verification remains in the existing Linux/macOS/Windows CI matrix. The
intentional MVP limit of 100 recent issues remains as specified in ticket 002.
