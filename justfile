# sweatshop - git worktree session manager

default:
    @just --list

# Build the nix package
build:
    nix build

# Build Go binary directly
build-go: build-gomod2nix
    nix develop --command go build -o build/sweatshop ./cmd/sweatshop

# Regenerate gomod2nix.toml
build-gomod2nix:
    nix develop --command gomod2nix

# Run the binary
run *ARGS:
    nix run . -- {{ARGS}}

# Run Go unit tests
test:
    nix develop --command go test ./...

# Run bats integration tests
test-bats: build
    nix develop --command bats --tap tests/

# Format Go code
fmt:
    nix develop --command gofumpt -w .

# Update Go dependencies
deps:
    nix develop --command go mod tidy
    nix develop --command gomod2nix

# Clean build artifacts
clean:
    rm -rf result build/
