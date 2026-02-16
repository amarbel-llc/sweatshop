package sweatfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func SnapshotEnv(worktreePath string) error {
	envPath := filepath.Join(worktreePath, ".sweatshop-env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.WriteFile(filepath.Join(worktreePath, ".sweatshop-env.snapshot"), data, 0o644)
}

func DiffEnv(worktreePath string) (added, changed map[string]string, err error) {
	before := parseEnvFile(filepath.Join(worktreePath, ".sweatshop-env.snapshot"))
	after := parseEnvFile(filepath.Join(worktreePath, ".sweatshop-env"))

	added = make(map[string]string)
	changed = make(map[string]string)

	for k, v := range after {
		if oldV, ok := before[k]; !ok {
			added[k] = v
		} else if oldV != v {
			changed[k] = v
		}
	}
	return added, changed, nil
}

func CleanupEnvSnapshot(worktreePath string) {
	os.Remove(filepath.Join(worktreePath, ".sweatshop-env.snapshot"))
}

func parseEnvFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if idx := strings.IndexByte(line, '='); idx > 0 {
			m[line[:idx]] = line[idx+1:]
		}
	}
	return m
}
