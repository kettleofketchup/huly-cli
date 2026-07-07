# Design: `huly login --otp` Charm TUI + config autofill

**Date:** 2026-07-07
**Status:** Approved, ready for implementation plan

## Goal

Make `huly login --otp` interactive and friendly: open a Charmbracelet
form that lets the user fill in / confirm the Huly URL, email, and
workspace, then enter the emailed one-time code. Prefill those fields
from config so repeat logins are near-zero-typing, and let the user
persist them back to config.

## Scope

In scope:

1. New config fields `login.email` and `login.workspace` (alongside the
   existing `server.url`).
2. A `huly config set <key> <value>` command that writes a single key
   back to the YAML config file without bloating it.
3. A `charmbracelet/huh` TUI for the `login --otp` flow, with autofill
   from flags → config and an optional "save to config" toggle.
4. Non-interactive fallback to the existing stdin prompt path.

Out of scope: changing the password login flow's UX, changing the
network/creds layer, remote config stores.

## Current State

- `cmd/login.go`: `login --otp` calls `runLoginOTP(ctx, url, email,
  workspace, codeFn)`. `codeFn` is `promptCode()`, which reads the code
  from stdin. `--url` already falls back to `viper.GetString("server.url")`.
  `runLoginOTP` is the network/creds core: request OTP → validate → select
  workspace → `creds.Save`. It takes `codeFn func() (string, error)`, which
  is already a clean seam for injecting input (tests use a stub).
- `config/config.go`: `Config` has `Server.URL` (`server.url`),
  `Defaults.Project`, `Output`, `Log`. No email/workspace fields.
- `cmd/config.go`: `config` has read-only `show`/`path`/`schema`. No writer.
- Global viper is set up with `config.Defaults()` and `AutomaticEnv()`
  (`HULY_` prefix) in `cmd/root.go` `initConfig`.
- No charmbracelet dependency yet.
- `internal/huly/account.go`: `LoginOtp(ctx, email)`, `ValidateOtp(ctx,
  email, code)`, `SelectWorkspace(ctx, token, workspaceURL)` — unchanged.

## Design

### 1. Config fields

Add a `Login` section to `config.Config`:

```go
type Config struct {
    Log      LogConfig      // unchanged
    Server   ServerConfig   // unchanged (server.url)
    Login    LoginConfig    // NEW
    Defaults DefaultsConfig // unchanged
    Output   string         // unchanged
}

type LoginConfig struct {
    Email     string `yaml:"email"     json:"email"     mapstructure:"email"     jsonschema:"description=Default account email for login"`
    Workspace string `yaml:"workspace" json:"workspace" mapstructure:"workspace" jsonschema:"description=Default workspace url/name for login"`
}
```

Resulting config shape:

```yaml
server:
  url: https://huly.example.com
login:
  email: me@corp.com
  workspace: acme
```

No new defaults are registered (empty is the sensible default).

### 2. `huly config set <key> <value>`

New subcommand under `config`. It must **not** use the global viper —
that instance has defaults registered and `AutomaticEnv` on, so
`viper.WriteConfig()` would serialize defaults (`log.level: info`,
`output: table`) and env-derived values into the file, bloating it and
leaking env values to disk. Instead use a fresh instance that reads only
the existing file:

```go
func runConfigSet(key, value string) error {
    path := resolveConfigPath()          // --config, else viper.ConfigFileUsed(), else ./config/huly.yaml
    v := viper.New()
    v.SetConfigFile(path)
    _ = v.ReadInConfig()                 // tolerate missing file
    v.Set(key, value)
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
        return err
    }
    return v.WriteConfigAs(path)         // writes existing file content + the one key
}
```

- Key is validated against the known config keys (`server.url`,
  `login.email`, `login.workspace`, `defaults.project`, `output`,
  `log.level`, `log.format`) so typos fail loudly rather than writing a
  dead key.
- `resolveConfigPath()` prefers an already-loaded file; if none, defaults
  to `./config/huly.yaml` and creates the dir.

`config show`/`path`/`schema` are unchanged.

### 3. TUI for `login --otp`

New file `cmd/login_tui.go` (keeps `login.go` thin). Uses
`charmbracelet/huh`.

