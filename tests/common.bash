#!/bin/bash -e

if [[ -z $BATS_TEST_TMPDIR ]]; then
  echo 'common.bash loaded before $BATS_TEST_TMPDIR set. aborting.' >&2
  exit 1
fi

pushd "$BATS_TEST_TMPDIR" >/dev/null || exit 1

# Directory containing the Go binary under test
# When built with nix, the binary is in result/bin/
# When built with go build, it's in the project root
PROJECT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)"
if [[ -x "$PROJECT_DIR/result/bin/sweatshop" ]]; then
  BIN_DIR="$PROJECT_DIR/result/bin"
elif [[ -x "$PROJECT_DIR/sweatshop" ]]; then
  BIN_DIR="$PROJECT_DIR"
else
  echo "sweatshop binary not found. Run 'just build' or 'just build-go' first." >&2
  exit 1
fi

# Create a fake HOME for test isolation
setup_test_home() {
  export REAL_HOME="$HOME"
  export HOME="$BATS_TEST_TMPDIR/home"
  mkdir -p "$HOME"
}

# Create mock commands in a temp bin directory on PATH
setup_mock_path() {
  export MOCK_BIN="$BATS_TEST_TMPDIR/mock-bin"
  mkdir -p "$MOCK_BIN"
  export PATH="$BIN_DIR:$MOCK_BIN:$PATH"
}
