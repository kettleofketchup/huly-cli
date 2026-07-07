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
    huly login --otp
    ```

    `huly` requests a code, you paste the code from your email, and it exchanges
    it for a session token. This is the recommended path for SSO accounts.

    On a real terminal, `--otp` opens an interactive form (URL, email,
    workspace, then the code) prefilled from any `--url`/`--email`/`--workspace`
    flags you pass, falling back to `server.url`, `login.email`, and
    `login.workspace` in your config file. The form's "Save to config?" toggle
    (on by default) writes whatever you entered back to those same keys, so
    the next `huly login --otp` needs no flags at all.

    ```sh
    # first run — fill in the form once, let it save
    huly login --otp

    # later — URL/email/workspace are prefilled from config
    huly login --otp
    ```

    Add `--url`, `--email`, or `--workspace` to override the prefill for a
    single run without touching the saved config.

    For scripts and CI (or any non-TTY session), pass `--no-interactive` to
    skip the form and fall back to plain stdin prompts. In this mode
    `--url`/`--email`/`--workspace` (or their config equivalents) are
    required up front:

    ```sh
    huly login --otp --no-interactive --url https://huly.example.com --email you@example.com --workspace myws
    ```

=== "App token (non-interactive)"

    For CI/scripts, if you already have a token (e.g. copied from the browser's
    `Authorization: Bearer` header while logged into Huly web):

    ```sh
    huly auth set-token --endpoint https://transactor.example.com --workspace ws-uuid --token "$HULY_APP_TOKEN"
    ```

## Pre-filling login with `config set`

`huly config set <key> <value>` writes a single value into your config file
(same location as `--config`, or `huly config path`) without touching
anything else already there. It's useful for pre-seeding the fields that
`huly login --otp` prefills:

```sh
huly config set server.url https://huly.example.com
huly config set login.email you@example.com
huly config set login.workspace myws

huly login --otp   # form opens with all three fields already filled in
```

`config set` only accepts a known set of keys and rejects anything else with
an error listing the valid ones:

| Key | Description |
|-----|-------------|
| `server.url` | Default Huly base URL |
| `login.email` | Default login email (OTP autofill) |
| `login.workspace` | Default workspace (OTP autofill) |
| `defaults.project` | Default project for issue commands |
| `output` | Default output format (`table`, `json`) |
| `log.level` | Log verbosity |
| `log.format` | Log output format |

## Environment overrides

| Variable | Overrides |
|----------|-----------|
| `HULY_ENDPOINT`  | transactor endpoint |
| `HULY_WORKSPACE` | workspace id |
| `HULY_TOKEN`     | bearer token |

!!! danger "Token expiry"
    If a command fails with `huly: unauthorized`, re-run `huly login`.
