package shop

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/executor"
	"github.com/amarbel-llc/sweatshop/internal/flake"
	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/tap"
	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

func OpenRemote(host, path string) error {
	log.Info("opening remote shop", "host", host, "path", path)
	cmd := exec.Command("ssh", "-t", host, "zmx attach "+path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func OpenExisting(exec executor.Executor, sweatshopPath, format string, noAttach bool, claudeArgs []string) error {
	if noAttach {
		return nil
	}

	comp, err := worktree.ParsePath(sweatshopPath)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	worktreePath := worktree.WorktreePath(home, sweatshopPath)

	var command []string
	if len(claudeArgs) > 0 {
		command = append([]string{"claude"}, claudeArgs...)
	}

	if err := exec.Attach(worktreePath, comp.ShopKey(), command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return CloseShop(sweatshopPath, format)
}

func OpenNew(exec executor.Executor, sweatshopPath, format string, noAttach bool, claudeArgs []string) error {
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

	if noAttach {
		return nil
	}

	var command []string
	if len(claudeArgs) > 0 {
		if flake.HasDevShell(worktreePath) {
			log.Info("flake.nix detected, starting claude in nix develop")
			command = append([]string{"nix", "develop", "--command", "claude"}, claudeArgs...)
		} else {
			command = append([]string{"claude"}, claudeArgs...)
		}
	} else if flake.HasDevShell(worktreePath) {
		log.Info("flake.nix detected, starting session in nix develop")
		command = []string{"nix", "develop", "--command", os.Getenv("SHELL")}
	}

	if err := exec.Attach(worktreePath, comp.ShopKey(), command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return CloseShop(sweatshopPath, format)
}

func CloseShop(sweatshopPath, format string) error {
	comp, err := worktree.ParsePath(sweatshopPath)
	if err != nil {
		return nil
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

	desc := statusDescription(defaultBranch, commitsAhead, worktreeStatus)

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.PlanAhead(1)
		tw.Ok("close " + comp.Worktree + " # " + desc)
	} else {
		log.Info(desc, "worktree", sweatshopPath)
	}

	return nil
}

func statusDescription(defaultBranch string, commitsAhead int, porcelain string) string {
	var parts []string

	if commitsAhead == 1 {
		parts = append(parts, fmt.Sprintf("1 commit ahead of %s", defaultBranch))
	} else {
		parts = append(parts, fmt.Sprintf("%d commits ahead of %s", commitsAhead, defaultBranch))
	}

	if porcelain == "" {
		parts = append(parts, "clean")
	} else {
		parts = append(parts, "dirty")
	}

	if commitsAhead == 0 && porcelain == "" {
		parts = append(parts, "(merged)")
	}

	return strings.Join(parts, ", ")
}
