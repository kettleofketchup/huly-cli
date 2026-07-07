# Login OTP TUI + Config Autofill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `huly login --otp` a Charmbracelet form that autofills the Huly URL, email, and workspace from config, and add a `config set` command to persist those values.

**Architecture:** Add `login.email`/`login.workspace` to the viper config struct. Add a `config set` command that writes single keys via a *fresh* viper instance (so global defaults/env don't bloat the file). Add a `charmbracelet/huh` form behind small helper functions that only *collect* inputs; the existing `runLoginOTP` network/creds core is untouched and still takes a `codeFn`. Non-TTY sessions fall back to today's stdin path.

**Tech Stack:** Go 1.25, Cobra, Viper, `charmbracelet/huh`, `golang.org/x/term`.

## Global Constraints

- Module root for all Go commands: `src/huly` (run `go`/`just` from there).
- Go version floor: `go 1.25.0` (do not lower `go.mod`).
- No Claude watermark / Co-Authored-By lines in commits (per project convention).
- Use `git -C /home/kettle/git_repos/huly-cli ...` for git; do not `cd` for git.
- Env prefix is `HULY_`; dotted keys map with `.`→`_` (`login.email` → `HULY_LOGIN_EMAIL`).
- `runLoginOTP(ctx, baseURL, email, workspace string, codeFn func() (string, error)) error` is the network/creds core and MUST NOT change signature — the TUI supplies inputs and a `codeFn`.
- Config `set` writer MUST use a fresh `viper.New()` instance, never the global viper, to avoid serializing defaults/env into the file.

---

## File Structure

- `src/huly/config/config.go` — MODIFY: add `LoginConfig` + `Config.Login` field.
- `src/huly/config/config_test.go` — MODIFY: assert login fields unmarshal.
- `src/huly/cmd/config.go` — MODIFY: add `config set` command + `writeConfigValues`/`resolveConfigPath`/`validConfigKeys` helpers.
- `src/huly/cmd/config_set_test.go` — CREATE: tests for the writer + command.
- `src/huly/cmd/login_tui.go` — CREATE: `otpInputs` type + `collectOTPInputs` + `promptCodeTUI` (huh forms).
- `src/huly/cmd/login.go` — MODIFY: `--no-interactive` flag, prefill resolver, TTY branch, save-to-config.
- `src/huly/cmd/login_prefill_test.go` — CREATE: tests for prefill resolution + save.
- `src/huly/go.mod` / `go.sum` — MODIFY: add `charmbracelet/huh` (via `go get`).

---

### Task 1: Config fields for login autofill

**Files:**
- Modify: `src/huly/config/config.go`
- Test: `src/huly/config/config_test.go`

**Interfaces:**
- Produces: `config.LoginConfig{ Email string; Workspace string }`, accessible via `config.Config.Login`, viper keys `login.email` / `login.workspace`.

- [ ] **Step 1: Write the failing test**

Add to `src/huly/config/config_test.go`:

```go
func TestLoginConfigUnmarshal(t *testing.T) {
	viper.Reset()
	Defaults()
	viper.Set("login.email", "me@corp.com")
	viper.Set("login.workspace", "acme")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Login.Email != "me@corp.com" || cfg.Login.Workspace != "acme" {
		t.Fatalf("login cfg = %+v", cfg.Login)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./config/ -run TestLoginConfigUnmarshal -v`
Expected: FAIL — compile error (`cfg.Login` undefined).

- [ ] **Step 3: Add the struct + field**

In `src/huly/config/config.go`, add `Login LoginConfig` to `Config` (after `Server`), and add the type:

```go
// Config is the top-level configuration struct.
type Config struct {
	Log      LogConfig      `yaml:"log"      json:"log"      jsonschema:"title=Logging Configuration,description=Configure log output"`
	Server   ServerConfig   `yaml:"server"   json:"server"   mapstructure:"server"`
	Login    LoginConfig    `yaml:"login"    json:"login"    mapstructure:"login"`
	Defaults DefaultsConfig `yaml:"defaults" json:"defaults" mapstructure:"defaults"`
	Output   string         `yaml:"output"   json:"output"   mapstructure:"output" jsonschema:"enum=table,enum=json,default=table"`
}

// LoginConfig holds default values used to autofill the login form.
type LoginConfig struct {
	Email     string `yaml:"email"     json:"email"     mapstructure:"email"     jsonschema:"description=Default account email for login"`
	Workspace string `yaml:"workspace" json:"workspace" mapstructure:"workspace" jsonschema:"description=Default workspace url/name for login"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./config/ -run TestLoginConfigUnmarshal -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git -C /home/kettle/git_repos/huly-cli add src/huly/config/config.go src/huly/config/config_test.go
git -C /home/kettle/git_repos/huly-cli commit -m "feat(config): add login.email and login.workspace fields"
```

---

### Task 2: `config set` command + single-key writer

**Files:**
- Modify: `src/huly/cmd/config.go`
- Test: `src/huly/cmd/config_set_test.go` (create)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `func writeConfigValues(path string, kv map[string]string) error` — writes each key into the YAML file at `path` using a fresh viper instance; creates the parent dir; leaves untouched keys as-is.
  - `func resolveConfigPath() string` — `--config` value if set, else `viper.ConfigFileUsed()`, else `./config/huly.yaml`.
  - `var validConfigKeys map[string]bool` — allowlist of settable keys.
  - `configSetCmd` wired under `configCmd`.

- [ ] **Step 1: Write the failing test**

Create `src/huly/cmd/config_set_test.go`:

```go
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigValuesOnlyWritesGivenKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "huly.yaml")

	if err := writeConfigValues(path, map[string]string{"login.email": "me@corp.com"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, "me@corp.com") {
		t.Fatalf("missing value; file=%q", got)
	}
	// Must NOT leak global defaults into the file.
	if strings.Contains(got, "log:") || strings.Contains(got, "output:") {
		t.Fatalf("file bloated with defaults; file=%q", got)
	}
}

func TestWriteConfigValuesPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huly.yaml")
	if err := os.WriteFile(path, []byte("server:\n  url: https://existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigValues(path, map[string]string{"login.workspace": "acme"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	if !strings.Contains(got, "https://existing") || !strings.Contains(got, "acme") {
		t.Fatalf("expected both values; file=%q", got)
	}
}

func TestConfigSetRejectsUnknownKey(t *testing.T) {
	if validConfigKeys["not.a.key"] {
		t.Fatal("unknown key unexpectedly allowed")
	}
	if !validConfigKeys["login.email"] {
		t.Fatal("login.email should be allowed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./cmd/ -run 'TestWriteConfigValues|TestConfigSet' -v`
Expected: FAIL — `writeConfigValues` / `validConfigKeys` undefined.

- [ ] **Step 3: Implement writer, resolver, allowlist, and command**

In `src/huly/cmd/config.go`, add imports `path/filepath` and `github.com/spf13/viper` (already imported) and implement:

```go
// validConfigKeys is the allowlist of keys `config set` may write, so typos
// fail loudly instead of writing a dead key.
var validConfigKeys = map[string]bool{
	"server.url":        true,
	"login.email":       true,
	"login.workspace":   true,
	"defaults.project":  true,
	"output":            true,
	"log.level":         true,
	"log.format":        true,
}

// resolveConfigPath picks the file config writes should target.
func resolveConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	if used := viper.ConfigFileUsed(); used != "" {
		return used
	}
	return filepath.Join("config", "huly.yaml")
}

// writeConfigValues merges kv into the YAML file at path using a fresh viper
// instance (NOT the global one) so defaults/env are never serialized to disk.
func writeConfigValues(path string, kv map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	_ = v.ReadInConfig() // tolerate missing file
	for k, val := range kv {
		v.Set(k, val)
	}
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a single config value (e.g. login.email me@corp.com)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		if !validConfigKeys[key] {
			keys := make([]string, 0, len(validConfigKeys))
			for k := range validConfigKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("unknown config key %q; valid keys: %s", key, strings.Join(keys, ", "))
		}
		path := resolveConfigPath()
		if err := writeConfigValues(path, map[string]string{key: value}); err != nil {
			return err
		}
		fmt.Printf("set %s in %s\n", key, path)
		return nil
	},
}
```

Add imports `sort` and `strings` to the file, and register the command in `init()`:

```go
	configCmd.AddCommand(configSetCmd)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./cmd/ -run 'TestWriteConfigValues|TestConfigSet' -v`
Expected: PASS.

- [ ] **Step 5: Sanity-check the command end-to-end**

Run: `cd src/huly && go run . config set login.email demo@x.com --config /tmp/huly-plan.yaml && cat /tmp/huly-plan.yaml && rm -f /tmp/huly-plan.yaml`
Expected: prints `set login.email in /tmp/huly-plan.yaml`; file contains only `login:` / `email: demo@x.com` (no `log:`/`output:`).

- [ ] **Step 6: Commit**

```bash
git -C /home/kettle/git_repos/huly-cli add src/huly/cmd/config.go src/huly/cmd/config_set_test.go
git -C /home/kettle/git_repos/huly-cli commit -m "feat(config): add 'config set' command with clean single-key writer"
```

---

### Task 3: huh dependency + TUI form helpers

**Files:**
- Modify: `src/huly/go.mod`, `src/huly/go.sum`
- Create: `src/huly/cmd/login_tui.go`

**Interfaces:**
- Produces:
  - `type otpInputs struct { URL, Email, Workspace string; Save bool }`
  - `func collectOTPInputs(prefill otpInputs) (otpInputs, error)` — runs form 1 (URL/Email/Workspace + Save toggle), returns filled inputs; returns a "login cancelled" error if the user aborts.
  - `func promptCodeTUI() (string, error)` — runs form 2, returns the emailed code; matches the `codeFn func() (string, error)` shape `runLoginOTP` expects.

- [ ] **Step 1: Add the dependency**

Run: `cd src/huly && go get github.com/charmbracelet/huh@latest && go mod tidy`
Expected: `go.mod` now requires `github.com/charmbracelet/huh`.

- [ ] **Step 2: Create the TUI helpers**

Create `src/huly/cmd/login_tui.go`:

```go
package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// otpInputs are the values the login form collects.
type otpInputs struct {
	URL       string
	Email     string
	Workspace string
	Save      bool
}

var errLoginCancelled = errors.New("login cancelled")

func required(field string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

// collectOTPInputs shows the URL/Email/Workspace form (prefilled) plus a
// "save to config" toggle, and returns the completed inputs.
func collectOTPInputs(prefill otpInputs) (otpInputs, error) {
	in := prefill
	in.Save = true // default the toggle on
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Huly URL").Value(&in.URL).Validate(required("URL")),
			huh.NewInput().Title("Email").Value(&in.Email).Validate(required("email")),
			huh.NewInput().Title("Workspace").Value(&in.Workspace).Validate(required("workspace")),
			huh.NewConfirm().Title("Save URL/email/workspace to config?").Value(&in.Save),
		),
	).WithTitle("Huly OTP Login")
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return otpInputs{}, errLoginCancelled
		}
		return otpInputs{}, err
	}
	return in, nil
}

// promptCodeTUI shows a styled input for the emailed one-time code.
func promptCodeTUI() (string, error) {
	var code string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Enter the code sent to your email").Value(&code).Validate(required("code")),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", errLoginCancelled
		}
		return "", err
	}
	return code, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd src/huly && go build ./...`
Expected: builds with no errors.

- [ ] **Step 4: Commit**

```bash
git -C /home/kettle/git_repos/huly-cli add src/huly/go.mod src/huly/go.sum src/huly/cmd/login_tui.go
git -C /home/kettle/git_repos/huly-cli commit -m "feat(login): add huh TUI form helpers for OTP login"
```

---

### Task 4: Wire the TUI into `login --otp`

**Files:**
- Modify: `src/huly/cmd/login.go`
- Test: `src/huly/cmd/login_prefill_test.go` (create)

**Interfaces:**
- Consumes: `otpInputs`, `collectOTPInputs`, `promptCodeTUI` (Task 3); `writeConfigValues`, `resolveConfigPath` (Task 2); existing `runLoginOTP`, `promptCode`.
- Produces:
  - `func otpPrefill(flagURL, flagEmail, flagWS string) otpInputs` — for each field returns the flag value, else the matching viper key (`server.url` / `login.email` / `login.workspace`).
  - `func saveOTPInputs(in otpInputs) error` — writes `server.url`/`login.email`/`login.workspace` to the resolved config path via `writeConfigValues`.
  - `--no-interactive` flag on `loginCmd` (var `loginNoInteractive`).

- [ ] **Step 1: Write the failing test**

Create `src/huly/cmd/login_prefill_test.go`:

```go
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestOTPPrefillPrefersFlagThenConfig(t *testing.T) {
	viper.Reset()
	viper.Set("server.url", "https://cfg")
	viper.Set("login.email", "cfg@x.com")
	viper.Set("login.workspace", "cfgws")

	// Flags win when present.
	got := otpPrefill("https://flag", "", "")
	if got.URL != "https://flag" {
		t.Fatalf("url = %q, want flag", got.URL)
	}
	// Config fills the blanks.
	if got.Email != "cfg@x.com" || got.Workspace != "cfgws" {
		t.Fatalf("prefill = %+v", got)
	}
}

func TestSaveOTPInputsWritesThreeKeys(t *testing.T) {
	viper.Reset()
	path := filepath.Join(t.TempDir(), "huly.yaml")
	t.Setenv("HULY_CONFIG_TEST_PATH", "") // keep env clean
	viper.SetConfigFile(path)

	err := saveOTPInputs(otpInputs{URL: "https://h", Email: "e@x.com", Workspace: "ws", Save: true})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	for _, want := range []string{"https://h", "e@x.com", "ws"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q; file=%q", want, got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./cmd/ -run 'TestOTPPrefill|TestSaveOTPInputs' -v`
Expected: FAIL — `otpPrefill` / `saveOTPInputs` undefined.

- [ ] **Step 3: Implement prefill + save + flag, and branch the RunE**

In `src/huly/cmd/login.go`:

Add the flag var near the others:

```go
var loginNoInteractive bool
```

Add helpers:

```go
// otpPrefill resolves each field to its flag value, else the config value.
func otpPrefill(flagURL, flagEmail, flagWS string) otpInputs {
	pick := func(flag, key string) string {
		if flag != "" {
			return flag
		}
		return viper.GetString(key)
	}
	return otpInputs{
		URL:       pick(flagURL, "server.url"),
		Email:     pick(flagEmail, "login.email"),
		Workspace: pick(flagWS, "login.workspace"),
	}
}

// saveOTPInputs persists URL/email/workspace to the config file for autofill.
func saveOTPInputs(in otpInputs) error {
	return writeConfigValues(resolveConfigPath(), map[string]string{
		"server.url":      in.URL,
		"login.email":     in.Email,
		"login.workspace": in.Workspace,
	})
}

// otpInteractive reports whether we should show the TUI: a real terminal on
// both stdin and stderr, and --no-interactive was not passed.
func otpInteractive() bool {
	if loginNoInteractive {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}
```

Replace the `if loginOTP { ... }` block in the RunE (currently lines ~29-38) so the OTP branch runs first and uses the TUI when interactive:

```go
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginOTP {
			return runLoginOTPInteractive(cmd.Context())
		}
		if loginURL == "" {
			loginURL = viper.GetString("server.url")
		}
		if loginURL == "" || loginEmail == "" || loginWorkspace == "" {
			return fmt.Errorf("--url, --email and --workspace are required")
		}
		fmt.Fprint(os.Stderr, "Password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		return runLogin(cmd.Context(), loginURL, loginEmail, string(pw), loginWorkspace)
	},
```

Add the orchestrator:

```go
// runLoginOTPInteractive resolves inputs (TUI when on a terminal, plain
// stdin otherwise) and runs the OTP login.
func runLoginOTPInteractive(ctx context.Context) error {
	prefill := otpPrefill(loginURL, loginEmail, loginWorkspace)

	if !otpInteractive() {
		// Non-interactive: require resolved fields, use the stdin code prompt.
		if prefill.URL == "" || prefill.Email == "" || prefill.Workspace == "" {
			return fmt.Errorf("--url, --email and --workspace are required")
		}
		return runLoginOTP(ctx, prefill.URL, prefill.Email, prefill.Workspace, promptCode)
	}

	in, err := collectOTPInputs(prefill)
	if err != nil {
		return err
	}
	if in.Save {
		if err := saveOTPInputs(in); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
		}
	}
	return runLoginOTP(ctx, in.URL, in.Email, in.Workspace, promptCodeTUI)
}
```

Register the flag in `init()`:

```go
	loginCmd.Flags().BoolVar(&loginNoInteractive, "no-interactive", false, "skip the form; use plain stdin prompts (for scripts/CI)")
```

- [ ] **Step 4: Run the new tests**

Run: `cd src/huly && go test ./cmd/ -run 'TestOTPPrefill|TestSaveOTPInputs' -v`
Expected: PASS.

- [ ] **Step 5: Run the full suite (regression check)**

Run: `cd src/huly && go test ./...`
Expected: PASS — existing `login_test.go` OTP/password tests still green (they call `runLoginOTP`/`runLogin` directly and are unaffected).

- [ ] **Step 6: Manual smoke of the non-interactive fallback**

Run: `cd src/huly && printf '' | go run . login --otp --no-interactive 2>&1 | head -3`
Expected: fails fast with `--url, --email and --workspace are required` (no config set), confirming the non-TTY path does not open a form.

- [ ] **Step 7: Commit**

```bash
git -C /home/kettle/git_repos/huly-cli add src/huly/cmd/login.go src/huly/cmd/login_prefill_test.go
git -C /home/kettle/git_repos/huly-cli commit -m "feat(login): interactive OTP form with config autofill and --no-interactive fallback"
```

---

### Task 5: Lint + docs pass

**Files:**
- Modify: any file flagged by lint; `docs/` login/config reference if present.

- [ ] **Step 1: Format + lint**

Run: `cd /home/kettle/git_repos/huly-cli && just lint`
Expected: no errors. Fix any formatting/lint findings and re-run.

- [ ] **Step 2: Update docs if a login/config page exists**

Run: `ls docs/ 2>/dev/null; grep -rl 'login --otp\|config set\|server.url' docs/ 2>/dev/null`
If a relevant page exists, add: the `--no-interactive` flag, `config set login.email/login.workspace`, and the autofill behavior. If no such page exists, skip.

- [ ] **Step 3: Commit (if anything changed)**

```bash
git -C /home/kettle/git_repos/huly-cli add -A
git -C /home/kettle/git_repos/huly-cli commit -m "chore(login): lint pass and docs for OTP form + config set"
```

---

## Self-Review

**Spec coverage:**
- Config fields `login.email`/`login.workspace` → Task 1. ✓
- `config set` with clean single-key writer (fresh viper) → Task 2. ✓
- huh TUI form + code prompt → Task 3. ✓
- Prefill (flags→config), Save toggle, `--no-interactive`, non-TTY fallback → Task 4. ✓
- Error handling: cancel → `errLoginCancelled` (Task 3); unknown key (Task 2); save failure warns and continues (Task 4). ✓
- Testing plan (config set no-bloat, prefill precedence, existing tests green) → Tasks 2/4. ✓
- Dependency added → Task 3. ✓

**Placeholder scan:** No TBD/TODO; all code shown in full. ✓

**Type consistency:** `otpInputs{URL,Email,Workspace,Save}` defined in Task 3, consumed identically in Task 4. `writeConfigValues(path, kv)` / `resolveConfigPath()` defined in Task 2, called in Task 4's `saveOTPInputs`. `runLoginOTP(ctx, url, email, workspace, codeFn)` signature unchanged; `promptCodeTUI`/`promptCode` both fit `func() (string, error)`. ✓
