# Authentication

`huly` supports three ways to authenticate. All end with credentials stored in
`<config-dir>/huly/credentials.yaml` (mode `0600`).

!!! info "Where credentials live"
    `os.UserConfigDir()/huly/credentials.yaml`:

    - Linux: `~/.config/huly/credentials.yaml`
    - macOS: `~/Library/Application Support/huly/credentials.yaml`
    - Windows: `%AppData%\huly\credentials.yaml`

=== "Password login"

    ```sh
    huly login --url https://huly.example.com --email you@example.com --workspace myws
    ```

    Prompts for your password (never echoed or stored).

=== "One-time code (SSO / no password)"

    If your account uses **external login** (Google, GitHub, SSO) it has no
    password. Use an emailed one-time code instead — no password required:

    ```sh
    huly login --otp --url https://huly.example.com --email you@example.com --workspace myws
    ```

    `huly` requests a code, you paste the code from your email, and it exchanges
    it for a session token. This is the recommended path for SSO accounts.

=== "App token (non-interactive)"

    For CI/scripts, if you already have a token (e.g. copied from the browser's
    `Authorization: Bearer` header while logged into Huly web):

    ```sh
    huly auth set-token --endpoint https://transactor.example.com --workspace ws-uuid --token "$HULY_APP_TOKEN"
    ```

## Environment overrides

| Variable | Overrides |
|----------|-----------|
| `HULY_ENDPOINT`  | transactor endpoint |
| `HULY_WORKSPACE` | workspace id |
| `HULY_TOKEN`     | bearer token |

!!! danger "Token expiry"
    If a command fails with `huly: unauthorized`, re-run `huly login`.
