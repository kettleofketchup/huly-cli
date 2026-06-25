# huly-cli - Go CLI project
# Run `just --list` to see all available recipes

set quiet
set dotenv-load

# Modules - call as `just module::recipe`
mod certs 'just/certs.just'
mod config 'just/config.just'
mod cicd 'just/cicd.just'
mod compose 'just/compose.just'
mod copier 'just/copier.just'
mod docker 'just/docker.just'
mod docs 'just/docs.just'
mod git 'just/git.just'
mod go 'just/go.just'
mod release 'just/release.just'
mod testing 'just/testing.just'

# Import top-level recipes (merged into root namespace)
import 'just/dev.just'

# Variables
TOOL_NAME := "huly"
PROJECT_NAME := "huly-cli"
TOOL_FOLDER := justfile_directory() + "/src/" + TOOL_NAME

# List all available recipes
default:
    just --list

# Build the CLI binary (alias for go::build)
[group('dev')]
build:
    just go::build {{ TOOL_NAME }}

# Run tests (alias for go::test)
[group('dev')]
[group('ci')]
test:
    just go::test {{ TOOL_NAME }}

# Run linter (alias for go::lint)
[group('dev')]
[group('ci')]
lint:
    just go::lint {{ TOOL_NAME }}

# Build and run the binary
[group('dev')]
run: build
    ./bin/{{ TOOL_NAME }}
