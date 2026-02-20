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

func Create(rp worktree.ResolvedPath) error {
	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		if err := worktree.Create(rp.EngAreaDir, rp.RepoPath, rp.AbsPath); err != nil {
			return err
		}
	}

	return os.Chdir(rp.AbsPath)
}

func Attach(exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string) error {
	if err := Create(rp); err != nil {
		return err
	}

	var command []string
	if len(claudeArgs) > 0 {
		if flake.HasDevShell(rp.AbsPath) {
			log.Info("flake.nix detected, starting claude in nix develop")
			command = append([]string{"nix", "develop", "--command", "claude"}, claudeArgs...)
		} else {
			command = append([]string{"claude"}, claudeArgs...)
		}
	} else if flake.HasDevShell(rp.AbsPath) {
		log.Info("flake.nix detected, starting session in nix develop")
		command = []string{"nix", "develop", "--command", os.Getenv("SHELL")}
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return CloseShop(rp, format)
}

func CloseShop(rp worktree.ResolvedPath, format string) error {
	if rp.Branch == "" {
		if err := rp.FillBranchFromGit(); err != nil {
			log.Warn("could not determine current branch")
			return nil
		}
	}

	defaultBranch, err := git.BranchCurrent(rp.RepoPath)
	if err != nil || defaultBranch == "" {
		log.Warn("could not determine default branch")
		return nil
	}

	commitsAhead := git.CommitsAhead(rp.AbsPath, defaultBranch, rp.Branch)
	worktreeStatus := git.StatusPorcelain(rp.AbsPath)

	desc := statusDescription(defaultBranch, commitsAhead, worktreeStatus)

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.PlanAhead(1)
		tw.Ok("close " + rp.Branch + " # " + desc)
	} else {
		log.Info(desc, "worktree", rp.SessionKey)
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
