# Authentication

`huly` supports two ways to authenticate. Both end with credentials stored in
`<config-dir>/huly/credentials.yaml` (mode `0600`).

!!! info "Where credentials live"
    `os.UserConfigDir()/huly/credentials.yaml`:

    - Linux: `~/.config/huly/credentials.yaml`
    - macOS: `~/Library/Application Support/huly/credentials.yaml`
    - Windows: `%AppData%\huly\credentials.yaml`

=== "Interactive login"

    ```sh
    huly login --url https://huly.example.com --email you@example.com --workspace myws
    ```

=== "App token (non-interactive)"

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
