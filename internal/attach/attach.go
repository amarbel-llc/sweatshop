package attach

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/flake"
	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/tap"
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

func Existing(sweatshopPath, format string) error {
	cmd := exec.Command("zmx", "attach", sweatshopPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run() // zmx returns non-zero on detach

	return PostZmx(sweatshopPath, format)
}

func ToPath(sweatshopPath, format string) error {
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

	zmxArgs := []string{"attach", sweatshopPath}
	if flake.HasDevShell(worktreePath) {
		log.Info("flake.nix detected, starting session in nix develop")
		zmxArgs = append(zmxArgs, "nix", "develop", "--command", os.Getenv("SHELL"))
	}

	cmd := exec.Command("zmx", zmxArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	_ = cmd.Run()

	return PostZmx(sweatshopPath, format)
}

func PostZmx(sweatshopPath, format string) error {
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

	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(os.Stdout)
	}

	if commitsAhead == 0 && worktreeStatus == "" {
		if tw != nil {
			tw.PlanAhead(1)
			tw.Skip("post-zmx "+comp.Worktree, "no changes")
		} else {
			log.Info("no changes in worktree", "worktree", comp.Worktree)
		}
		return nil
	}

	hasUncommitted := worktreeStatus != ""
	if hasUncommitted {
		if tw == nil {
			log.Warn("worktree has uncommitted changes", "worktree", comp.Worktree)
			cmd := exec.Command("git", "-C", worktreePath, "status", "--short")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}

	action, err := chooseAction(comp.Worktree, hasUncommitted)
	if err != nil || action == "" {
		if tw != nil {
			tw.PlanAhead(0)
		}
		return nil
	}

	if action == "Abort" {
		return Existing(sweatshopPath, format)
	}

	if tw != nil {
		tw.PlanAhead(estimateSteps(action))
	}

	return executeAction(action, repoPath, worktreePath, sweatshopPath, defaultBranch, comp.Worktree, format, tw)
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
			huh.NewOption("Abort (reattach)", "Abort"),
		}
	} else {
		header = fmt.Sprintf("Post-zmx actions for %s:", worktreeName)
		options = []huh.Option[string]{
			huh.NewOption("Pull + Rebase + Merge + Remove worktree + Push", "Pull + Rebase + Merge + Remove worktree + Push"),
			huh.NewOption("Rebase + Merge + Remove worktree + Push", "Rebase + Merge + Remove worktree + Push"),
			huh.NewOption("Rebase + Merge + Remove worktree", "Rebase + Merge + Remove worktree"),
			huh.NewOption("Rebase + Merge", "Rebase + Merge"),
			huh.NewOption("Rebase", "Rebase"),
			huh.NewOption("Abort (reattach)", "Abort"),
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

func runGit(tw *tap.Writer, repoPath string, args ...string) error {
	if tw != nil {
		_, err := git.Run(repoPath, args...)
		return err
	}
	return git.RunPassthrough(repoPath, args...)
}

func tapStep(tw *tap.Writer, desc string, err error) error {
	if tw == nil {
		return err
	}
	if err != nil {
		tw.NotOk(desc, map[string]string{"error": err.Error()})
	} else {
		tw.Ok(desc)
	}
	return err
}

func executeAction(action, repoPath, worktreePath, sweatshopPath, defaultBranch, worktreeName, format string, tw *tap.Writer) error {
	home, _ := os.UserHomeDir()

	// Pull
	if len(action) >= 4 && action[:4] == "Pull" {
		err := runGit(tw, repoPath, "pull")
		if tapStep(tw, "pull "+defaultBranch, err) != nil {
			if tw == nil {
				log.Error("pull failed, reattaching to session to resolve")
			}
			return Existing(sweatshopPath, format)
		}
		if tw == nil {
			log.Info("pulled from origin", "branch", defaultBranch)
		}
	}

	// Rebase
	err := runGit(tw, worktreePath, "rebase", defaultBranch)
	if tapStep(tw, "rebase "+worktreeName+" onto "+defaultBranch, err) != nil {
		if tw == nil {
			log.Error("rebase failed, reattaching to session to resolve conflicts")
		}
		return Existing(sweatshopPath, format)
	}
	if tw == nil {
		log.Info("rebased onto default branch", "worktree", worktreeName, "base", defaultBranch)
	}

	if action == "Rebase" {
		return nil
	}

	// Stash repo changes before merge
	repoStashed := false
	if git.HasDirtyTracked(repoPath) {
		if err := runGit(tw, repoPath, "stash", "push", "-m", "sweatshop: auto-stash before merge of "+worktreeName); err == nil {
			repoStashed = true
			if tw == nil {
				log.Info("stashed changes", "path", repoPath)
			}
		}
	}

	// Merge
	mergeErr := runGit(tw, repoPath, "merge", worktreeName, "--ff-only")
	if tapStep(tw, "merge "+worktreeName+" into "+defaultBranch, mergeErr) != nil {
		if tw == nil {
			log.Error("merge failed (not fast-forward), reattaching to session to resolve")
		}
		if repoStashed {
			runGit(tw, repoPath, "stash", "pop")
			if tw == nil {
				log.Info("restored stashed changes", "path", repoPath)
			}
		}
		return Existing(sweatshopPath, format)
	}
	if tw == nil {
		log.Info("merged into default branch", "worktree", worktreeName, "base", defaultBranch)
	}

	// Restore stash
	if repoStashed {
		runGit(tw, repoPath, "stash", "pop")
		if tw == nil {
			log.Info("restored stashed changes", "path", repoPath)
		}
	}

	if action == "Rebase + Merge" {
		return nil
	}

	// Remove worktree
	if containsRemoveWorktree(action) {
		fullPath := filepath.Join(home, sweatshopPath)
		wtErr := runGit(tw, repoPath, "worktree", "remove", fullPath)
		if tapStep(tw, "remove worktree "+worktreeName, wtErr) != nil {
			if tw == nil {
				log.Error("failed to remove worktree")
			}
			return wtErr
		}
		if tw == nil {
			log.Info("removed worktree", "worktree", worktreeName)
		}

		brErr := runGit(tw, repoPath, "branch", "-D", worktreeName)
		if tapStep(tw, "delete branch "+worktreeName, brErr) != nil {
			if tw == nil {
				log.Error("failed to delete branch", "branch", worktreeName)
			}
			return brErr
		}
		if tw == nil {
			log.Info("deleted branch", "branch", worktreeName)
		}
	}

	// Push
	if len(action) >= 4 && action[len(action)-4:] == "Push" {
		pushErr := runGit(tw, repoPath, "push", "origin", defaultBranch)
		if tapStep(tw, "push "+defaultBranch, pushErr) != nil {
			if tw == nil {
				log.Error("push failed")
			}
			return pushErr
		}
		if tw == nil {
			log.Info("pushed to origin", "branch", defaultBranch)
		}
	}

	return nil
}

func containsRemoveWorktree(action string) bool {
	return len(action) > 15 && (action == "Rebase + Merge + Remove worktree" ||
		action == "Rebase + Merge + Remove worktree + Push" ||
		action == "Pull + Rebase + Merge + Remove worktree + Push")
}

func estimateSteps(action string) int {
	n := 1 // rebase is always present
	if strings.HasPrefix(action, "Pull") {
		n++
	}
	if action != "Rebase" {
		n++ // merge
	}
	if containsRemoveWorktree(action) {
		n += 2 // remove worktree + delete branch
	}
	if strings.HasSuffix(action, "Push") {
		n++
	}
	return n
}
