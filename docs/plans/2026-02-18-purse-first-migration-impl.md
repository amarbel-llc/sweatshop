# Purse-First Command Framework Migration — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extend purse-first's `command.App` to support unified CLI+MCP handlers with a Prompter interface, then migrate sweatshop from cobra to use it.

**Architecture:** Two-phase: (1) extend `command.App` in purse-first with `Result`, `Prompter`, `Run`/`RunCLI` fields, a CLI flag parser, and updated artifact generators; (2) rewrite sweatshop's commands using the new framework, producing `sweatshop` (CLI) and `sweatshop-mcp` (MCP server) binaries.

**Tech Stack:** Go, purse-first `libs/go-mcp/command`, charmbracelet/huh (CLI prompter only), `go-lib-mcp/protocol` + `go-lib-mcp/server` (MCP)

---

## Phase 1: purse-first framework changes

All tasks in this phase are in the **purse-first** repo (`/Users/sfriedenberg/eng/worktrees/purse-first/bob/`), specifically `libs/go-mcp/command/`.

### Task 1: Add Result type and Prompter interface

**Files:**
- Create: `libs/go-mcp/command/result.go`
- Create: `libs/go-mcp/command/prompter.go`
- Test: `libs/go-mcp/command/result_test.go`
- Test: `libs/go-mcp/command/prompter_test.go`

**Step 1: Write the failing test for Result**

```go
// result_test.go
package command

import "testing"

func TestResultText(t *testing.T) {
	r := &Result{Text: "hello"}
	if r.Text != "hello" {
		t.Errorf("Text = %q, want %q", r.Text, "hello")
	}
	if r.IsErr {
		t.Error("IsErr should be false by default")
	}
}

func TestResultJSON(t *testing.T) {
	r := &Result{JSON: map[string]string{"key": "val"}}
	if r.JSON == nil {
		t.Error("JSON should not be nil")
	}
}

func TestErrorResult(t *testing.T) {
	r := TextErrorResult("something failed")
	if !r.IsErr {
		t.Error("IsErr should be true")
	}
	if r.Text != "something failed" {
		t.Errorf("Text = %q, want %q", r.Text, "something failed")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestResult -v`
Expected: FAIL — `Result` and `TextErrorResult` not defined

**Step 3: Write Result type**

```go
// result.go
package command

// Result holds the output of a command handler, used by both CLI and MCP runners.
type Result struct {
	Text  string // plain text output
	JSON  any    // structured output (marshaled to JSON for display)
	IsErr bool   // marks this result as an error for MCP
}

// TextResult creates a Result with plain text.
func TextResult(text string) *Result {
	return &Result{Text: text}
}

// JSONResult creates a Result with structured data.
func JSONResult(v any) *Result {
	return &Result{JSON: v}
}

// TextErrorResult creates an error Result with plain text.
func TextErrorResult(text string) *Result {
	return &Result{Text: text, IsErr: true}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestResult -v`
Expected: PASS

**Step 5: Write the failing test for Prompter and StubPrompter**

```go
// prompter_test.go
package command

import "testing"

func TestStubPrompterConfirm(t *testing.T) {
	p := StubPrompter{}
	_, err := p.Confirm("proceed?")
	if err == nil {
		t.Error("StubPrompter.Confirm should return error")
	}
}

func TestStubPrompterSelect(t *testing.T) {
	p := StubPrompter{}
	_, err := p.Select("choose:", []string{"a", "b"})
	if err == nil {
		t.Error("StubPrompter.Select should return error")
	}
}

func TestStubPrompterInput(t *testing.T) {
	p := StubPrompter{}
	_, err := p.Input("name?")
	if err == nil {
		t.Error("StubPrompter.Input should return error")
	}
}
```

**Step 6: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestStubPrompter -v`
Expected: FAIL — `Prompter` and `StubPrompter` not defined

**Step 7: Write Prompter interface and StubPrompter**

```go
// prompter.go
package command

