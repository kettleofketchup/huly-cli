# huly-cli

huly-cli CLI tool.

<!--doc-start-->
## Installation

### From Source

```sh
git clone gitlab.lan/huly-cli.git
cd huly-cli
./dev  # Bootstrap environment
just build
```

### Docker

```sh
docker pull gitlab.lan:5050/huly-cli/huly:latest
docker run --rm gitlab.lan:5050/huly-cli/huly:latest --help
```

## Usage

```sh
# Show help
./bin/huly --help

# Show version
./bin/huly version

# Use with config file
./bin/huly --config ./config/huly.yaml
```

## Development

### Prerequisites

- Go 1.23+
- golangci-lint
- [just](https://github.com/casey/just) (auto-installed by `./dev`)
- uv (for documentation)

### Quick Start

```sh
./dev  # Bootstrap environment, install just if needed
```

### Build

```sh
just build          # Build binary
just test           # Run tests
just lint           # Run linter
just release::all   # Build for all platforms
```

### Documentation

```sh
just docs::serve    # Start dev server at localhost:8000
just docs::build    # Build static documentation
```

### Docker

```sh
just docker::build  # Build Docker image
just docker::push   # Push to registry
```
<!--doc-end-->

## License

[Add your license here]
