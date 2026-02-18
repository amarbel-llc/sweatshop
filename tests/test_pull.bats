#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path
}

create_repo_with_remote() {
  local repo_path="$1"
  local bare_path="$2"

  # Create a bare repo as the "remote"
  mkdir -p "$bare_path"
  git -C "$bare_path" init -q --bare

  # Clone it to create the working repo
  git clone -q "$bare_path" "$repo_path"
  git -C "$repo_path" commit --allow-empty -m "init" -q
  git -C "$repo_path" push -q
}

push_remote_commit() {
  local bare_path="$1"
  local tmp_clone="$BATS_TEST_TMPDIR/tmp-clone-$$"

  git clone -q "$bare_path" "$tmp_clone"
  git -C "$tmp_clone" commit --allow-empty -m "remote update" -q
  git -C "$tmp_clone" push -q
  rm -rf "$tmp_clone"
}

create_worktree() {
  local repo_path="$1"
  local branch="$2"
  local worktree_path="$3"

  mkdir -p "$(dirname "$worktree_path")"
  git -C "$repo_path" worktree add -q "$worktree_path" -b "$branch"
  git -C "$worktree_path" commit --allow-empty -m "worktree commit" -q
}

function pull_pulls_clean_repos { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  push_remote_commit "$bare"

  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"TAP version 14"* ]]
  [[ "$output" == *"ok"*"pull eng/repos/myrepo"* ]]
}

function pull_skips_dirty_repos_without_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  echo "uncommitted" > "$HOME/eng/repos/myrepo/dirty.txt"

  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"# SKIP dirty"* ]]
}

function pull_includes_dirty_repos_with_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  echo "uncommitted" > "$HOME/eng/repos/myrepo/dirty.txt"

  run sweatshop pull --dirty
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"pull eng/repos/myrepo"* ]]
  [[ "$output" != *"# SKIP"* ]]
}

function pull_rebases_clean_worktrees { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"

  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"rebase eng/worktrees/myrepo/feature-x"* ]]
}

function pull_skips_dirty_worktrees_without_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"
  echo "uncommitted" > "$HOME/eng/worktrees/myrepo/feature-x/dirty.txt"

  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"rebase"*"# SKIP dirty"* ]]
}

function pull_includes_dirty_worktrees_with_flag { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"
  echo "uncommitted" > "$HOME/eng/worktrees/myrepo/feature-x/dirty.txt"

  run sweatshop pull -d
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"rebase eng/worktrees/myrepo/feature-x"* ]]
}

function pull_reports_plan_line { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  create_worktree "$HOME/eng/repos/myrepo" "feature-x" "$HOME/eng/worktrees/myrepo/feature-x"

  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"1..2"* ]]
}

function pull_shows_skip_when_no_repos { # @test
  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"# SKIP no repos found"* ]]
}

function pull_works_across_eng_areas { # @test
  local bare_a="$BATS_TEST_TMPDIR/bare/repo-a.git"
  local bare_b="$BATS_TEST_TMPDIR/bare/repo-b.git"
  create_repo_with_remote "$HOME/eng/repos/repo-a" "$bare_a"
  create_repo_with_remote "$HOME/eng2/repos/repo-b" "$bare_b"

  run sweatshop pull
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"pull eng/repos/repo-a"* ]]
  [[ "$output" == *"pull eng2/repos/repo-b"* ]]
  [[ "$output" == *"1..2"* ]]
}

function pull_short_flag_d_works { # @test
  local bare="$BATS_TEST_TMPDIR/bare/myrepo.git"
  create_repo_with_remote "$HOME/eng/repos/myrepo" "$bare"
  echo "uncommitted" > "$HOME/eng/repos/myrepo/dirty.txt"

  run sweatshop pull -d
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"ok"*"pull eng/repos/myrepo"* ]]
}
