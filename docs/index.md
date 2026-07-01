# huly

`huly` is a self-updating command-line client for [Huly](https://huly.io) tracker
planning. It manages **projects, issues, milestones, and components**, with
session + app-token authentication and cache-backed shell completion.

!!! tip "New here?"
    1. [Install](#install) the binary, 2. [authenticate](auth.md), 3. run
    `huly cache sync` to power TAB completion, then 4. start
    [planning](planning.md).

## Install

=== "Go install"

    ```sh
    go install github.com/kettleofketchup/huly-cli/src/huly@latest
    ```

=== "From source"

    ```sh
    git clone https://github.com/kettleofketchup/huly-cli
    cd huly-cli && just build
    ./bin/huly version
    ```

=== "Self-update"

    ```sh
    huly update
    ```

## Quick start

```sh
huly login --url https://huly.example.com --email you@example.com --workspace myws
huly cache sync
huly project list
huly issue create --project PROJ --title "Fix the thing" --priority High
```

!!! warning "Run `cache sync` after logging in"
    Shell completion reads **only** the local cache (never the network), so TAB
    completion is empty until you run `huly cache sync` at least once.
