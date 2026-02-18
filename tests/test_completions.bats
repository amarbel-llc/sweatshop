#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
  setup_test_home
  setup_mock_path
}

function completions_lists_repos_as_new_worktree { # @test
  mkdir -p "$HOME/eng/repos/myrepo"

  run sweatshop completions
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"eng/worktrees/myrepo/"*"new worktree"* ]]
}

function completions_lists_existing_worktrees { # @test
  mkdir -p "$HOME/eng/repos/myrepo"
  mkdir -p "$HOME/eng/worktrees/myrepo/feature-x"
  echo "gitdir: $HOME/eng/repos/myrepo/.git/worktrees/feature-x" > "$HOME/eng/worktrees/myrepo/feature-x/.git"

  run sweatshop completions
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"eng/worktrees/myrepo/feature-x"*"existing worktree"* ]]
}

function completions_handles_multiple_eng_areas { # @test
  mkdir -p "$HOME/eng/repos/repo-a"
  mkdir -p "$HOME/eng2/repos/repo-b"

  run sweatshop completions
  [[ "$status" -eq 0 ]]
  [[ "$output" == *"eng/worktrees/repo-a/"* ]]
  [[ "$output" == *"eng2/worktrees/repo-b/"* ]]
}

function completions_output_is_tab_separated { # @test
  mkdir -p "$HOME/eng/repos/myrepo"

  run sweatshop completions
  [[ "$status" -eq 0 ]]
  local line
  line="$(echo "$output" | head -1)"
  [[ "$line" == *$'\t'* ]]
}

function completions_handles_no_repos { # @test
  run sweatshop completions
  [[ "$status" -eq 0 ]]
}