import "fmt"

// Prompter provides interactive prompts. CLI implementations use terminal UI;
// MCP implementations return errors since mid-call prompting is not supported.
type Prompter interface {
	Confirm(msg string) (bool, error)
	Select(msg string, options []string) (int, error)
	Input(msg string) (string, error)
}

// StubPrompter returns errors for all prompts. Used in MCP mode.
type StubPrompter struct{}

func (StubPrompter) Confirm(msg string) (bool, error) {
	return false, fmt.Errorf("interactive prompt not available in MCP mode: %s", msg)
}

func (StubPrompter) Select(msg string, options []string) (int, error) {
	return 0, fmt.Errorf("interactive prompt not available in MCP mode: %s", msg)
}

func (StubPrompter) Input(msg string) (string, error) {
	return "", fmt.Errorf("interactive prompt not available in MCP mode: %s", msg)
}
```

**Step 8: Run test to verify it passes**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestStubPrompter -v`
Expected: PASS

**Step 9: Commit**

```bash
git add libs/go-mcp/command/result.go libs/go-mcp/command/result_test.go \
       libs/go-mcp/command/prompter.go libs/go-mcp/command/prompter_test.go
git commit -m "feat(command): add Result type and Prompter interface"
```

---

### Task 2: Replace RunMCP with Run and RunCLI on Command

**Files:**
- Modify: `libs/go-mcp/command/command.go`
- Modify: `libs/go-mcp/command/command_test.go`
- Modify: `libs/go-mcp/command/mcp.go`
- Modify: `libs/go-mcp/command/mcp_test.go`

**Step 1: Write failing test for the new Command fields**

Add to `command_test.go`:

```go
func TestCommandHasRunAndRunCLI(t *testing.T) {
	cmd := Command{
		Name: "status",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult("ok"), nil
		},
	}
	if cmd.Run == nil {
		t.Error("Run should be set")
	}
	if cmd.RunCLI != nil {
		t.Error("RunCLI should be nil")
	}
}

func TestCommandCLIOnly(t *testing.T) {
	cmd := Command{
		Name: "open",
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			return nil
		},
	}
	if cmd.Run != nil {
		t.Error("Run should be nil for CLI-only commands")
	}
	if cmd.RunCLI == nil {
		t.Error("RunCLI should be set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestCommandHasRun -v`
Expected: FAIL — `Command` struct has `RunMCP`, not `Run`

**Step 3: Update Command struct**

In `libs/go-mcp/command/command.go`, replace the `RunMCP` field:

```go
// Command declares a single subcommand with all metadata needed
// to generate CLI parsing, MCP tool registration, manpages,
// completions, and plugin manifests.
type Command struct {
	Name        string
	Aliases     []string
	Description Description
	Hidden      bool

	Params   []Param
	MapsBash []BashMapping

	// Run handles both MCP tool invocations and CLI execution.
	// In MCP mode, Prompter is a StubPrompter that returns errors.
	// In CLI mode, Prompter is a real interactive implementation.
	Run func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error)

	// RunCLI handles CLI-only invocations. Commands with only RunCLI
	// are not registered as MCP tools or included in plugin.json.
	RunCLI func(ctx context.Context, args json.RawMessage) error
}
```

**Step 4: Update RegisterMCPTools to use Run instead of RunMCP**

In `libs/go-mcp/command/mcp.go`:

```go
func (a *App) RegisterMCPTools(registry *server.ToolRegistry) {
	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		if cmd.Run == nil {
			continue
		}

		run := cmd.Run
		registry.Register(
			cmd.Name,
			cmd.Description.Short,
			cmd.InputSchema(),
			func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
				result, err := run(ctx, args, StubPrompter{})
				if err != nil {
					return nil, err
				}
				return resultToMCP(result), nil
			},
		)
	}
}

func resultToMCP(r *Result) *protocol.ToolCallResult {
	var text string
	if r.JSON != nil {
		data, _ := json.Marshal(r.JSON)
		text = string(data)
	} else {
		text = r.Text
	}
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{protocol.TextContent(text)},
		IsError: r.IsErr,
	}
}
```

