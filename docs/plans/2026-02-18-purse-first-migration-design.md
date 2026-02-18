# Purse-First Command Framework Migration

## Goal

Migrate sweatshop from cobra to purse-first's `command.App` framework, producing two binaries (`sweatshop` for CLI, `sweatshop-mcp` for MCP) from a single command definition. Extend `command.App` to support both execution modes with a unified handler, a Prompter interface for optional interactivity, and a CLI-only escape hatch.

## Motivation

- Single source of truth for commands: one definition generates CLI parsing, MCP tool registration, plugin.json, manpages, and shell completions.
- Consistency with grit and other purse-first projects.
- Sweatshop gains MCP tool exposure (status, merge, clean, perms-check, perms-list) without duplicating command definitions.

## Design

### Command struct

Replace the current `RunMCP` field with two optional handlers:

```go
type Command struct {
    Name        string
    Aliases     []string
    Description Description
    Hidden      bool
    Params      []Param
    MapsBash    []BashMapping

    // Run: unified handler for both MCP and CLI.
    // args is json.RawMessage (from MCP directly, or marshaled from parsed CLI flags).
    // Prompter is huh-backed in CLI mode, returns errors in MCP mode.
    Run func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error)

    // RunCLI: CLI-only handler, skipped by RegisterMCPTools and GeneratePlugin.
    RunCLI func(ctx context.Context, args json.RawMessage) error
}
```

Handler dispatch rules:

| Run set | RunCLI set | MCP behavior | CLI behavior |
|---------|-----------|--------------|--------------|
| yes     | no        | Registered as MCP tool | Dispatched with HuhPrompter |
| no      | yes       | Skipped | Dispatched directly |
| yes     | yes       | Run used for MCP | RunCLI used for CLI |
| no      | no        | Skipped | Metadata-only (no execution) |

### Result type

```go
type Result struct {
    Text  string // plain text (CLI prints to stdout, MCP wraps in TextContent)
    JSON  any    // structured data (CLI marshals+prints, MCP marshals into TextContent)
    IsErr bool   // MCP: sets isError on ToolCallResult
}
```

The CLI runner prints `Result.Text` or marshals `Result.JSON`. The MCP adapter wraps it into `protocol.ToolCallResult`.

### Prompter interface

```go
type Prompter interface {
    Confirm(msg string) (bool, error)
    Select(msg string, options []string) (int, error)
    Input(msg string) (string, error)
}
```

Two implementations:

- **HuhPrompter** (package `command/huh/`): wraps charmbracelet/huh forms. Isolated in a sub-package so the MCP binary never imports huh or bubbletea.
- **StubPrompter** (package `command/`): every method returns `fmt.Errorf("interactive prompt not available in MCP mode")`.

### CLI runner

New method on App:

```go
func (a *App) RunCLI(ctx context.Context, args []string, p Prompter) error
```

Behavior:

1. Parse global flags (`App.Params`) from the front of `args`.
2. First non-flag arg is the subcommand name (looked up via `GetCommand`).
3. Parse remaining args as `--flags` using the command's `Params`:
   - `String`: `--name value` or `--name=value`
   - `Bool`: `--name` (true), `--name=false`
   - `Int`: `--name 42`
   - `Float`: `--name 3.14`
   - `Array`: `--name val1 --name val2`
4. Marshal parsed values into `json.RawMessage`.
5. Dispatch: `RunCLI` if set, else `Run` with the provided Prompter.
6. Print `Result` (text to stdout, or marshal JSON).
7. No subcommand or `--help`: print usage from `Description` and `Params`.

CLI main entry point:

```go
func main() {
    app := buildApp()
    if err := app.RunCLI(context.Background(), os.Args[1:], huh.NewPrompter()); err != nil {
        fmt.Fprintf(os.Stderr, "%s: %v\n", app.Name, err)
        os.Exit(1)
    }
}
```

### MCP binary pattern

Separate `cmd/<name>-mcp/main.go` reuses the same `buildApp()`:

```go
func main() {
    app := buildApp()
    t := transport.NewStdio(os.Stdin, os.Stdout)
    registry := server.NewToolRegistry()
    app.RegisterMCPTools(registry)
    srv, _ := server.New(t, server.Options{
        ServerName:    app.Name,
        ServerVersion: app.Version,
        Tools:         registry,
    })
    srv.Run(context.Background())
}
```

`RegisterMCPTools` wraps each `cmd.Run` with a `StubPrompter` and adapts `Result` to `protocol.ToolCallResult`. Commands with only `RunCLI` are skipped.

`GeneratePlugin` points the manifest command at `<name>-mcp` instead of `<name>`.

### Sweatshop command mapping

Shared `buildApp()` function used by both binaries:

```go
func buildApp() *command.App {
    app := command.NewApp("sweatshop", "Shell-agnostic git worktree session manager")
    app.Version = "0.1.0"
    app.Params = []command.Param{
        {Name: "format", Type: command.String, Description: "output format: tap or table"},
    }
    registerStatusCommands(app)
    registerOpenCommands(app)
    registerMergeCommands(app)
    registerCleanCommands(app)
    registerCompletionsCommands(app)
    app.MergeWithPrefix(perms.BuildApp(), "perms")
    return app
}
```

| Command | Handler | Rationale |
|---------|---------|-----------|
| status | Run | Non-interactive. Returns rows as JSON or table/tap. |
| merge | Run | Non-interactive. Runs git merge, returns success/failure. |
| clean | Run + Prompter | `Prompter.Confirm()` for `-i` mode. Without `-i`, no prompts. |
| completions | RunCLI | Writes tab-separated output to stdout. No MCP use case. |
| open | RunCLI | Launches zmx sessions, interactive close-shop flow. Terminal operation. |
| perms-check | Run | Non-interactive hook handler. |
| perms-list | Run | Non-interactive, returns tier rules. |
| perms-review | RunCLI | Interactive huh form. |
| perms-edit | RunCLI | Launches $EDITOR. |

## Changes by repository

### purse-first (libs/go-mcp/command/)

1. Add `Prompter` interface and `StubPrompter` to command package.
2. Add `HuhPrompter` in sub-package `command/huh/` to isolate dependency.
3. Add `Result` type.
4. Replace `RunMCP` field with `Run` and `RunCLI` on `Command`.
5. Add `App.RunCLI()` with flag parsing from `Param` declarations.
6. Update `RegisterMCPTools` to wrap `Run` with stub prompter and Result-to-ToolCallResult adapter.
7. Update `GeneratePlugin` to point at `<name>-mcp` binary.
8. Update `GenerateCompletions` and `GenerateManpages` to include `RunCLI` commands.

### sweatshop

1. Add shared `buildApp()` function using `command.App`.
2. Replace `cmd/sweatshop/main.go`: cobra root becomes `app.RunCLI()`.
3. Add `cmd/sweatshop-mcp/main.go`: MCP server binary.
4. Migrate each command's logic into `Run` or `RunCLI` handlers.
5. Replace direct huh usage in `clean -i` with `Prompter.Confirm()`.
6. Update `flake.nix` to build both binaries and run `GenerateAll` for artifacts.
7. Remove cobra dependency.

### grit (follow-up)

1. Rename `RunMCP` to `Run`, add ignored `Prompter` param, wrap returns in `Result`.
2. Add `cmd/grit-mcp/main.go` or designate current main as the MCP binary and add a CLI entry point.
