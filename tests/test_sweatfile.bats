#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path

  # Mock zmx to exit immediately
  cat > "$MOCK_BIN/zmx" <<'MOCKEOF'
#!/bin/bash
exit 0
MOCKEOF
  chmod +x "$MOCK_BIN/zmx"

  # Git needs user config in the isolated HOME
  git config --global user.name "Test User"
  git config --global user.email "test@example.com"
  git config --global init.defaultBranch main

  # Create a real git repo as the "main repo"
  mkdir -p "$HOME/eng/repos/testrepo"
  git init -q "$HOME/eng/repos/testrepo"
  git -C "$HOME/eng/repos/testrepo" commit --allow-empty -m "init" -q

  # Create eng-area sweatfile.
  # Note: git_excludes must include all generated dotfiles so the worktree
  # stays clean after creation — otherwise CloseShop triggers interactive
  # prompts that block in a non-TTY test environment.
  cat > "$HOME/eng/sweatfile" <<'EOF'
git_excludes = [".claude/", ".direnv/", ".envrc", ".sweatshop-env.snapshot"]

[env]
EDITOR = "nvim"

[files.envrc]
content = "use flake ."

setup = []
EOF
}

function sweatfile_creates_dotfiles_from_eng_area_sweatfile { # @test
  run sweatshop open "eng/worktrees/testrepo/feature-x" --format tap
  [[ "$status" -eq 0 ]]
  [[ -f "$HOME/eng/worktrees/testrepo/feature-x/.envrc" ]]
  [[ "$(cat "$HOME/eng/worktrees/testrepo/feature-x/.envrc")" == "use flake ." ]]
}

function sweatfile_writes_sweatshop_env_from_env_section { # @test
  run sweatshop open "eng/worktrees/testrepo/feature-env" --format tap
  [[ "$status" -eq 0 ]]
  [[ -f "$HOME/eng/worktrees/testrepo/feature-env/.sweatshop-env" ]]
  grep -q "EDITOR=nvim" "$HOME/eng/worktrees/testrepo/feature-env/.sweatshop-env"
}

function sweatfile_applies_git_excludes { # @test
  run sweatshop open "eng/worktrees/testrepo/feature-exc" --format tap
  [[ "$status" -eq 0 ]]
  local wt="$HOME/eng/worktrees/testrepo/feature-exc"
  local exclude_path
  exclude_path="$(git -C "$wt" rev-parse --git-path info/exclude)"
  if [[ ! "$exclude_path" = /* ]]; then
    exclude_path="$wt/$exclude_path"
  fi
  grep -q ".claude/" "$exclude_path"
  grep -q ".direnv/" "$exclude_path"
  grep -q ".envrc" "$exclude_path"
  # .sweatshop-env is always excluded (hardcoded)
  grep -q ".sweatshop-env" "$exclude_path"
}

function sweatfile_repo_sweatfile_merges_with_eng_area { # @test
  # Add repo-level sweatfile with additional env var
  cat > "$HOME/eng/repos/testrepo/sweatfile" <<'EOF'
[env]
PAGER = "less"
EOF

  run sweatshop open "eng/worktrees/testrepo/feature-merge" --format tap
  [[ "$status" -eq 0 ]]
  local env_file="$HOME/eng/worktrees/testrepo/feature-merge/.sweatshop-env"
  # Should have both EDITOR (inherited from eng-area) and PAGER (from repo)
  grep -q "EDITOR=nvim" "$env_file"
  grep -q "PAGER=less" "$env_file"
}

function sweatfile_repo_sweatfile_overrides_eng_area_file_entries { # @test
  cat > "$HOME/eng/repos/testrepo/sweatfile" <<'EOF'
[files.envrc]
content = "custom envrc"
EOF

  run sweatshop open "eng/worktrees/testrepo/feature-override" --format tap
  [[ "$status" -eq 0 ]]
  local envrc="$HOME/eng/worktrees/testrepo/feature-override/.envrc"
  [[ "$(cat "$envrc")" == "custom envrc" ]]
}

function no_attach_creates_worktree_without_calling_zmx { # @test
  # Make zmx create a marker file so we can detect if it ran
  cat > "$MOCK_BIN/zmx" <<'MOCKEOF'
#!/bin/bash
touch "$HOME/.zmx-was-called"
exit 0
MOCKEOF
  chmod +x "$MOCK_BIN/zmx"

  run sweatshop open "eng/worktrees/testrepo/feature-noattach" --no-attach --format tap
  [[ "$status" -eq 0 ]]
  # Worktree should be created and sweatfile applied
  [[ -d "$HOME/eng/worktrees/testrepo/feature-noattach" ]]
  [[ -f "$HOME/eng/worktrees/testrepo/feature-noattach/.envrc" ]]
  # zmx should NOT have been called
  [[ ! -f "$HOME/.zmx-was-called" ]]
}

function sweatfile_empty_sections_produce_no_dotfiles_or_env { # @test
  # Replace eng-area sweatfile with one that has only git_excludes
  # (no files, no env). This verifies graceful fallback when those
  # sections are absent.
  cat > "$HOME/eng/sweatfile" <<'EOF'
git_excludes = [".claude/", ".sweatshop-env.snapshot"]
EOF

  run sweatshop open "eng/worktrees/testrepo/feature-empty" --format tap
  [[ "$status" -eq 0 ]]
  # Worktree should still be created
  [[ -d "$HOME/eng/worktrees/testrepo/feature-empty" ]]
  # .sweatshop-env should NOT exist (no env vars defined)
  [[ ! -f "$HOME/eng/worktrees/testrepo/feature-empty/.sweatshop-env" ]]
  # .envrc should NOT exist (no files defined)
  [[ ! -f "$HOME/eng/worktrees/testrepo/feature-empty/.envrc" ]]
}
