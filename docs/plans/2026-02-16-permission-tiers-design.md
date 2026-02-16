# Permission Tiers for Sweatshop

## Problem

Claude Code permission approvals accumulate as junk in `.claude/settings.local.json`. There's no distinction between global, repo-specific, and one-off approvals. Alert fatigue from repeated prompts leads to careless approvals.

## Solution

Three components: curated permission tier files, a PermissionRequest hook that auto-approves matches with visible banners, and a post-session review step that routes new approvals to the right tier.

## Architecture

```
~/.config/sweatshop/permissions/
  global.json              # safe for all repos
  repos/<repo>.json        # repo-specific permissions

PermissionRequest hook (sweatshop perms check):
  stdin → tool_name + tool_input + cwd
  loads global + repo tiers based on cwd
  match → {"decision":"allow"} + systemMessage banner
  no match → empty output (falls through to normal prompt)

Post-session review (in PostZmx flow):
  diffs settings.local.json against pre-session snapshot
  huh prompt per new entry: promote global / promote repo / keep worktree / discard
  regenerates settings.local.json without tier-covered entries
```

## Permission Tier File Format

```json
{
  "allow": [
    "Bash(go test:*)",
    "mcp__plugin_lux_lux__hover"
  ]
}
```

Same syntax as Claude's native permission rules.

## New Subcommands

- `sweatshop perms check` — PermissionRequest hook handler
- `sweatshop perms review <path>` — post-session review (called from PostZmx)
- `sweatshop perms list [--repo <repo>]` — show curated tiers
- `sweatshop perms edit [--global | --repo <repo>]` — open tier in $EDITOR

## Hook Registration

```json
{
  "hooks": {
    "PermissionRequest": [{
      "matcher": ".*",
      "hooks": [{
        "type": "command",
        "command": "sweatshop perms check",
        "timeout": 5
      }]
    }]
  }
}
```

## Permission Matching

The hook constructs the permission string Claude would use from `tool_name` + `tool_input`, then checks against tier rules using the same wildcard syntax Claude uses (e.g. `Bash(go test:*)` matches `Bash(go test ./...)`).

## systemMessage Visibility

Auto-approvals emit a systemMessage banner visible in Claude's UI:

```
[sweatshop] auto-approved: Bash(go test ./...) (global tier)
```

## Post-Session Review Flow

1. Before zmx attach: snapshot `.claude/settings.local.json`
2. After detach: diff against snapshot
3. Per new entry: huh select → promote global / promote repo / keep / discard
4. Regenerate settings.local.json without tier-covered entries
