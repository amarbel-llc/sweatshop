#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path

  # Mock gum to capture output without requiring it
  cat >"$MOCK_BIN/gum" <<'MOCK'
#!/bin/bash
if [[ "$1" == "log" ]]; then
  shift
  # Skip -t flag and its argument
  while [[ "$1" == -* ]]; do
    shift 2
  done
  echo "$*"
fi
MOCK
  chmod +x "$MOCK_BIN/gum"

  source "$BIN_DIR/z"
}

create_mock_repo() {
  local repo_path="$1"
  mkdir -p "$repo_path"
  git -C "$repo_path" init -q
  git -C "$repo_path" commit --allow-empty -m "init" -q
}

function status_discovers_repos_across_eng_areas { # @test
  create_mock_repo "$HOME/eng/repos/repo-a"
  create_mock_repo "$HOME/eng2/repos/repo-b"

  run z_status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"eng/repos/repo-a"* ]]
  [[ "$output" == *"eng2/repos/repo-b"* ]]
}

function status_handles_repos_with_no_worktrees { # @test
  create_mock_repo "$HOME/eng/repos/solo-repo"

  run z_status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"eng/repos/solo-repo"* ]]
  # Should show main branch info
  [[ "$output" == *"commit:"* ]]
}

function status_handles_repos_with_worktrees { # @test
  create_mock_repo "$HOME/eng/repos/myrepo"

  local worktree_path="$HOME/eng/worktrees/myrepo/feature-x"
  mkdir -p "$(dirname "$worktree_path")"
  git -C "$HOME/eng/repos/myrepo" worktree add -q "$worktree_path"

  run z_status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"eng/repos/myrepo"* ]]
  [[ "$output" == *"feature-x"* ]]
}

function status_shows_clean_for_clean_repo { # @test
  create_mock_repo "$HOME/eng/repos/clean-repo"

  run z_status_branch "$HOME/eng/repos/clean-repo" "main"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"clean"* ]]
}

function status_shows_dirty_count { # @test
  create_mock_repo "$HOME/eng/repos/dirty-repo"
  echo "change" >"$HOME/eng/repos/dirty-repo/file.txt"

  run z_status_branch "$HOME/eng/repos/dirty-repo" "main"
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"1?"* ]]
}

function status_shows_no_repos_message { # @test
  # Empty HOME, no eng* dirs
  run z_status
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"no repos found"* ]]
}

function status_skips_non_git_directories { # @test
  mkdir -p "$HOME/eng/repos/not-a-repo"

  run z_status
  [[ "$status" -eq 0 ]]
  # Should not show the non-git directory
  [[ "$output" != *"not-a-repo"* ]]
}
