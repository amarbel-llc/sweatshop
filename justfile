set output-format := "tap"

default: build test

build: build-gomod2nix build-go

# Build Go binary directly
build-go: build-gomod2nix
    nix develop --command go build -o build/sweatshop ./cmd/sweatshop

# Regenerate gomod2nix.toml
build-gomod2nix:
    nix develop --command gomod2nix

# Run the binary
run-nix *ARGS:
    nix run . -- {{ARGS}}

test: test-go test-bats

test-go:
    nix develop --command go test ./...

test-bats:
    nix develop --command bats --tap tests/

codemod-fmt: codemod-fmt-go

codemod-fmt-go:
    nix develop --command gofumpt -w .

update-go: && build-gomod2nix
    nix develop --command go mod tidy

clean: clean-build

clean-build:
    rm -rf result build/