**Input struct** (what the form collects):

```go
type otpInputs struct {
    URL, Email, Workspace string
    Save                  bool
}
```

**Flow in `login --otp` RunE:**

1. Resolve prefill values: flag value, else config
   (`server.url` / `login.email` / `login.workspace`).
2. Decide interactive vs. not:
   - Interactive iff stdin **and** stderr are TTYs (`term.IsTerminal`)
     and `--no-tui` was not passed.
   - Non-interactive → keep the **current** behavior exactly: require
     url/email/workspace (from flags/config), call `runLoginOTP` with the
     existing `promptCode` stdin `codeFn`. This preserves scripting and
     the existing tests.
3. Interactive:
   - Run **form 1** (huh): text inputs URL, Email, Workspace (prefilled),
     each with a non-empty validator, plus a confirm/toggle "Save these to
     config" (default on).
   - If Save: call the same writer as `config set` for any of the three
     values that differ from what's already on disk (`server.url`,
     `login.email`, `login.workspace`).
   - Call `runLoginOTP(ctx, url, email, workspace, tuiCodeFn)` where
     `tuiCodeFn` runs **form 2**: a single styled huh input that prompts
     for the emailed code (`runLoginOTP` requests the OTP *before* calling
     `codeFn`, so the "code sent to <email>" message still precedes entry).

**Structure / boundaries:**

- `runLoginOTP` is unchanged — it stays the network/creds core and keeps
  taking a `codeFn`. The TUI only *collects* inputs and supplies a
  `codeFn`. This preserves the existing test seam.
- The huh forms live behind small functions
  (`collectOTPInputs(prefill) (otpInputs, error)` and `promptCodeTUI()
  (string, error)`) so `login.go` stays declarative and the form logic is
  isolated.
- `--no-tui` bool flag added to `loginCmd` for forcing the plain path
  even on a TTY.

### 4. Dependency

Add `github.com/charmbracelet/huh` (pulls in bubbletea/lipgloss). Run
`just go::tidy huly` (or `go mod tidy`) to record it.

## Data Flow

```
huly login --otp
      │  resolve prefill: flags → config (server.url/login.email/login.workspace)
      ▼
 TTY & !--no-tui ? ──no──▶ require fields ─▶ runLoginOTP(..., promptCode[stdin])
      │yes
      ▼
 huh form 1 (URL, Email, Workspace, [x] Save)
      │  if Save → config-set writer for changed keys
      ▼
 runLoginOTP(ctx, url, email, workspace, promptCodeTUI)
      │  LoginOtp(email)  → "code sent to <email>"
      │  promptCodeTUI()  → huh input (form 2)
      │  ValidateOtp      → SelectWorkspace → creds.Save
      ▼
   logged in
```

## Error Handling

- Form cancelled (Ctrl-C / Esc): return a clean "login cancelled" error,
  no creds written.
- Empty required field: caught by huh field validators before submit.
- `config set` on unknown key: error listing valid keys.
- `config set` write failure (permissions, etc.): wrapped `%w` error.
- Network/OTP errors: unchanged — surfaced by `runLoginOTP` as today.
- Save-to-config failure during login: warn on stderr but continue the
  login (persisting creds matters more than caching autofill).

## Testing

- **config set:** write to a temp file, assert only intended keys land in
  the YAML (no `log.level`/`output` bloat); unknown key errors; creates
  missing dir/file.
- **config struct:** `login.email`/`login.workspace` unmarshal from YAML
  and from `HULY_LOGIN_EMAIL` / `HULY_LOGIN_WORKSPACE` env.
- **login non-interactive path:** existing `login_test.go` tests keep
  passing unchanged (they call `runLoginOTP`/`runLogin` directly).
- **prefill resolution:** unit-test the flag→config precedence helper.
- huh form rendering itself is not unit-tested (interactive TTY); the
  logic seams around it (`otpInputs` resolution, save-diff) are.

## YAGNI / cut lines

If scope needs trimming: drop the `config set` command (rely on manual
YAML editing + the form's Save toggle), or drop the Save toggle (rely on
`config set`). The two together are the flexible default; either alone
still delivers autofill.
