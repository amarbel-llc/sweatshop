package attach

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

func Remote(host, path string) error {
	log.Info("connecting to remote session", "host", host, "path", path)
	cmd := exec.Command("ssh", "-t", host, "zmx attach "+path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func Existing(sweatshopPath string) error {
	cmd := exec.Command("zmx", "attach", sweatshopPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run() // zmx returns non-zero on detach

	return PostZmx(sweatshopPath)
}

func ToPath(sweatshopPath string) error {
	comp, err := worktree.ParsePath(sweatshopPath)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	repoPath := worktree.RepoPath(home, comp)
	worktreePath := worktree.WorktreePath(home, sweatshopPath)

	if err := worktree.Create(comp.EngArea, repoPath, worktreePath); err != nil {
		return err
	}

	if err := os.Chdir(worktreePath); err != nil {
		return fmt.Errorf("changing to worktree: %w", err)
	}

	cmd := exec.Command("zmx", "attach", sweatshopPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()

	return PostZmx(sweatshopPath)
}

func PostZmx(sweatshopPath string) error {
	comp, err := worktree.ParsePath(sweatshopPath)
	if err != nil {
		return nil // not a worktree path, nothing to do
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	repoPath := worktree.RepoPath(home, comp)
	worktreePath := worktree.WorktreePath(home, sweatshopPath)

	defaultBranch, err := git.BranchCurrent(repoPath)
	if err != nil || defaultBranch == "" {
		log.Warn("could not determine default branch")
		return nil
	}

	commitsAhead := git.CommitsAhead(worktreePath, defaultBranch, comp.Worktree)
	worktreeStatus := git.StatusPorcelain(worktreePath)

	if commitsAhead == 0 && worktreeStatus == "" {
		log.Info("no changes in worktree", "worktree", comp.Worktree)
		return nil
	}

	hasUncommitted := worktreeStatus != ""
	if hasUncommitted {
		log.Warn("worktree has uncommitted changes", "worktree", comp.Worktree)
		cmd := exec.Command("git", "-C", worktreePath, "status", "--short")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

	action, err := chooseAction(comp.Worktree, hasUncommitted)
	if err != nil || action == "" {
		return nil
	}

	return executeAction(action, repoPath, worktreePath, sweatshopPath, defaultBranch, comp.Worktree)
}

func chooseAction(worktreeName string, hasUncommitted bool) (string, error) {
	var options []huh.Option[string]
	var header string

	if hasUncommitted {
		header = fmt.Sprintf("Post-zmx actions for %s (uncommitted changes, will not remove worktree):", worktreeName)
		options = []huh.Option[string]{
			huh.NewOption("Pull + Rebase + Merge + Push", "Pull + Rebase + Merge + Push"),
			huh.NewOption("Rebase + Merge + Push", "Rebase + Merge + Push"),
			huh.NewOption("Rebase + Merge", "Rebase + Merge"),
			huh.NewOption("Rebase", "Rebase"),
		}
	} else {
		header = fmt.Sprintf("Post-zmx actions for %s:", worktreeName)
		options = []huh.Option[string]{
			huh.NewOption("Pull + Rebase + Merge + Remove worktree + Push", "Pull + Rebase + Merge + Remove worktree + Push"),
			huh.NewOption("Rebase + Merge + Remove worktree + Push", "Rebase + Merge + Remove worktree + Push"),
			huh.NewOption("Rebase + Merge + Remove worktree", "Rebase + Merge + Remove worktree"),
			huh.NewOption("Rebase + Merge", "Rebase + Merge"),
			huh.NewOption("Rebase", "Rebase"),
		}
	}

	var action string
	err := huh.NewSelect[string]().
		Title(header).
		Options(options...).
		Value(&action).
		Run()
	if err != nil {
		return "", nil
	}

	return action, nil
}

func executeAction(action, repoPath, worktreePath, sweatshopPath, defaultBranch, worktreeName string) error {
	home, _ := os.UserHomeDir()

	// Pull
	if len(action) >= 4 && action[:4] == "Pull" {
		if err := git.RunPassthrough(repoPath, "pull"); err != nil {
			log.Error("pull failed")
			return err
		}
		log.Info("pulled from origin", "branch", defaultBranch)
	}

	// Rebase
	if err := git.RunPassthrough(worktreePath, "rebase", defaultBranch); err != nil {
		log.Error("rebase failed")
		return err
	}
	log.Info("rebased onto default branch", "worktree", worktreeName, "base", defaultBranch)

	if action == "Rebase" {
		return nil
	}

	// Stash repo changes before merge
	repoStashed := false
	if git.HasDirtyTracked(repoPath) {
		if err := git.RunPassthrough(repoPath, "stash", "push", "-m", "sweatshop: auto-stash before merge of "+worktreeName); err == nil {
			repoStashed = true
			log.Info("stashed changes", "path", repoPath)
		}
	}

	// Merge
	if err := git.RunPassthrough(repoPath, "merge", worktreeName, "--ff-only"); err != nil {
		log.Error("merge failed (not fast-forward)")
		if repoStashed {
			git.RunPassthrough(repoPath, "stash", "pop")
			log.Info("restored stashed changes", "path", repoPath)
		}
		return err
	}
	log.Info("merged into default branch", "worktree", worktreeName, "base", defaultBranch)

	// Restore stash
	if repoStashed {
		git.RunPassthrough(repoPath, "stash", "pop")
		log.Info("restored stashed changes", "path", repoPath)
	}

	if action == "Rebase + Merge" {
		return nil
	}

	// Remove worktree
	if containsRemoveWorktree(action) {
		fullPath := filepath.Join(home, sweatshopPath)
		if err := git.RunPassthrough(repoPath, "worktree", "remove", fullPath); err != nil {
			log.Error("failed to remove worktree")
			return err
		}
		log.Info("removed worktree", "worktree", worktreeName)

		if err := git.RunPassthrough(repoPath, "branch", "-D", worktreeName); err != nil {
			log.Error("failed to delete branch", "branch", worktreeName)
			return err
		}
		log.Info("deleted branch", "branch", worktreeName)
	}

	// Push
	if len(action) >= 4 && action[len(action)-4:] == "Push" {
		if err := git.RunPassthrough(repoPath, "push", "origin", defaultBranch); err != nil {
			log.Error("push failed")
			return err
		}
		log.Info("pushed to origin", "branch", defaultBranch)
	}

	return nil
}

func containsRemoveWorktree(action string) bool {
	return len(action) > 15 && (action == "Rebase + Merge + Remove worktree" ||
		action == "Rebase + Merge + Remove worktree + Push" ||
		action == "Pull + Rebase + Merge + Remove worktree + Push")
}
