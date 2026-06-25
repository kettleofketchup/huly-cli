# Config management recipes
# Call as: just config::schema, just config::check

TOOL_NAME := "huly"
TOOL_FOLDER := justfile_directory() + "/src/" + TOOL_NAME

# Generate JSON Schema from config struct
[no-cd]
[group('config')]
schema:
    #!/usr/bin/env bash
    set -euo pipefail
    cd {{ TOOL_FOLDER }} && go run . config schema > {{ justfile_directory() }}/config/schema.json
    echo "Generated config/schema.json"

# Check that schema.json is up to date with config struct
[no-cd]
[group('config')]
check:
    #!/usr/bin/env bash
    set -euo pipefail
    TMPFILE=$(mktemp)
    trap "rm -f $TMPFILE" EXIT
    cd {{ TOOL_FOLDER }} && go run . config schema > "$TMPFILE"
    if ! diff -q config/schema.json "$TMPFILE" > /dev/null 2>&1; then
        echo "ERROR: config/schema.json is out of date with config struct"
        echo "Run: just config::schema"
        exit 1
    fi
    cd {{ TOOL_FOLDER }} && go run . --config {{ justfile_directory() }}/config/{{ TOOL_NAME }}.yaml.example config show > /dev/null
    echo "Config schema and example are up to date"
