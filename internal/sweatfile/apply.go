package sweatfile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amarbel-llc/sweatshop/internal/git"
)

// HardcodedExcludes are always written to .git/info/exclude regardless of sweatfile config.
var HardcodedExcludes = []string{
	".sweatshop-env",
}

func Apply(worktreePath string, sf Sweatfile) error {
	allExcludes := append(sf.GitExcludes, HardcodedExcludes...)
	if len(allExcludes) > 0 {
		excludePath, err := resolveExcludePath(worktreePath)
		if err != nil {
			return fmt.Errorf("resolving git exclude path: %w", err)
		}
		if err := applyGitExcludes(excludePath, allExcludes); err != nil {
			return fmt.Errorf("applying git excludes: %w", err)
		}
	}

	if err := ApplyFiles(worktreePath, sf.Files); err != nil {
		return fmt.Errorf("applying files: %w", err)
	}

	if err := ApplyEnv(worktreePath, sf.Env); err != nil {
		return fmt.Errorf("applying env: %w", err)
	}

	if err := RunSetup(worktreePath, sf.Setup); err != nil {
		return fmt.Errorf("running setup: %w", err)
	}

	return nil
}

func resolveExcludePath(worktreePath string) (string, error) {
	rel, err := git.Run(worktreePath, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(rel) {
		rel = filepath.Join(worktreePath, rel)
	}
	return rel, nil
}

func applyGitExcludes(excludePath string, patterns []string) error {
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range patterns {
		if _, err := fmt.Fprintln(f, p); err != nil {
			return err
		}
	}
	return nil
}

func ApplyFiles(worktreePath string, files map[string]FileEntry) error {
	for name, entry := range files {
		dest := filepath.Join(worktreePath, "."+name)

		if _, err := os.Stat(dest); err == nil {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		if entry.Source != "" {
			src := expandHome(entry.Source)
			if err := os.Symlink(src, dest); err != nil {
				return err
			}
		} else if entry.Content != "" {
			if err := os.WriteFile(dest, []byte(entry.Content), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func ApplyEnv(worktreePath string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	for _, k := range keys {
		v := env[k]
		if v == "" {
			continue
		}
		lines = append(lines, k+"="+v)
	}

	if len(lines) == 0 {
		return nil
	}

	path := filepath.Join(worktreePath, ".sweatshop-env")
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func RunSetup(worktreePath string, commands []string) error {
	for _, cmdStr := range commands {
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = worktreePath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setup command %q: %w", cmdStr, err)
		}
	}
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