Note: check the `protocol.ToolCallResult` struct for the exact `IsError` field name — it may be `IsError` or `isError`. Read the protocol package to confirm.

**Step 5: Update mcp_test.go to use Run instead of RunMCP**

Replace all `RunMCP:` with `Run:` and update signatures to include `Prompter` parameter and return `*Result`:

```go
func TestAppRegisterMCPTools(t *testing.T) {
	app := NewApp("grit", "Git MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult("ok"), nil
		},
	})

	app.AddCommand(&Command{
		Name:   "internal",
		Hidden: true,
	})

	// CLI-only command should not be registered
	app.AddCommand(&Command{
		Name: "interactive",
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			return nil
		},
	})

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	tools, err := registry.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1 (hidden and CLI-only excluded)", len(tools))
	}

	if tools[0].Name != "status" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "status")
	}
}

func TestAppMCPToolCall(t *testing.T) {
	app := NewApp("test", "test")

	app.AddCommand(&Command{
		Name: "echo",
		Params: []Param{
			{Name: "message", Type: String, Description: "Message to echo"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Message string `json:"message"`
			}
			json.Unmarshal(args, &params)
			return TextResult(params.Message), nil
		},
	})

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	result, err := registry.CallTool(
		context.Background(),
		"echo",
		json.RawMessage(`{"message":"hello"}`),
	)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.Content[0].Text != "hello" {
		t.Errorf("result = %q, want %q", result.Content[0].Text, "hello")
	}
}
```

**Step 6: Run all tests**

Run: `nix develop --command go test ./libs/go-mcp/command/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add libs/go-mcp/command/command.go libs/go-mcp/command/command_test.go \
       libs/go-mcp/command/mcp.go libs/go-mcp/command/mcp_test.go
git commit -m "feat(command): replace RunMCP with unified Run and CLI-only RunCLI"
```

---

### Task 3: Add CLI flag parser and App.RunCLI

**Files:**
- Create: `libs/go-mcp/command/cli.go`
- Create: `libs/go-mcp/command/cli_test.go` (replace existing file)

**Step 1: Write failing tests for flag parsing**

Replace `libs/go-mcp/command/cli_test.go` with:

