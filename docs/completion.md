# Shell completion

`huly` generates completion scripts for bash, zsh, fish, and PowerShell via
Cobra's built-in `__complete` mechanism. Project identifiers, issue refs,
statuses, milestones, and components are all completed from the local cache.

!!! tip "Populate the cache first"
    Run `huly cache sync` before using TAB completion — the completion functions
    read only the local cache.

## Install

=== "zsh"

    ```sh
    source <(huly completion zsh)
    # persist:
    huly completion zsh > "${fpath[1]}/_huly"
    ```

=== "bash"

    ```sh
    source <(huly completion bash)
    # persist:
    huly completion bash > /etc/bash_completion.d/huly
    ```

=== "fish"

    ```sh
    huly completion fish | source
    # persist:
    huly completion fish > ~/.config/fish/completions/huly.fish
    ```

=== "PowerShell"

    ```powershell
    huly completion powershell | Out-String | Invoke-Expression
    # persist: add the above line to your $PROFILE
    ```

## What completes

| Argument / flag | Completions |
|-----------------|-------------|
| `--project` | Project identifiers from cache |
| `--status` | Status names for the selected project |
| `--milestone` | Milestone titles for the selected project |
| `--component` | Component titles for the selected project |
| `--priority` | `Urgent`, `High`, `Medium`, `Low`, `No priority` |
| `--output` | `table`, `json` |
| Issue ref (e.g. `PROJ-42`) | Recent issue refs from cache |

!!! note "Completion and quiet mode"
    `huly` suppresses all init noise and non-completion output when invoked as
    `__complete` or `__completeNoDesc`, so piped completion output is always clean.
