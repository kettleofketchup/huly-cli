# huly-cli

`huly` is a self-updating command-line interface for [Huly](https://huly.io) tracker. It lets you manage projects, issues, milestones, and components from a terminal, with cache-backed shell completion so TAB-expanding project keys and issue identifiers works offline.

## Install

### go install (recommended)

```sh
go install github.com/kettleofketchup/huly-cli/src/huly@latest
```

### From source

```sh
git clone https://github.com/kettleofketchup/huly-cli.git
cd huly-cli
just build          # binary written to bin/huly
```

### Self-update

```sh
huly update
```

`huly update` detects the git remote of the installed binary (GitHub or GitLab) and replaces the running executable with the latest release for your OS and architecture.

## Authentication

### Interactive login

```sh
huly login --url https://huly.example.com --email user@example.com --workspace myworkspace
```

You will be prompted for your password. On success, credentials are saved to `<config-dir>/huly/credentials.yaml` (mode 0600). The config dir follows the OS convention: `~/.config` on Linux, `~/Library/Application Support` on macOS, `%AppData%` on Windows.

### Non-interactive token

For CI or scripted use, store a pre-generated app/API token directly:

```sh
huly auth set-token \
  --endpoint https://huly.example.com \
  --workspace <workspace-uuid> \
  --token <api-token>
```

### Environment variable overrides

The three connection parameters can be supplied as environment variables, which take precedence over the credentials file:

| Variable         | Purpose                           |
|------------------|-----------------------------------|
| `HULY_ENDPOINT`  | Transactor REST base URL          |
| `HULY_WORKSPACE` | Workspace UUID / URL              |
| `HULY_TOKEN`     | App or API token                  |

### Verify login

```sh
huly whoami
```

### Log out

```sh
huly logout
```

## Planning commands

### Projects

```sh
huly project list
huly project list --output json
```

### Issues

```sh
# List issues in a project
huly issue list --project PROJ
huly issue list --project PROJ --status "In Progress"

# View one issue
huly issue view PROJ-42

# Create an issue
huly issue create --project PROJ --title "Fix login redirect" --priority High
huly issue create --project PROJ --title "Add export endpoint" \
  --priority Medium --status "Todo" --component backend --milestone "v1.2"

# Update an issue
huly issue update PROJ-42 --project PROJ --status "Done"
huly issue update PROJ-42 --project PROJ --priority Urgent --assignee <account-ref>
```

Priority values: `NoPriority`, `Urgent`, `High`, `Medium`, `Low`.

Available flags for `issue create` and `issue update`:

| Flag            | Description                     |
|-----------------|---------------------------------|
| `--title`       | Issue title                     |
| `--description` | Description text                |
| `--status`      | Status name (e.g. "In Progress")|
| `--priority`    | Priority value (see above)      |
| `--assignee`    | Assignee account reference      |
| `--component`   | Component label                 |
| `--milestone`   | Milestone label                 |

### Milestones

```sh
huly milestone list --project PROJ
huly milestone create --project PROJ --label "v1.2" --target-date 2025-09-30
```

### Components

```sh
huly component list --project PROJ
huly component get --project PROJ "backend"
huly component create --project PROJ --label "backend" --description "Backend services"
# `add` is an alias for `create`
huly component add --project PROJ --label "frontend"
```

## Cache

Shell completion reads from a local cache, not the live API. Run `huly cache sync` once after login (and again whenever projects, components, milestones, or statuses change) so TAB completion has current data.

```sh
huly cache sync
# Limit to one project to speed up the sync:
huly cache sync --project PROJ
```

Example output:

```
cache synced: 3 projects, 142 issues, 8 components, 5 milestones, 6 statuses
```

## Shell completion

### zsh

```sh
# Add to ~/.zshrc (sourced on every shell start):
source <(huly completion zsh)

# Or install persistently into fpath:
huly completion zsh > "${fpath[1]}/_huly"
```

### bash

```sh
# Add to ~/.bashrc:
source <(huly completion bash)
```

### fish

```sh
huly completion fish | source
# Or persist:
huly completion fish > ~/.config/fish/completions/huly.fish
```

After installing completion and running `huly cache sync`, pressing TAB on `--project`, `--status`, `--priority`, `--component`, and `--milestone` flags will offer completions from the local cache without hitting the network.

## Global flags

| Flag               | Default   | Description                         |
|--------------------|-----------|-------------------------------------|
| `--output`         | `table`   | Output format: `table` or `json`    |
| `--quiet` / `-q`   | false     | Suppress stdout                     |
| `--config`         | (none)    | Override config file path           |

## Configuration file

`huly` looks for `./config/huly.yaml` or `./huly.yaml` by default. Copy the example and edit it:

```sh
cp config/huly.yaml.example config/huly.yaml
```

```yaml
server:
  url: https://huly.example.com   # default --url for `huly login`
defaults:
  project: PROJ                   # default --project when the flag is omitted
output: table                     # table | json
```

All keys map to `HULY_<KEY>` environment variables (dots replaced with underscores), e.g. `HULY_SERVER_URL` and `HULY_DEFAULTS_PROJECT`.

## Development

Prerequisites: Go 1.23+, [just](https://github.com/casey/just).

```sh
just build          # Build binary → bin/huly
just test           # Run tests (go test ./...)
just lint           # Format + lint
just release::all   # Cross-compile for all platforms → dist/
```

## License

[Add your license here]
