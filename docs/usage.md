# Usage

`huly` is a Cobra-based CLI. Every command accepts `--help` for inline help.

```sh
huly [command] [flags]
```

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `~/.config/huly/huly.yaml` | Config file path |
| `--output`, `-o` | `table` | Output format: `table`, `json` |
| `--quiet`, `-q` | `false` | Suppress table/JSON output |

## Top-level commands

| Command | Description |
|---------|-------------|
| `huly login` | Interactive login (session token) |
| `huly logout` | Clear stored credentials |
| `huly whoami` | Print current identity |
| `huly auth set-token` | Store an app token (non-interactive) |
| `huly project` | Project sub-commands |
| `huly issue` | Issue sub-commands |
| `huly milestone` | Milestone sub-commands |
| `huly component` | Component sub-commands |
| `huly cache` | Cache management |
| `huly completion` | Shell completion scripts |
| `huly update` | Self-update the binary |
| `huly version` | Print version info |

!!! note "Output format"
    Use `--output json` (or `-o json`) on `list` and `get`/`view` commands to
    get machine-readable output suitable for `jq` pipelines.

## Examples

```sh
# List projects
huly project list

# Get a single project
huly project get PROJ

# List issues in a project
huly issue list --project PROJ

# Create an issue
huly issue create --project PROJ --title "Fix login" --priority High

# View an issue
huly issue view PROJ-42

# Self-update
huly update
```
