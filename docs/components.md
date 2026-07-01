# Components

Components represent logical areas of a project (e.g., "Backend", "Frontend",
"Infrastructure"). They can be attached to issues for organisation and filtering.

## Commands

```sh
huly component list --project PROJ
huly component get --project PROJ --component "Backend"
huly component create --project PROJ --title "Backend"
huly component delete --project PROJ --component "Backend"
```

## Attaching a component to an issue

Pass `--component` when creating or updating an issue:

```sh
huly issue create --project PROJ --title "Add retry logic" --component "Backend"
huly issue update PROJ-10 --component "Infrastructure"
```

!!! note "Component resolution"
    `--component` accepts the component **title**. TAB completion lists available
    components from the local cache — run `huly cache sync` to keep it current.

## Listing with output options

```sh
huly component list --project PROJ --output json | jq '.[].title'
```
