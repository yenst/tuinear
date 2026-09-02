# 006 — OAuth login

Status: DONE

## User goal

Sign in without manually creating or exporting an API key.

## Planned behavior

- Use Authorization Code with PKCE and state verification.
- Store tokens in the operating-system credential store.
- Refresh and revoke tokens safely.
- Retain `LINEAR_API_KEY` as a development override.

## Acceptance

- Tokens never appear in logs or configuration files.
- Callback timeout, state mismatch, refresh, and logout are tested.
- Headless environments receive a copyable authorization URL.

## Delivered

- `tuinear --login` opens a browser and completes a loopback PKCE flow.
- A missing session starts the same login flow automatically.
- Access tokens refresh one minute before expiry and rotated refresh tokens are
  saved atomically by the operating-system credential store.
- `tuinear --logout` revokes the refresh token before removing it locally.
- Only Linear's `read` and `write` OAuth scopes are requested; privileged scopes
  such as `admin` are never requested. The `write` scope is used only by explicit
  post-MVP edit commands.
