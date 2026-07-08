# Agent skills

`huly skills` installs huly's embedded agent skills into the AI coding agents
on your machine, so they know how to use the huly CLI. The same native
`SKILL.md` format works across Claude Code, Codex, opencode, Cursor, and Pi.

## Commands

```sh
huly skills list                 # status of each skill per detected agent
huly skills install --all        # install into every detected agent
huly skills install --agents claude,codex
huly skills update --all         # refresh skills that are behind
huly skills uninstall --all      # remove huly-owned skills
```

## Selecting agents

`install`/`update`/`uninstall` require an explicit target:

- `--all` — every agent detected on your machine.
- `--agents <ids>` — a comma-separated subset (`claude,codex,opencode,cursor,pi`).

`huly skills list` shows every detected agent with no selector.

## Interactive mode

At a terminal, `huly skills` with no subcommand opens an interactive dashboard
(pick skills → pick agents → pick an action). `huly skills tui` forces the
dashboard and errors if you're not on a terminal.

`install`/`update`/`uninstall` with no `--agents`/`--all` at a terminal open a
pre-checked agent picker, then a confirmation. Add `--yes` to skip the
confirmation, or `--no-interactive` to force the non-interactive behavior
(which then requires `--all` or `--agents`). In a pipe or script (no TTY) the
commands are always non-interactive and require an explicit selector.

The dashboard always confirms before applying and does not accept
`--force`/`--dry-run`/`--fail-on-conflict`; for those, use the explicit
commands (e.g. `huly skills install --force`).

## Flags

| Flag | Meaning |
|------|---------|
| `--all` | target all detected agents |
| `--agents <csv>` | target the named agents |
| `--force` | overwrite a conflicting/edited/foreign skill (the old copy is backed up to `<dir>.bak-<n>` first) |
| `--dry-run` | print what would change; write nothing |
| `--fail-on-conflict` | exit non-zero if any target conflicts (for CI) |
| `--output json` | machine-readable results |
| `--yes` | skip the interactive confirmation |
| `--no-interactive` | never prompt; require `--all`/`--agents` |

## How updates work

Each installed skill records the shipping huly version and a content hash in
its `SKILL.md` frontmatter. `huly skills update` re-installs a skill only when
the embedded content actually differs — it never clobbers a skill you edited
(that reports `conflict`/`modified`; use `--force`, which backs up first). After
`huly update` upgrades the binary, run `huly skills update` to refresh.

## Result tokens

`installed`, `updated`, `repaired`, `up-to-date`, `conflict`, `removed`,
`skipped` (and `error`). Text output prefixes each line with the token, so
`huly skills install --all | grep conflict` works.
