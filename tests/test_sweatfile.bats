#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path

  # Mock SHELL to exit immediately (ShellExecutor runs $SHELL when no command given)
  cat > "$MOCK_BIN/mock-shell" <<'MOCKEOF'
#!/bin/bash
exit 0
MOCKEOF
  chmod +x "$MOCK_BIN/mock-shell"
  export SHELL="$MOCK_BIN/mock-shell"

  # Create a real git repo as the "main repo"
  mkdir -p "$HOME/eng/repos/testrepo"
  git init -q "$HOME/eng/repos/testrepo"
  git -C "$HOME/eng/repos/testrepo" commit --allow-empty -m "init" -q

  # Create eng-area sweatfile with only git_excludes
  cat > "$HOME/eng/sweatfile" <<'EOF'
git_excludes = [".claude/", ".direnv/"]
EOF
}

function sweatfile_applies_git_excludes { # @test
  run sweatshop attach "eng/worktrees/testrepo/feature-exc" --format tap
  [[ "$status" -eq 0 ]]
  local wt="$HOME/eng/worktrees/testrepo/feature-exc"
  local exclude_path
  exclude_path="$(git -C "$wt" rev-parse --git-path info/exclude)"
  if [[ ! "$exclude_path" = /* ]]; then
    exclude_path="$wt/$exclude_path"
  fi
  grep -q ".claude/" "$exclude_path"
  grep -q ".direnv/" "$exclude_path"
}

function sweatfile_repo_sweatfile_merges_with_eng_area { # @test
  cat > "$HOME/eng/repos/testrepo/sweatfile" <<'EOF'
git_excludes = [".envrc"]
EOF

  run sweatshop attach "eng/worktrees/testrepo/feature-merge" --format tap
  [[ "$status" -eq 0 ]]
  local wt="$HOME/eng/worktrees/testrepo/feature-merge"
  local exclude_path
  exclude_path="$(git -C "$wt" rev-parse --git-path info/exclude)"
  if [[ ! "$exclude_path" = /* ]]; then
    exclude_path="$wt/$exclude_path"
  fi
  # Should have both eng-area and repo excludes
  grep -q ".claude/" "$exclude_path"
  grep -q ".direnv/" "$exclude_path"
  grep -q ".envrc" "$exclude_path"
}

function create_makes_worktree_without_running_shell { # @test
  # Make mock shell create a marker file so we can detect if it ran
  cat > "$MOCK_BIN/mock-shell" <<'MOCKEOF'
#!/bin/bash
touch "$HOME/.shell-was-called"
exit 0
MOCKEOF
  chmod +x "$MOCK_BIN/mock-shell"

  run sweatshop create "eng/worktrees/testrepo/feature-create"
  [[ "$status" -eq 0 ]]
  # Worktree should be created
  [[ -d "$HOME/eng/worktrees/testrepo/feature-create" ]]
  # Shell should NOT have been called
  [[ ! -f "$HOME/.shell-was-called" ]]

  # Claude settings should be generated
  local wt="$HOME/eng/worktrees/testrepo/feature-create"
  local settings="$wt/.claude/settings.local.json"
  [[ -f "$settings" ]]

  # defaultMode should be acceptEdits
  local mode
  mode="$(jq -r '.permissions.defaultMode' "$settings")"
  [[ "$mode" = "acceptEdits" ]]

  # Scoped Edit and Write rules should be present
  jq -e ".permissions.allow | map(select(startswith(\"Edit(\"))) | length > 0" "$settings" >/dev/null
  jq -e ".permissions.allow | map(select(startswith(\"Write(\"))) | length > 0" "$settings" >/dev/null
}

function create_with_repo_flag_uses_custom_repo_path { # @test
  # Create a repo in a non-standard location (not under eng/repos/)
  mkdir -p "$HOME/custom/location"
  git init -q "$HOME/custom/location/myrepo"
  git -C "$HOME/custom/location/myrepo" commit --allow-empty -m "custom init" -q

  run sweatshop create --repo "$HOME/custom/location/myrepo" "eng/worktrees/myrepo/feature-custom"
  [[ "$status" -eq 0 ]]

  # Worktree should be created
  local wt="$HOME/eng/worktrees/myrepo/feature-custom"
  [[ -d "$wt" ]]

  # Verify the worktree was created from the custom repo
  git -C "$wt" log --oneline | grep -q "custom init"
}

function sweatfile_empty_sections_produce_clean_worktree { # @test
  # Replace eng-area sweatfile with minimal content
  cat > "$HOME/eng/sweatfile" <<'EOF'
git_excludes = [".claude/"]
EOF

  run sweatshop attach "eng/worktrees/testrepo/feature-empty" --format tap
  [[ "$status" -eq 0 ]]
  # Worktree should still be created
  [[ -d "$HOME/eng/worktrees/testrepo/feature-empty" ]]
}

function create_with_arbitrary_absolute_path { # @test
  local wt="$BATS_TEST_TMPDIR/arbitrary-wt"
  run sweatshop create --repo "$HOME/eng/repos/testrepo" "$wt"
  [[ "$status" -eq 0 ]]
  # Worktree should be created at the arbitrary path
  [[ -d "$wt" ]]
  # Verify it was created from the correct repo
  git -C "$wt" log --oneline | grep -q "init"
}

function create_arbitrary_path_without_repo_fails { # @test
  local wt="$BATS_TEST_TMPDIR/no-repo-wt"
  run sweatshop create "$wt"
  [[ "$status" -ne 0 ]]
  [[ "$output" == *"--repo is required"* ]]
}

function attach_with_arbitrary_path { # @test
  local wt="$BATS_TEST_TMPDIR/arbitrary-attach"
  run sweatshop attach --repo "$HOME/eng/repos/testrepo" "$wt" --format tap
  [[ "$status" -eq 0 ]]
  # Worktree should be created
  [[ -d "$wt" ]]
}
