# Development recipes

# Bootstrap the development environment
dev:
    @echo "Setting up development environment..."
    uv sync
    @command -v lefthook > /dev/null 2>&1 || { echo "Installing lefthook..."; go install github.com/evilmartians/lefthook@latest; }
    lefthook install
    @echo "Done! Development environment ready."
    @echo ""
    @echo "Available commands:"
    @echo "  just build        Build the binary"
    @echo "  just test         Run tests"
    @echo "  just lint         Run linter"
    @echo "  just run          Build and run"
    @echo "  just docs-serve   Start docs dev server"
