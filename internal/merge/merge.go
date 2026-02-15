package merge

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

func Run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Strip home prefix to get relative path
	if len(cwd) <= len(home)+1 {
		return fmt.Errorf("not in a subdirectory of home: %s", cwd)
	}
	currentPath := cwd[len(home)+1:]

	comp, err := worktree.ParsePath(currentPath)
	if err != nil {
		return fmt.Errorf("not in a worktree directory: %s", cwd)
	}

	repoPath := worktree.RepoPath(home, comp)

	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository not found: %s", repoPath)
	}

	log.Info("merging worktree", "worktree", comp.Worktree)

	if err := git.RunPassthrough(repoPath, "merge", "--no-ff", comp.Worktree, "-m", "Merge worktree: "+comp.Worktree); err != nil {
		log.Error("merge failed, not removing worktree")
		return err
	}

	worktreePath := worktree.WorktreePath(home, currentPath)
	log.Info("removing worktree", "path", worktreePath)
	if err := git.RunPassthrough(repoPath, "worktree", "remove", worktreePath); err != nil {
		return err
	}

	log.Info("detaching from zmx session")
	cmd := exec.Command("zmx", "detach")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
