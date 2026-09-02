# 001 — Application foundation

Status: DONE

## User goal

Run one cross-platform binary and understand its keyboard controls immediately.

## Behavior

- Start in the terminal's alternate screen.
- Resize without corrupting the layout.
- Quit with `q` or `ctrl+c`.
- Offer a demo mode and a printable snapshot for development.

## Acceptance

- The project builds on Linux, macOS, and Windows.
- UI state is owned by a Bubble Tea model.
- CI runs tests, vet, and build.
- No production feature requires demo-only branches in the UI.

