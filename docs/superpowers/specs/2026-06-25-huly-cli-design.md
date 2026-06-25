# huly-cli — Design Spec

**Date:** 2026-06-25
**Status:** Approved (design), pending spec review
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
    huly/                   # Go port of api-client REST path
      config.go             # GET /config.json -> ACCOUNTS_URL
      account.go            # JSON-RPC: login, selectWorkspace
      rest.go               # REST client: findAll, findOne, tx, getAccount, loadModel
      tx.go                 # TxCreateDoc / TxUpdateDoc / TxRemoveDoc builders
      tracker.go            # tracker class IDs + typed structs
      ids.go                # ref/uuid generation helpers
    creds/
      creds.go              # credentials file load/save (0600)
    cache/
      cache.go              # local read-through cache load/save
  version/version.go        # (inherited) build-stamped version
  config/huly.yaml.example  # (inherited example, extended)
```

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
     REST base (replace `ws`→`http`, `wss`→`https`).
3. REST client targets `{endpoint}/api/v1`, header
   `Authorization: Bearer <workspace token>`.

### 4.2 REST operations (all under `/api/v1`)
| Go method   | HTTP | Path                          | Notes |
|-------------|------|-------------------------------|-------|
| `FindAll`   | GET  | `/find-all/{ws}`              | query params `class`, `query` (JSON), `options` (JSON); returns `[]Doc` + `total` |
| `FindOne`   | GET  | `/find-all/{ws}`              | `FindAll` with `options.limit=1`, first result |
| `Tx`        | POST | `/tx/{ws}`                    | body = transaction JSON; returns `TxResult` |
| `GetAccount`| GET  | `/account/{ws}`               | current account uuid/role/socialIds |
| `LoadModel` | GET  | `/load-model/{ws}?full=`      | returns `[]Tx` (model); used for class/status/priority resolution |

Cross-cutting: snappy/gzip response decompression (inspect `content-encoding`);
HTTP 429 backoff honoring `Retry-After` / `Retry-After-ms` (max 3 retries);
all timestamps are Unix milliseconds.

### 4.3 Transactions (`tx.go`)
Builders producing the transaction envelope consumed by `POST /tx/{ws}`:

- **TxCreateDoc** — `_class:"core:class:TxCreateDoc"`, fields `objectId`,
  `objectClass`, `objectSpace`, `attributes`, plus `modifiedOn`, `modifiedBy`,
  `space:"core:space:Tx"`.
- **TxUpdateDoc** — `_class:"core:class:TxUpdateDoc"`, `objectId`, `objectClass`,
  `objectSpace`, `operations` (field updates).
- **TxRemoveDoc** — `_class:"core:class:TxRemoveDoc"`, `objectId`, `objectClass`,
  `objectSpace`.

`objectId` / `_id` are generated as Huly-style refs (id generation helper in
`ids.go`).

### 4.4 Tracker types & class IDs (`tracker.go`)
- `tracker:class:Project` — `identifier`, `name`, `sequence`,
  `defaultIssueStatus`, `defaultAssignee`. (Projects are spaces.)
- `tracker:class:Issue` — `title`, `description`, `status`, `priority`,
  `assignee`, `component`, `milestone`, `number`, `estimation`, `dueDate`,
  `space` (project).
- `tracker:class:Milestone` — `label`, `status`, `targetDate`, `space`.
- `tracker:class:Component` — `label`, `description`, `lead`, `space`.
- Priority enum: `NoPriority(0)`, `UrgentPriority(1)`, `HighPriority(2)`,
  `MediumPriority(3)`, `LowPriority(4)`.
- Standard statuses resolved via `LoadModel` / `FindAll` of
  `tracker:class:IssueStatus` per project (Backlog/Todo/InProgress/Done/Cancelled).

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
repo: `~/.config/huly/credentials.yaml`, mode `0600`. Schema:

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
error with guidance to run `huly login` or `huly auth set-token`.

## 6. Planning commands

Global persistent flag: `--output table|json` (default `table`).

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
huly component  add    ...                                                  # alias of create (emphasis: integrate)
```

- `create`/`update`/`add` issue a `tx` then **write-through** to the local cache.
- `<ID>` accepts a Huly issue number/identifier (e.g. `PROJ-42`) or internal id;
  resolved via cache then live `FindAll`.

## 7. Local cache & shell completion

A local read-through **mirror** of Huly (option (a) — never a source of truth):
`~/.cache/huly/cache.json`. Records are lightweight:

```json
{
  "projects":   [{"id":"...","identifier":"PROJ","name":"..."}],
  "components": [{"id":"...","project":"PROJ","label":"..."}],
  "milestones": [{"id":"...","project":"PROJ","label":"..."}],
  "issues":     [{"id":"...","project":"PROJ","identifier":"PROJ-42","title":"..."}],
  "syncedAt":   1750000000000
}
```

- **Sync:** `huly cache sync [--project KEY]` pulls the above from Huly and
  rewrites the file. `issue/component create|update` write-through to keep it
  fresh without a full re-sync.
- **Completion source:** Cobra dynamic completion functions
  (`ValidArgsFunction` + `RegisterFlagCompletionFunc`) read **only the cache** —
  no network on TAB, so completion is instant. Driven completions:
  - `huly issue update <TAB>` / `huly issue view <TAB>` → issue identifiers
  - `huly component get <TAB>`, `--component <TAB>` → component labels
  - `--project <TAB>` → project identifiers
  - `--milestone <TAB>` → milestone labels
  - `--status <TAB>` / `--priority <TAB>` → static enum values
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

Secrets (token/endpoint/workspace credentials) are **never** written here — they
live in `~/.config/huly/credentials.yaml` or env vars.

## 9. Error handling

- Network/auth failures return wrapped errors (`fmt.Errorf("...: %w", err)`) with
  actionable hints (e.g. "run `huly login`").
- Non-2xx REST responses surface status + body; 429 is retried with backoff.
- Name-resolution failures (`unknown project KEY`, `unknown status S`) list valid
  candidates from the cache/model.
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
  functions return expected candidates.
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
