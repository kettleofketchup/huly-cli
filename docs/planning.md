# Planning commands

Planning in Huly revolves around **projects**, **issues**, **milestones**, and
**components**. Each sub-command follows the same `list` / `get` / `create` /
`update` / `delete` shape.

## Projects

```sh
huly project list
huly project get <identifier>
huly project create --title "My Project" --identifier PROJ
```

## Issues

```sh
huly issue list --project PROJ
huly issue get PROJ-42
huly issue create --project PROJ --title "Title" [flags]
huly issue update PROJ-42 [flags]
huly issue delete PROJ-42
```

### Issue create flags

| Flag | Description |
|------|-------------|
| `--project` | Project identifier (required) |
| `--title` | Issue title (required) |
| `--priority` | Priority (see below) |
| `--status` | Status name |
| `--assignee` | Assignee ref |
| `--milestone` | Milestone title |
| `--component` | Component title |

!!! note "Priority values"
    Valid values (case-insensitive): `Urgent`, `High`, `Medium`, `Low`, `No priority`.

### Issue update flags

Same flags as `create`; only supply the fields you want to change.

## Milestones

```sh
huly milestone list --project PROJ
huly milestone get --project PROJ --milestone "Sprint 1"
huly milestone create --project PROJ --title "Sprint 1"
```

!!! note "Milestone targeting"
    `--milestone` accepts the milestone **title**. The CLI resolves it to an
    internal ref automatically.

## Filtering and output

All `list` commands accept `--output json` for piping into `jq`:

```sh
huly issue list --project PROJ --output json | jq '.[].title'
```