```go
package command

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRunCLIDispatchesRun(t *testing.T) {
	var called bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "greet",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name to greet", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			called = true
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(args, &params)
			return TextResult("hello " + params.Name), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"greet", "--name", "world"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !called {
		t.Error("Run handler was not called")
	}
}

func TestRunCLIDispatchesRunCLI(t *testing.T) {
	var called bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "open",
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			called = true
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"open"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !called {
		t.Error("RunCLI handler was not called")
	}
}

func TestRunCLIPrefersRunCLIOverRun(t *testing.T) {
	var ranCLI bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "dual",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			t.Error("Run should not be called when RunCLI is set")
			return TextResult(""), nil
		},
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			ranCLI = true
			return nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"dual"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !ranCLI {
		t.Error("RunCLI handler was not called")
	}
}

func TestRunCLIBoolFlag(t *testing.T) {
	var got bool
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Verbose bool `json:"verbose"`
			}
			json.Unmarshal(args, &params)
			got = params.Verbose
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--verbose"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if !got {
		t.Error("verbose should be true")
	}
}

func TestRunCLIIntFlag(t *testing.T) {
	var got int
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "count", Type: Int, Description: "Count"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Count int `json:"count"`
			}
			json.Unmarshal(args, &params)
			got = params.Count
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--count", "42"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != 42 {
		t.Errorf("count = %d, want 42", got)
	}
}

func TestRunCLIArrayFlag(t *testing.T) {
	var got []string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "tags", Type: Array, Description: "Tags"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Tags []string `json:"tags"`
			}
			json.Unmarshal(args, &params)
			got = params.Tags
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--tags", "a", "--tags", "b"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("tags = %v, want [a b]", got)
	}
}

func TestRunCLIGlobalParams(t *testing.T) {
	var format string
	app := NewApp("test", "test app")
	app.Params = []Param{
		{Name: "format", Type: String, Description: "Output format"},
	}
	app.AddCommand(&Command{
		Name: "status",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Format string `json:"format"`
			}
			json.Unmarshal(args, &params)
			format = params.Format
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"--format", "tap", "status"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if format != "tap" {
		t.Errorf("format = %q, want %q", format, "tap")
	}
}

func TestRunCLIUnknownCommand(t *testing.T) {
	app := NewApp("test", "test app")
	err := app.RunCLI(context.Background(), []string{"nonexistent"}, StubPrompter{})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestRunCLIEqualsFlag(t *testing.T) {
	var got string
	app := NewApp("test", "test app")
	app.AddCommand(&Command{
		Name: "cmd",
		Params: []Param{
			{Name: "name", Type: String, Description: "Name"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Name string `json:"name"`
			}
			json.Unmarshal(args, &params)
			got = params.Name
			return TextResult(""), nil
		},
	})

	err := app.RunCLI(context.Background(), []string{"cmd", "--name=alice"}, StubPrompter{})
	if err != nil {
		t.Fatalf("RunCLI: %v", err)
	}
	if got != "alice" {
		t.Errorf("name = %q, want %q", got, "alice")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestRunCLI -v`
Expected: FAIL — `App.RunCLI` not defined

**Step 3: Implement CLI runner**

Create `libs/go-mcp/command/cli.go`:

```go
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// RunCLI parses CLI arguments, dispatches to the matched command handler,
// and prints the result. Global params (App.Params) are parsed before
// the subcommand name; command params are parsed after.
func (a *App) RunCLI(ctx context.Context, args []string, p Prompter) error {
	globalVals := make(map[string]any)
	remaining, err := parseFlags(args, a.Params, globalVals)
	if err != nil {
		return fmt.Errorf("parsing global flags: %w", err)
	}

	if len(remaining) == 0 {
		a.printUsage()
		return nil
	}

	name := remaining[0]
	cmdArgs := remaining[1:]

	cmd, ok := a.GetCommand(name)
	if !ok {
		return fmt.Errorf("unknown command: %s", name)
	}

	cmdVals := make(map[string]any)
	for k, v := range globalVals {
		cmdVals[k] = v
	}

	_, err = parseFlags(cmdArgs, cmd.Params, cmdVals)
	if err != nil {
		return fmt.Errorf("parsing flags for %s: %w", name, err)
	}

	argsJSON, err := json.Marshal(cmdVals)
	if err != nil {
		return fmt.Errorf("marshaling args: %w", err)
	}

	if cmd.RunCLI != nil {
		return cmd.RunCLI(ctx, argsJSON)
	}

	if cmd.Run != nil {
		result, err := cmd.Run(ctx, argsJSON, p)
		if err != nil {
			return err
		}
		printResult(result)
		return nil
	}

	return fmt.Errorf("command %s has no handler", name)
}

func printResult(r *Result) {
	if r == nil {
		return
	}
	if r.JSON != nil {
		data, _ := json.MarshalIndent(r.JSON, "", "  ")
		fmt.Println(string(data))
	} else if r.Text != "" {
		fmt.Println(r.Text)
	}
}

func (a *App) printUsage() {
	fmt.Printf("%s — %s\n\n", a.Name, a.Description.Short)
	if a.Description.Long != "" {
		fmt.Printf("%s\n\n", a.Description.Long)
	}
	fmt.Println("Commands:")
	for name, cmd := range a.VisibleCommands() {
		fmt.Printf("  %-16s %s\n", name, cmd.Description.Short)
	}
}

// parseFlags extracts --flag values from args into vals, returning unconsumed args.
func parseFlags(args []string, params []Param, vals map[string]any) ([]string, error) {
	paramMap := make(map[string]Param)
	for _, p := range params {
		paramMap[p.Name] = p
	}

	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if !strings.HasPrefix(arg, "--") {
			remaining = append(remaining, args[i:]...)
			break
		}

		key := strings.TrimPrefix(arg, "--")
		var value string
		hasEquals := false

		if idx := strings.IndexByte(key, '='); idx >= 0 {
			value = key[idx+1:]
			key = key[:idx]
			hasEquals = true
		}

		p, ok := paramMap[key]
		if !ok {
			remaining = append(remaining, args[i:]...)
			break
		}

		switch p.Type {
		case Bool:
			if hasEquals {
				vals[key] = value != "false"
			} else {
				vals[key] = true
			}
		case Int:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: invalid integer %q", key, value)
			}
			vals[key] = n
		case Float:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, fmt.Errorf("flag --%s: invalid number %q", key, value)
			}
			vals[key] = f
		case Array:
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			arr, _ := vals[key].([]string)
			vals[key] = append(arr, value)
		default: // String
			if !hasEquals {
				i++
				if i >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", key)
				}
				value = args[i]
			}
			vals[key] = value
		}
	}

	return remaining, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestRunCLI -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `nix develop --command go test ./libs/go-mcp/command/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add libs/go-mcp/command/cli.go libs/go-mcp/command/cli_test.go
