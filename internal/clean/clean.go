package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/sweatshop/internal/git"
)

type FileChange struct {
	Code string
	Path string
}

func ParsePorcelain(porcelain string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := line[3:]
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		changes = append(changes, FileChange{Code: code, Path: path})
	}
	return changes
}

func (fc FileChange) Description() string {
	switch {
	case fc.Code == "??":
		return "untracked"
	case fc.Code[1] == 'D' || fc.Code[0] == 'D':
		return "deleted"
	case fc.Code[0] == 'A':
		return "added"
	case fc.Code[0] == 'R':
		return "renamed"
	default:
		return "modified"
	}
}

type worktreeInfo struct {
	engArea      string
	repo         string
	branch       string
	repoPath     string
	worktreePath string
	merged       bool
	dirty        bool
}

func scanWorktrees(home string) []worktreeInfo {
	var worktrees []worktreeInfo

	pattern := filepath.Join(home, "eng*", "repos")
	matches, _ := filepath.Glob(pattern)

	for _, reposDir := range matches {
		engArea := filepath.Base(filepath.Dir(reposDir))
		repos, err := os.ReadDir(reposDir)
		if err != nil {
			continue
		}
		for _, repo := range repos {
			if !repo.IsDir() {
				continue
			}
			repoPath := filepath.Join(reposDir, repo.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
				continue
			}

			defaultBranch, err := git.DefaultBranch(repoPath)
			if err != nil || defaultBranch == "" {
				continue
			}

			wtDir := filepath.Join(home, engArea, "worktrees", repo.Name())
			entries, err := os.ReadDir(wtDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				wtPath := filepath.Join(wtDir, entry.Name())
				branch := entry.Name()

				ahead := git.CommitsAhead(wtPath, defaultBranch, branch)
				porcelain := git.StatusPorcelain(wtPath)

				worktrees = append(worktrees, worktreeInfo{
					engArea:      engArea,
					repo:         repo.Name(),
					branch:       branch,
					repoPath:     repoPath,
					worktreePath: wtPath,
					merged:       ahead == 0,
					dirty:        porcelain != "",
				})
			}
		}
	}

	return worktrees
}

func removeWorktree(wt worktreeInfo) error {
	if err := git.WorktreeRemove(wt.repoPath, wt.worktreePath); err != nil {
		return fmt.Errorf("removing worktree %s: %w", wt.branch, err)
	}
	if err := git.BranchDelete(wt.repoPath, wt.branch); err != nil {
		return fmt.Errorf("deleting branch %s: %w", wt.branch, err)
	}
	return nil
}

func discardFile(wtPath string, fc FileChange) error {
	if fc.Code == "??" {
		return os.Remove(filepath.Join(wtPath, fc.Path))
	}
	if fc.Code[0] != ' ' {
		if err := git.ResetFile(wtPath, fc.Path); err != nil {
			return err
		}
	}
	return git.CheckoutFile(wtPath, fc.Path)
}

func handleDirtyWorktree(wt worktreeInfo) (removed bool, err error) {
	porcelain := git.StatusPorcelain(wt.worktreePath)
	changes := ParsePorcelain(porcelain)

	for _, fc := range changes {
		var discard bool
		prompt := fmt.Sprintf("Discard %s (%s)?", fc.Path, fc.Description())
		err := huh.NewConfirm().
			Title(prompt).
			Value(&discard).
			Run()
		if err != nil {
			return false, err
		}
		if discard {
			if err := discardFile(wt.worktreePath, fc); err != nil {
				log.Warn("failed to discard file", "file", fc.Path, "err", err)
			}
		}
	}

	recheckPorcelain := git.StatusPorcelain(wt.worktreePath)
	if recheckPorcelain != "" {
		return false, nil
	}

	if err := removeWorktree(wt); err != nil {
		return false, err
	}
	return true, nil
}

func Run(home string, interactive bool) error {
	worktrees := scanWorktrees(home)
	if len(worktrees) == 0 {
		log.Info("no worktrees found")
		return nil
	}

	var removed, skipped int

	for _, wt := range worktrees {
		label := wt.engArea + "/worktrees/" + wt.repo + "/" + wt.branch

		if !wt.merged {
			continue
		}

		if !wt.dirty {
			if err := removeWorktree(wt); err != nil {
				log.Error("failed to remove worktree", "worktree", label, "err", err)
				skipped++
				continue
			}
			log.Info("removed", "worktree", label)
			removed++
			continue
		}

		if interactive {
			log.Info("dirty worktree", "worktree", label)
			wasRemoved, err := handleDirtyWorktree(wt)
			if err != nil {
				log.Error("error handling dirty worktree", "worktree", label, "err", err)
				skipped++
				continue
			}
			if wasRemoved {
				log.Info("removed", "worktree", label)
				removed++
			} else {
				log.Info("kept", "worktree", label)
				skipped++
			}
		} else {
			log.Info("skipped (dirty)", "worktree", label)
			skipped++
		}
	}

	log.Info("clean complete", "removed", removed, "skipped", skipped)
	return nil
}
