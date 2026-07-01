# Cache sync

`huly` caches project metadata locally so shell completion works offline and
fast. The cache lives at `<config-dir>/huly/cache.json` and is protected by a
file lock to prevent concurrent corruption.

## Commands

```sh
# Populate (or refresh) the cache from the server
huly cache sync

# Inspect cache contents
huly cache show

# Remove the cache file
huly cache clear
```

!!! warning "Cache is required for TAB completion"
    Completion functions read **only** the local cache. If TAB completion returns
    nothing, run `huly cache sync`.

## What is cached

| Key | Contents |
|-----|----------|
| `projects` | Identifier + title for every project |
| `statuses` | Status names per project |
| `milestones` | Milestone titles per project |
| `components` | Component titles per project |

## Auto-healing

When a cached ref is no longer found on the server, `huly` prints a hint:

```
hint: ref not found — run 'huly cache sync' to refresh
```

The stale entry is pruned from the cache automatically on next sync.

## Manual cache location

```sh
# Linux
cat ~/.config/huly/cache.json

# macOS
cat ~/Library/Application\ Support/huly/cache.json
```
