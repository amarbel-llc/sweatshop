#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path
}

create_repo_with_commit() {
  local repo_path="$1"
  mkdir -p "$repo_path"
  git -C "$repo_path" init -q
  git -C "$repo_path" commit --allow-empty -m "init" -q
}

create_merged_worktree() {
  local repo_path="$1"
  local branch="$2"
  local worktree_path="$3"

  mkdir -p "$(dirname "$worktree_path")"
  git -C "$repo_path" worktree add -q "$worktree_path" -b "$branch"
  # Make a commit so the branch exists, then merge it back
  git -C "$worktree_path" commit --allow-empty -m "worktree commit" -q
  local main_branch
  main_branch=$(git -C "$repo_path" branch --show-current)
  git -C "$repo_path" merge "$branch" --ff-only -q
}

create_unmerged_worktree() {
  local repo_path="$1"
  local branch="$2"
  local worktree_path="$3"

  mkdir -p "$(dirname "$worktree_path")"
  git -C "$repo_path" worktree add -q "$worktree_path" -b "$branch"
  # Make a commit that is NOT merged back
  git -C "$worktree_path" commit --allow-empty -m "unmerged commit" -q
}

function clean_removes_merged_clean_worktrees { # @test
  create_repo_with_commit "$HOME/eng/repos/myrepo"
  create_merged_worktree "$HOME/eng/repos/myrepo" "done-branch" "$HOME/eng/worktrees/myrepo/done-branch"

  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"TAP version 14"* ]]
  [[ "$output" == *"ok"*"remove eng/worktrees/myrepo/"*"done-branch"* ]]
  [[ ! -d "$HOME/eng/worktrees/myrepo/done-branch" ]]
}

function clean_skips_unmerged_worktrees { # @test
  create_repo_with_commit "$HOME/eng/repos/myrepo"
  create_unmerged_worktree "$HOME/eng/repos/myrepo" "wip-branch" "$HOME/eng/worktrees/myrepo/wip-branch"

  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" != *"remove"*"wip-branch"* ]]
  [[ -d "$HOME/eng/worktrees/myrepo/wip-branch" ]]
}

function clean_skips_dirty_worktrees_without_interactive { # @test
  create_repo_with_commit "$HOME/eng/repos/myrepo"
  create_merged_worktree "$HOME/eng/repos/myrepo" "dirty-branch" "$HOME/eng/worktrees/myrepo/dirty-branch"
  echo "uncommitted" >"$HOME/eng/worktrees/myrepo/dirty-branch/dirty.txt"

  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"# SKIP dirty worktree"* ]]
  [[ -d "$HOME/eng/worktrees/myrepo/dirty-branch" ]]
}

function clean_reports_plan_line { # @test
  create_repo_with_commit "$HOME/eng/repos/myrepo"
  create_merged_worktree "$HOME/eng/repos/myrepo" "merged-a" "$HOME/eng/worktrees/myrepo/merged-a"
  create_merged_worktree "$HOME/eng/repos/myrepo" "merged-b" "$HOME/eng/worktrees/myrepo/merged-b"

  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"1..2"* ]]
}

function clean_shows_message_when_no_worktrees { # @test
  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"# SKIP no worktrees found"* ]]
}

function clean_handles_mixed_merged_and_unmerged { # @test
  create_repo_with_commit "$HOME/eng/repos/myrepo"
  create_merged_worktree "$HOME/eng/repos/myrepo" "merged" "$HOME/eng/worktrees/myrepo/merged"
  create_unmerged_worktree "$HOME/eng/repos/myrepo" "unmerged" "$HOME/eng/worktrees/myrepo/unmerged"

  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"1..1"* ]]
  [[ ! -d "$HOME/eng/worktrees/myrepo/merged" ]]
  [[ -d "$HOME/eng/worktrees/myrepo/unmerged" ]]
}

function clean_works_across_eng_areas { # @test
  create_repo_with_commit "$HOME/eng/repos/repo-a"
  create_repo_with_commit "$HOME/eng2/repos/repo-b"
  create_merged_worktree "$HOME/eng/repos/repo-a" "done-a" "$HOME/eng/worktrees/repo-a/done-a"
  create_merged_worktree "$HOME/eng2/repos/repo-b" "done-b" "$HOME/eng2/worktrees/repo-b/done-b"

  run sweatshop clean
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"1..2"* ]]
  [[ ! -d "$HOME/eng/worktrees/repo-a/done-a" ]]
  [[ ! -d "$HOME/eng2/worktrees/repo-b/done-b" ]]
}

function clean_table_format_uses_log_output { # @test
  create_repo_with_commit "$HOME/eng/repos/myrepo"
  create_merged_worktree "$HOME/eng/repos/myrepo" "done-branch" "$HOME/eng/worktrees/myrepo/done-branch"

  run sweatshop clean --format table
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"removed"* ]]
  [[ "$output" == *"done-branch"* ]]
  [[ "$output" != *"TAP version"* ]]
}
