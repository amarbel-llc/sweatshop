package merge

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/executor"
	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

func Run(exec executor.Executor) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	var repoPath, branch string

	// Try convention path first
	if len(cwd) > len(home)+1 {
		currentPath := cwd[len(home)+1:]
		if comp, parseErr := worktree.ParsePath(currentPath); parseErr == nil {
			repoPath = worktree.RepoPath(home, comp)
			branch = comp.Worktree
		}
	}

	// Fall back to git-based detection
	if repoPath == "" {
		repoPath, err = git.CommonDir(cwd)
		if err != nil {
			return fmt.Errorf("not in a worktree directory: %s", cwd)
		}
		branch, err = git.BranchCurrent(cwd)
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
	}

	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository not found: %s", repoPath)
	}

	log.Info("merging worktree", "worktree", branch)

	if err := git.RunPassthrough(repoPath, "merge", "--no-ff", branch, "-m", "Merge worktree: "+branch); err != nil {
		log.Error("merge failed, not removing worktree")
		return err
	}

	log.Info("removing worktree", "path", cwd)
	if err := git.RunPassthrough(repoPath, "worktree", "remove", cwd); err != nil {
		return err
	}

	log.Info("detaching from session")
	return exec.Detach()
}
