# huly-cli — Design Spec

**Date:** 2026-06-25
**Status:** Approved (design); revised after multi-agent review; pending final spec review
**HTTP client:** `go-resty/resty` v2 (see §4.2, §8.1)
**Repo:** `github.com/kettleofketchup/huly-cli`
**Binary:** `huly`

## 1. Summary

`huly` is a self-updating Cobra CLI for planning and integration against a
[Huly](https://huly.io) instance (self-hosted or Huly Cloud). It manages
tracker planning entities — **projects, issues, milestones, and components** —
by reimplementing the REST surface of Huly's `@hcengineering/api-client` in Go.

It supports two authentication modes (interactive session login and
non-interactive app/API token), and a local read-through cache that powers
instant shell tab-completion for issue IDs, component labels, project keys, and
milestone labels.

## 2. Scaffold & repository

The project is generated from the user's `go-template` (Copier) at
`github.com/kettleofketchup/go-template`, using these answers:

| copier var        | value                                  | effect |
|-------------------|----------------------------------------|--------|
| `project_name`    | `huly-cli`                             | repo dir / module base |
| `tool_name`       | `huly`                                 | binary `huly`, config `huly.yaml`, env prefix `HULY_` |
| `ci_platform`     | `github`                               | GitHub Actions CI + release workflow |
| `github_registry` | `ghcr.io/kettleofketchup/huly-cli`     | Go module `github.com/kettleofketchup/huly-cli/src/huly` |
| `self_update`     | `true`                                 | inherits the `huly update` command |

> The module path is derived by the template as
> `github.com/{{ github_registry | replace('ghcr.io/','') }}/src/{{ tool_name }}`,
> so `github_registry` **must** include the owner (`kettleofketchup/huly-cli`),
> otherwise the module path loses the owner segment.

A GitHub repo `kettleofketchup/huly-cli` is created and the scaffold pushed.

### Inherited from the template (no work required)
- `huly version` — version/build info.
- `huly config show | path | schema` — view/validate config; JSON-schema export.
- `huly update` — self-update. Detects the GitHub remote from `git remote`
  (falling back to the configured repo path), queries
  `api.github.com/repos/kettleofketchup/huly-cli/releases/latest`, downloads the
  asset named `huly_<goos>_<goarch>` (`.exe` on Windows), and atomically swaps
  the running binary. **The GitHub Actions release workflow must publish release
  assets named `huly_<goos>_<goarch>`** for self-update to find them.

## 3. Package layout

```
src/huly/
  main.go
  cmd/                      # Cobra commands (inherited + new)
    root.go                 # (inherited) config init, persistent flags
    version.go config.go    # (inherited)
    update.go               # (inherited, self_update=true)
    login.go                # huly login / logout / whoami
    auth.go                 # huly auth set-token
    project.go              # huly project list
    issue.go                # huly issue list/create/update/view
    milestone.go            # huly milestone list/create
    component.go            # huly component list/get/create
    cache.go                # huly cache sync
    completion_funcs.go     # cache-backed dynamic completion functions
  config/
    config.go               # (inherited) extended with Huly/server settings
  internal/
    huly/                   # Go port of api-client REST path (resty-based)
      config.go             # GET /config.json -> ACCOUNTS_URL
      account.go            # JSON-RPC: login, selectWorkspace
      rest.go               # REST client: findAll, findOne, tx, getAccount, loadModel
      tx.go                 # TxCreateDoc / TxUpdateDoc / TxRemoveDoc / collection-CUD builders
      tracker.go            # tracker class IDs + typed structs
      ids.go                # ref/uuid generation helpers
      errors.go             # sentinel errors (ErrUnauthorized, etc.)
    creds/
      creds.go              # credentials file load/save (0600), own viper.New() instance
    cache/
      cache.go              # local read-through cache: atomic write (tmp+rename) + flock
    output/
      output.go            # shared Table()/JSON() renderers; honors NO_COLOR / --quiet
  version/version.go        # (inherited) build-stamped version
  config/huly.yaml.example  # (inherited example, extended)
```

> **Module-path note:** the template fixes the module at
> `github.com/kettleofketchup/huly-cli/src/huly`, so internal imports carry the
> `/src/huly` segment. This is a template constraint, accepted as-is (not
> idiomatic standard-layout, but consistent with the user's other Go projects).

## 4. Huly client (`internal/huly`)

A pure Go port of the **REST adapter** of `@hcengineering/api-client`. Websocket
/ transactor protocol is intentionally NOT implemented — the `/api/v1` REST
surface covers all planning CRUD.

### 4.1 Connection bootstrap
1. `GET {baseURL}/config.json` → `ServerConfig{ ACCOUNTS_URL, ... }`.
2. Account service JSON-RPC (POST to `ACCOUNTS_URL`, body
   `{"method":"...","params":{...}}`):
   - `login{email,password}` → `LoginInfo{ account, token, ... }` (account token).
   - `selectWorkspace{workspaceUrl, kind:"external"}` →
     `WorkspaceLoginInfo{ endpoint, token, workspace, workspaceUrl, role }`.
     `token` here is the **workspace bearer token**; `endpoint` is the transactor
     REST base. Scheme rewrite must be **ordered** — `wss://`→`https://` first,
     then `ws://`→`http://` (a naive `replace("ws","http")` yields `httpss://`).
3. REST client targets `{endpoint}/api/v1`, header
   `Authorization: Bearer <workspace token>`.

### 4.2 REST operations (all under `/api/v1`)

The client is built on **`go-resty/resty` v2** (bearer via `SetAuthToken`, JSON
binding via `SetResult`, retry via `SetRetryCount`/`AddRetryCondition`). Because
**snappy is not a standard HTTP content-encoding**, calls that may return snappy
use `SetDoNotParseResponse(true)` + `resp.RawBody()` to decompress manually
before unmarshaling; gzip/plain responses use resty's normal JSON binding.

| Go method   | HTTP | Path                          | Notes |
|-------------|------|-------------------------------|-------|
| `FindAll`   | GET  | `/find-all/{ws}`              | query params `class`, `query` (JSON), `options` (JSON); returns `[]Doc` + `total` |
| `FindOne`   | GET  | `/find-all/{ws}`              | `FindAll` with `options.limit=1`, first result |
| `Tx`        | POST | `/tx/{ws}`                    | body = transaction JSON; returns `TxResult` |
| `GetAccount`| GET  | `/account/{ws}`               | current account uuid/role/socialIds |
| `LoadModel` | GET  | `/load-model/{ws}?full=`      | returns `[]Tx` (model); used for class/status/priority resolution |

Cross-cutting:
- snappy/gzip response decompression (inspect `content-encoding`).
- HTTP **429** backoff honoring `Retry-After` / `Retry-After-ms` (max 3 retries).
  Retry sleep is **context-aware**: `select { case <-time.After(d): case <-ctx.Done(): return ctx.Err() }`,
  so Ctrl-C interrupts a long backoff cleanly.
- HTTP **401** returns the sentinel `ErrUnauthorized` (`errors.go`); the command
  layer maps it to `"Session expired or token invalid. Run: huly login"`.
- all timestamps are Unix milliseconds.

### 4.3 Transactions (`tx.go`)
Builders producing the transaction envelope consumed by `POST /tx/{ws}`. The tx
envelope carries `space:"core:space:Tx"` (**confirmed** against core's
`TxFactory`), plus `modifiedOn`, `modifiedBy`, and `createdOn`/`createdBy` (the
factory defaults the latter pair to the modified values when omitted).

- **TxCreateDoc** — `_class:"core:class:TxCreateDoc"`; `objectId`, `objectClass`,
  `objectSpace`, `attributes`.
- **TxUpdateDoc** — `_class:"core:class:TxUpdateDoc"`; `objectId`, `objectClass`,
  `objectSpace`, `operations` (field updates).
- **TxRemoveDoc** — `_class:"core:class:TxRemoveDoc"`; `objectId`, `objectClass`,
  `objectSpace`.

> **Issues are `AttachedDoc`** (Issue → Task → `core:class:AttachedDoc`). Creating
> an issue is therefore **not** a bare `TxCreateDoc` — the create must also set
> the collection-membership fields:
> - `attachedTo` — parent issue ref, or **`tracker:ids:NoParent`** for a
>   top-level issue.
> - `attachedToClass` — `tracker:class:Issue`.
> - `collection` — `"subIssues"`.
>
> `tx.go` exposes a `NewCreateIssueTx(...)` helper that fills these correctly
> (defaulting `attachedTo` to `NoParent`) so callers don't have to. Milestones and
> Components are plain `Doc`s in the project space and use the bare
> `TxCreateDoc`. `objectId` / `_id` are generated as Huly-style refs (`ids.go`).

### 4.4 Tracker types & class IDs (`tracker.go`)
- `tracker:class:Project` — `identifier`, `name`, `sequence`,
  `defaultIssueStatus`, `defaultAssignee`. (Projects are spaces.)
- `tracker:class:Issue` — `title`, `description`, `status`, `priority`,
  `assignee`, `component`, `milestone`, `number`, `estimation`, `dueDate`,
  `attachedTo`, `attachedToClass`, `collection`, `space` (project).
- `tracker:class:Milestone` — `label`, `status`, `targetDate`, `space`.
- `tracker:class:Component` — `label`, `description`, `lead`, `space`.
- Priority enum (**corrected names**): `NoPriority(0)`, `Urgent(1)`, `High(2)`,
  `Medium(3)`, `Low(4)`.
- Standard statuses resolved via `LoadModel` / `FindAll` of
  `tracker:class:IssueStatus` per project (Backlog/Todo/InProgress/Done/Cancelled
  are defaults, but instances may customize — statuses are cached per project, §7).

### 4.5 Name → Ref resolution
Human-friendly inputs are resolved to Huly Refs before issuing a tx:
- project identifier (e.g. `PROJ`) → project space ref (FindAll Project by `identifier`).
- status label → IssueStatus ref (per project).
- priority name → priority enum int.
- assignee (name/email) → social id / person ref (FindAll over contacts).
- component/milestone label → ref within the project.

Resolution prefers the local cache (§7) and falls back to a live `FindAll`.

## 5. Authentication: session & app token

Credentials live in a **separate file**, never in `huly.yaml` and never in the
repo: `<UserConfigDir>/huly/credentials.yaml`, mode `0600`, where the directory
is resolved via Go's `os.UserConfigDir()` (honors `XDG_CONFIG_HOME` on Linux,
`~/Library/Application Support` on macOS, `%APPDATA%` on Windows). Loaded with a
**dedicated `viper.New()` instance** (the golang-viper "config + secrets"
pattern), kept entirely separate from the inherited main-config viper. Schema:

```yaml
endpoint: https://transactor.example/api  # transactor REST base
workspace: <workspace-uuid-or-url>
token: <bearer token>                      # workspace or app token
account: <account-uuid>                    # informational
```

### 5.1 Session setup (interactive)
- `huly login --url <huly-base> --email <e> --workspace <ws>` — prompts for the
  password (read without echo; never stored), runs `login` + `selectWorkspace`,
  and persists `{endpoint, workspace, token, account}` to the credentials file.
- `huly logout` — removes the credentials file.
- `huly whoami` — calls `GetAccount` and prints account uuid / role / workspace.

### 5.2 App / API token setup (non-interactive)
- `huly auth set-token --endpoint <e> --workspace <ws> --token <t>` — stores a
  pre-generated app token directly (token generated in the Huly UI / account
  service). This is the CI / automation path. It does **not** mint a token —
  minting via the account RPC may be added later once that RPC is confirmed.
- Environment overrides for ephemeral / CI use: `HULY_ENDPOINT`, `HULY_WORKSPACE`,
  `HULY_TOKEN`. When set, they take precedence over the credentials file.

Credential resolution order at command run: env vars → credentials file →
error with guidance to run `huly login` or `huly auth set-token`. A `401` at
runtime (expired/invalid token, for either auth mode) surfaces the same
re-login guidance via the `ErrUnauthorized` sentinel (§4.2).

> **Completion safety:** credential loading happens in a `PersistentPreRunE`,
> **not** in `cobra.OnInitialize`, so it is skipped during `__complete`
> invocations. Completion functions (§7) never need credentials or network.

## 6. Planning commands

Global persistent flags:
- `--output table|json` (default `table`) — **bound via `viper.BindPFlag("output", ...)`**
  so the `huly.yaml` `output:` value is honored when the flag is absent.
- `--quiet`/`-q` — suppress stdout (errors still go to stderr); for scripting.
- Color in table output is auto-stripped when `NO_COLOR` is set or stdout is not
  a TTY (centralized in `internal/output`).

```
huly project   list

huly issue      list   [--project KEY] [--status S] [--assignee A] [--component C] [--milestone M]
huly issue      view   <ID>
huly issue      create --project KEY --title T [--description D --priority P --assignee A --component C --milestone M]
huly issue      update <ID> [--title --description --status --priority --assignee --component --milestone]

huly milestone  list   --project KEY
huly milestone  create --project KEY --label L [--target-date YYYY-MM-DD --description D]

huly component  list   [--project KEY]
huly component  get    <label|id> [--project KEY]
huly component  create --project KEY --label L [--description D --lead A]   # creates in Huly + write-through cache
huly component  add    ...                                                  # Cobra Aliases entry on `create` (one impl)
```

- **`--project` resolution** (applies to `issue list`, and to any command where it
  is "optional" above): if omitted, fall back to `defaults.project` from
  `huly.yaml`; if neither is present, error with
  `"--project is required (or set defaults.project in huly.yaml)"`. Cross-project
  listing is explicitly deferred (§11).
- **`component add`** is the same command as `component create`, registered via
  Cobra `Aliases: []string{"add"}` — not a separate, drift-prone implementation.
- `create`/`update` issue a `tx` then **write-through** to the local cache
  (atomic + locked, §7).
- `<ID>` accepts a Huly issue number/identifier (e.g. `PROJ-42`) or internal id;
  resolved via cache then live `FindAll`.

## 7. Local cache & shell completion

A local read-through **mirror** of Huly (option (a) — never a source of truth):
`<UserCacheDir>/huly/cache.json` (via Go's `os.UserCacheDir()` — honors
`XDG_CACHE_HOME` / platform conventions). Records are lightweight:

```json
{
  "projects":   [{"id":"...","identifier":"PROJ","name":"..."}],
  "components": [{"id":"...","project":"PROJ","label":"..."}],
  "milestones": [{"id":"...","project":"PROJ","label":"..."}],
  "statuses":   [{"id":"...","project":"PROJ","name":"Backlog","category":"..."}],
  "issues":     [{"id":"...","project":"PROJ","identifier":"PROJ-42","title":"..."}],
  "syncedAt":   1750000000000
}
```

`statuses` is included because Huly statuses are **per-project and customizable**
(not a static enum) — `--status` completion reads them from the cache, filtered
by `--project` context when that flag is already present.

- **Sync:** `huly cache sync [--project KEY]` pulls the above from Huly and
  rewrites the file. `issue/component create|update` write-through to keep it
  fresh without a full re-sync.
- **Durability & concurrency:** every cache write is **atomic** (write to
  `cache.json.tmp` on the same filesystem, then `os.Rename`). Write-through and
  `sync` acquire an **exclusive advisory lock** (`flock` on
  `<UserCacheDir>/huly/cache.lock`) for the read-modify-write so concurrent
  `huly issue create` runs don't clobber each other. Readers (completion) do
  **not** lock — a slightly stale snapshot is fine for completion.
- **Completion source:** Cobra dynamic completion functions
  (`ValidArgsFunction` + `RegisterFlagCompletionFunc`) read **only the cache** —
  no network, no credentials, no config init on TAB, so completion is instant and
  side-effect-free. Driven completions:
  - `huly issue update <TAB>` / `huly issue view <TAB>` → issue identifiers
  - `huly component get <TAB>`, `--component <TAB>` → component labels
  - `--project <TAB>` → project identifiers
  - `--milestone <TAB>` → milestone labels
  - `--status <TAB>` → cached status names (project-scoped); `--priority <TAB>` →
    static priority names (`NoPriority/Urgent/High/Medium/Low`)
- **Staleness self-healing:** if a name→ref live resolution fails for a value that
  *was* present in the cache, the error includes
  `"Hint: the cache may be stale. Run: huly cache sync"`; a `404` on a direct-ID
  lookup removes that entry from the cache (self-healing write).
- **Install:** `huly completion bash|zsh|fish|powershell` (Cobra built-in). README
  documents the zsh install line.

If the cache file is missing, completion functions return no candidates (and
commands still work via live resolution); `huly cache sync` populates it.

## 8. Configuration (`huly.yaml`)

Extends the inherited `config.Config` with **non-secret** settings only:

```yaml
log:   { level: info, format: text }   # inherited
server:
  url: https://huly.example            # default base url for `huly login`
defaults:
  project: PROJ                        # default --project when omitted
output: table                          # default output format
```

New fields are added to the inherited `config.Config` struct (with `mapstructure`
tags) so viper unmarshals them; the `output` persistent flag is bound back with
`viper.BindPFlag` so flag-and-config interplay works.

Secrets (token/endpoint/workspace credentials) are **never** written here — they
live in `<UserConfigDir>/huly/credentials.yaml` or env vars.

### 8.1 Dependencies (added to inherited go.mod)
- `github.com/go-resty/resty/v2` — HTTP client (bearer, JSON binding, 429 retry,
  raw-body access for snappy).
- `github.com/golang/snappy` — snappy decompression of REST responses.
- `golang.org/x/term` — no-echo password prompt for `huly login`.
- `github.com/gofrs/flock` (or `x/sys` advisory locking) — cache write lock.
- Inherited: `spf13/cobra`, `spf13/viper`, `invopop/jsonschema`, `yaml.v3`.

## 9. Error handling

- Network/auth failures return wrapped errors (`fmt.Errorf("...: %w", err)`) with
  actionable hints (e.g. "run `huly login`").
- Non-2xx REST responses surface status + body; 429 is retried with context-aware
  backoff; 401 → `ErrUnauthorized` → re-login guidance (§4.2).
- Name-resolution failures (`unknown project KEY`, `unknown status S`) list valid
  candidates from the cache/model, plus the stale-cache hint (§7) when applicable.
- Missing credentials → clear message pointing to `huly login` /
  `huly auth set-token`.

## 10. Testing

- **Unit:** table-driven tests for tx builders (`tx.go`), id generation,
  priority/status parsing, and name→Ref resolution.
- **Client integration:** an `httptest.Server` replaying recorded `/config.json`,
  account JSON-RPC (`login`, `selectWorkspace`), and `/api/v1` (`find-all`, `tx`,
  `account`, `load-model`) responses — exercises the full client without a live
  Huly.
- **Cache/completion:** unit tests that seed a cache file and assert completion
  functions return expected candidates; atomic-write + lock behavior; a
  `__complete` invocation produces clean stdout (no config-init stderr noise).
- **Command smoke:** Cobra command wiring tests (flags parse, help renders).

## 11. Out of scope (YAGNI)

- Websocket / transactor live-query protocol.
- Offline/stageable local component lists (cache is read-only mirror — option a).
- Token minting via the account RPC (only accepting a pre-generated token).
- Non-tracker Huly modules (HR, documents, etc.).

## 12. Implementation phasing (for the plan)

1. Scaffold from `go-template`, create/push GitHub repo, verify inherited
   commands build (`version`, `config`, `update`).
2. `internal/huly` client (config bootstrap, account login, REST, tx, tracker
   types) + httptest coverage.
3. Auth: credentials file, `login`/`logout`/`whoami`, `auth set-token`, env
   overrides.
4. Planning commands: project/issue/milestone/component CRUD with name→Ref
   resolution.
5. Cache + dynamic completion + `cache sync` + `completion` install docs.
6. Docs (README/mkdocs) and release workflow asset-name verification.
