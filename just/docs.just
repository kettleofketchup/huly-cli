# Documentation recipes — call as: just docs::serve, just docs::build

# Start Zensical development server with live reload
serve:
    @echo "Starting docs server with live reload..."
    uv run zensical serve --open

# Build static documentation into ./public
build:
    @echo "Building documentation..."
    uv run zensical build --clean

# Clean generated documentation
clean:
    rm -rf public/
    rm -rf site/