git commit -m "feat(command): add CLI flag parser and App.RunCLI"
```

---

### Task 4: Update GeneratePlugin to use MCP binary name

**Files:**
- Modify: `libs/go-mcp/command/app.go`
- Modify: `libs/go-mcp/command/generate_plugin.go`
- Modify: `libs/go-mcp/command/generate_plugin_test.go`

**Step 1: Write failing test**

Add to `generate_plugin_test.go`:

```go
func TestGeneratePluginUsesMCPBinary(t *testing.T) {
	app := NewApp("sweatshop", "Worktree manager")
	app.MCPBinary = "sweatshop-mcp"

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "sweatshop", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["sweatshop"].(map[string]any)
	if srv["command"] != "sweatshop-mcp" {
		t.Errorf("command = %v, want sweatshop-mcp", srv["command"])
	}
}

func TestGeneratePluginDefaultsMCPBinaryToName(t *testing.T) {
	app := NewApp("grit", "Git operations")

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "grit", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["grit"].(map[string]any)
	if srv["command"] != "grit" {
		t.Errorf("command = %v, want grit (default)", srv["command"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestGeneratePluginUsesMCPBinary -v`
Expected: FAIL — `MCPBinary` field doesn't exist

**Step 3: Add MCPBinary field to App and update GeneratePlugin**

In `app.go`, add field to `App`:

```go
type App struct {
	Name        string
	Description Description
	Version     string
	MCPArgs     []string
	MCPBinary   string  // binary name for plugin.json command; defaults to Name
	Params      []Param
	commands    map[string]*Command
}
```

In `generate_plugin.go`, use `MCPBinary` if set:

```go
func (a *App) GeneratePlugin(dir string) error {
	cmdName := a.Name
	if a.MCPBinary != "" {
		cmdName = a.MCPBinary
	}

	manifest := pluginManifest{
		Name: a.Name,
		McpServers: map[string]pluginMcpServer{
			a.Name: {
				Type:    "stdio",
				Command: cmdName,
				Args:    a.MCPArgs,
			},
		},
	}

	pluginDir := filepath.Join(dir, a.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)
}
```

**Step 4: Run tests**

Run: `nix develop --command go test ./libs/go-mcp/command/ -run TestGeneratePlugin -v`
Expected: PASS

**Step 5: Run full suite**

Run: `nix develop --command go test ./libs/go-mcp/command/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add libs/go-mcp/command/app.go libs/go-mcp/command/generate_plugin.go \
       libs/go-mcp/command/generate_plugin_test.go
git commit -m "feat(command): add MCPBinary field for separate MCP binary names"
```

---

### Task 5: Add HuhPrompter in sub-package

**Files:**
- Create: `libs/go-mcp/command/huh/prompter.go`
- Create: `libs/go-mcp/command/huh/prompter_test.go`

**Step 1: Write the test**

```go
// libs/go-mcp/command/huh/prompter_test.go
package huh

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
)

func TestHuhPrompterImplementsPrompter(t *testing.T) {
	var _ command.Prompter = &Prompter{}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./libs/go-mcp/command/huh/ -v`
Expected: FAIL — package and type don't exist

**Step 3: Implement HuhPrompter**

```go
// libs/go-mcp/command/huh/prompter.go
package huh

import (
	"github.com/charmbracelet/huh"
)

// Prompter wraps charmbracelet/huh for interactive CLI prompts.
type Prompter struct{}

func (Prompter) Confirm(msg string) (bool, error) {
	var result bool
	err := huh.NewConfirm().
		Title(msg).
		Value(&result).
		Run()
	return result, err
}

func (Prompter) Select(msg string, options []string) (int, error) {
	var result int
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	err := huh.NewSelect[int]().
		Title(msg).
		Options(opts...).
		Value(&result).
		Run()
	return result, err
}

func (Prompter) Input(msg string) (string, error) {
	var result string
	err := huh.NewInput().
		Title(msg).
		Value(&result).
		Run()
	return result, err
}
```

**Step 4: Run test**

Run: `nix develop --command go test ./libs/go-mcp/command/huh/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add libs/go-mcp/command/huh/prompter.go libs/go-mcp/command/huh/prompter_test.go
git commit -m "feat(command/huh): add HuhPrompter wrapping charmbracelet/huh"
```

---

## Phase 2: sweatshop migration

All tasks in this phase are in the **sweatshop** repo (`/Users/sfriedenberg/eng/worktrees/sweatshop/purse/`).

### Task 6: Add purse-first dependency and shared buildApp

**Files:**
- Modify: `go.mod` (add purse-first dependency)
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`

**Step 1: Add purse-first dependency**

Run: `nix develop --command go get github.com/amarbel-llc/purse-first/libs/go-mcp@latest`

Then run: `nix develop --command go mod tidy`

**Step 2: Write failing test for buildApp**

```go
// internal/app/app_test.go
package app

import "testing"

func TestBuildAppHasExpectedCommands(t *testing.T) {
	app := BuildApp()

	expected := []string{"status", "merge", "clean", "open", "completions"}
	for _, name := range expected {
		if _, ok := app.GetCommand(name); !ok {
			t.Errorf("missing command: %s", name)
		}
	}
}

func TestBuildAppHasGlobalFormatParam(t *testing.T) {
	app := BuildApp()
	if len(app.Params) == 0 {
		t.Fatal("expected global params")
	}
	if app.Params[0].Name != "format" {
		t.Errorf("first global param = %q, want %q", app.Params[0].Name, "format")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `nix develop --command go test ./internal/app/ -v`
Expected: FAIL — package doesn't exist

**Step 4: Create buildApp with just the command registrations (no handlers yet)**

```go
// internal/app/app.go
package app

import "github.com/amarbel-llc/purse-first/libs/go-mcp/command"

func BuildApp() *command.App {
	app := command.NewApp("sweatshop", "Shell-agnostic git worktree session manager")
	app.Version = "0.1.0"
	app.MCPBinary = "sweatshop-mcp"
	app.Params = []command.Param{
		{Name: "format", Type: command.String, Description: "output format: tap or table"},
	}

	registerStatusCommand(app)
	registerMergeCommand(app)
	registerCleanCommand(app)
	registerOpenCommand(app)
	registerCompletionsCommand(app)

	return app
}
```

Stub out the register functions in the same file or separate files (one per command). Each registers a `command.Command` with `Run` or `RunCLI` that delegates to the existing `internal/status`, `internal/merge`, etc. packages.

This is a large step — implement each `register*Command` function one at a time, wiring the existing internal package functions into the new `Run`/`RunCLI` handlers. Follow the command mapping from the design doc:

- `status` → `Run` (calls `status.CollectStatus`, returns `Result`)
- `merge` → `Run` (calls `merge.Run`, returns `Result`)
- `clean` → `Run` with Prompter (calls `clean.Run`, passes Prompter)
- `open` → `RunCLI` (calls `shop.OpenExisting`/`OpenNew`/`OpenRemote`)
- `completions` → `RunCLI` (calls `completions.Local`/`Remote`)

Note: `clean.Run` currently uses `huh.NewConfirm` directly. That needs to be refactored to accept a `Prompter` — do that in Task 7.

For this task, wire all commands except `clean` (which needs the Prompter refactor) and the `perms` sub-app.

**Step 5: Run test**

Run: `nix develop --command go test ./internal/app/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/app/
git commit -m "feat: add shared buildApp using command.App framework"
```

---

### Task 7: Refactor clean to accept Prompter

**Files:**
- Modify: `internal/clean/clean.go`
- Modify: `internal/clean/clean_test.go` (if exists)

**Step 1: Refactor clean.Run to accept a command.Prompter**

Change the `clean.Run` signature from:

```go
func Run(home string, interactive bool, format string) error
```

to:

```go
func Run(home string, interactive bool, format string, p command.Prompter) error
```

Replace the direct `huh.NewConfirm()` call with `p.Confirm(prompt)`.

**Step 2: Wire clean command in app.go with Prompter**

The `registerCleanCommand` sets `Run` and passes the Prompter through:

```go
func registerCleanCommand(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "clean",
		Description: command.Description{Short: "Remove merged worktrees"},
		Params: []command.Param{
			{Name: "interactive", Type: command.Bool, Description: "interactively handle dirty worktrees"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p command.Prompter) (*command.Result, error) {
			var params struct {
				Interactive bool   `json:"interactive"`
				Format      string `json:"format"`
			}
			json.Unmarshal(args, &params)

			home, _ := os.UserHomeDir()
			format := params.Format
			if format == "" {
				format = "tap"
			}

			err := clean.Run(home, params.Interactive, format, p)
			if err != nil {
				return command.TextErrorResult(err.Error()), nil
			}
			return command.TextResult("clean complete"), nil
		},
	})
}
```

**Step 3: Run tests**

Run: `nix develop --command go test ./internal/clean/ -v`
Run: `nix develop --command go test ./internal/app/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/clean/clean.go internal/app/
git commit -m "refactor(clean): accept Prompter instead of using huh directly"
```

---

### Task 8: Migrate perms sub-app

**Files:**
- Create: `internal/perms/app.go`
- Modify: `internal/app/app.go`

**Step 1: Create perms.BuildApp() returning a command.App**

Convert the existing `perms.NewPermsCmd()` (which returns a `*cobra.Command`) into `perms.BuildApp()` (which returns a `*command.App`):

- `perms-check` → `Run` (non-interactive)
- `perms-list` → `Run` (non-interactive)
- `perms-review` → `RunCLI` (interactive huh form)
- `perms-edit` → `RunCLI` (launches $EDITOR)

**Step 2: Merge into main app**

In `internal/app/app.go`, add:

```go
app.MergeWithPrefix(perms.BuildApp(), "perms")
```

**Step 3: Test**

Run: `nix develop --command go test ./internal/app/ -v`
Expected: PASS (update test to check for perms-check, perms-list, etc.)

**Step 4: Commit**

```bash
git add internal/perms/app.go internal/app/app.go internal/app/app_test.go
git commit -m "feat(perms): migrate to command.App with MergeWithPrefix"
```

---

### Task 9: Replace CLI main.go and add MCP main.go

**Files:**
- Modify: `cmd/sweatshop/main.go`
- Create: `cmd/sweatshop-mcp/main.go`
- Modify: `cmd/spinclass/main.go` (alias binary)

**Step 1: Rewrite cmd/sweatshop/main.go**

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/amarbel-llc/sweatshop/internal/app"
	huhprompter "github.com/amarbel-llc/purse-first/libs/go-mcp/command/huh"
)

func main() {
	a := app.BuildApp()
	if err := a.RunCLI(context.Background(), os.Args[1:], huhprompter.Prompter{}); err != nil {
		fmt.Fprintf(os.Stderr, "sweatshop: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Create cmd/sweatshop-mcp/main.go**

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/amarbel-llc/sweatshop/internal/app"
)

func main() {
	a := app.BuildApp()

	t := transport.NewStdio(os.Stdin, os.Stdout)
	registry := server.NewToolRegistry()
	a.RegisterMCPTools(registry)

	srv, err := server.New(t, server.Options{
		ServerName:    a.Name,
		ServerVersion: a.Version,
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

**Step 3: Update cmd/spinclass/main.go similarly** (alias binary)

Same as sweatshop main but with `a.Name = "spinclass"` or just reuse the same binary with the `sweatshop` name.

**Step 4: Remove cobra dependency**

Run: `nix develop --command go mod tidy`

Verify `github.com/spf13/cobra` is no longer in `go.mod`.

**Step 5: Build and smoke test**

Run: `nix develop --command go build ./cmd/sweatshop/`
Run: `nix develop --command go build ./cmd/sweatshop-mcp/`
Run: `./sweatshop status`
Run: `./sweatshop --help`

**Step 6: Commit**

```bash
git add cmd/sweatshop/main.go cmd/sweatshop-mcp/main.go cmd/spinclass/main.go go.mod go.sum
git commit -m "feat: migrate CLI to command.App, add sweatshop-mcp binary"
```

---

### Task 10: Update flake.nix for dual binary build

**Files:**
- Modify: `flake.nix`

**Step 1: Update flake.nix**

Add the `sweatshop-mcp` binary to the build. Update the `buildGoApplication` or post-build step to run `sweatshop generate-plugin $out` for artifact generation (plugin.json, manpages, completions).

Exact changes depend on the current flake.nix structure — read it, then:
- Ensure both `cmd/sweatshop` and `cmd/sweatshop-mcp` are built
- Add a post-build phase that runs artifact generation
- Install shell completions and manpages from the generated output

**Step 2: Build with nix**

Run: `just build` or `nix build --show-trace`
Expected: builds successfully, `result/bin/sweatshop` and `result/bin/sweatshop-mcp` both exist

**Step 3: Verify artifacts**

Run: `ls result/share/purse-first/sweatshop/`
Expected: `plugin.json` (and `mappings.json` if any MapsBash defined)

Run: `ls result/share/man/man1/`
Expected: manpages for sweatshop and subcommands

**Step 4: Commit**

```bash
git add flake.nix
git commit -m "feat(nix): build both sweatshop and sweatshop-mcp, generate plugin artifacts"
```

---

### Task 11: Run full test suite and clean up

**Step 1: Run Go tests**

Run: `just test`
Expected: PASS

**Step 2: Run bats integration tests**

Run: `just test-bats`
Expected: PASS (may need updates if tests reference cobra-specific behavior)

**Step 3: Run nix flake check**

Run: `nix flake check`
Expected: PASS

**Step 4: Clean up any remaining cobra references**

Search for leftover cobra imports or references:
Run: `grep -r "cobra" --include="*.go" .`
Expected: no results

**Step 5: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: remove remaining cobra references"
```
