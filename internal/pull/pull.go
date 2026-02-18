package pull

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/sweatshop/internal/git"
	"github.com/amarbel-llc/sweatshop/internal/tap"
)

type repoInfo struct {
	engArea  string
	name     string
	repoPath string
	dirty    bool
}

type worktreeInfo struct {
	engArea      string
	repo         string
	branch       string
	repoPath     string
	worktreePath string
	dirty        bool
}

func scanRepos(home string) []repoInfo {
	var repos []repoInfo

	pattern := filepath.Join(home, "eng*", "repos")
	matches, _ := filepath.Glob(pattern)

	for _, reposDir := range matches {
		engArea := filepath.Base(filepath.Dir(reposDir))
		entries, err := os.ReadDir(reposDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoPath := filepath.Join(reposDir, entry.Name())
			gitDir := filepath.Join(repoPath, ".git")
			if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
				continue
			}
			porcelain := git.StatusPorcelain(repoPath)
			repos = append(repos, repoInfo{
				engArea:  engArea,
				name:     entry.Name(),
				repoPath: repoPath,
				dirty:    porcelain != "",
			})
		}
	}

	return repos
}

func scanWorktrees(home string, repos []repoInfo) []worktreeInfo {
	var worktrees []worktreeInfo

	for _, repo := range repos {
		wtDir := filepath.Join(home, repo.engArea, "worktrees", repo.name)
		entries, err := os.ReadDir(wtDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			wtPath := filepath.Join(wtDir, entry.Name())
			porcelain := git.StatusPorcelain(wtPath)
			worktrees = append(worktrees, worktreeInfo{
				engArea:      repo.engArea,
				repo:         repo.name,
				branch:       entry.Name(),
				repoPath:     repo.repoPath,
				worktreePath: wtPath,
				dirty:        porcelain != "",
			})
		}
	}

	return worktrees
}

func Run(home string, dirty bool) error {
	tw := tap.NewWriter(os.Stdout)

	repos := scanRepos(home)
	worktrees := scanWorktrees(home, repos)

	if len(repos) == 0 && len(worktrees) == 0 {
		tw.Skip("pull", "no repos found")
		tw.Plan()
		return nil
	}

	var failed bool

	for _, repo := range repos {
		label := repo.engArea + "/repos/" + repo.name

		if repo.dirty && !dirty {
			tw.Skip("pull "+label, "dirty")
			continue
		}

		_, err := git.Pull(repo.repoPath)
		if err != nil {
			tw.NotOk("pull "+label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			failed = true
			continue
		}
		tw.Ok("pull " + label)
	}

	for _, wt := range worktrees {
		label := wt.engArea + "/worktrees/" + wt.repo + "/" + wt.branch

		if wt.dirty && !dirty {
			tw.Skip("rebase "+label, "dirty")
			continue
		}

		defaultBranch, err := git.DefaultBranch(wt.repoPath)
		if err != nil || defaultBranch == "" {
			tw.NotOk("rebase "+label, map[string]string{
				"message":  "could not determine default branch",
				"severity": "fail",
			})
			failed = true
			continue
		}

		_, err = git.Rebase(wt.worktreePath, defaultBranch)
		if err != nil {
			tw.NotOk("rebase "+label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			failed = true
			continue
		}
		tw.Ok("rebase " + label)
	}

	tw.Plan()

	if failed {
		return fmt.Errorf("one or more operations failed")
	}

	return nil
}
