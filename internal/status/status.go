package status

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/amarbel-llc/sweatshop/internal/git"
)

type BranchStatus struct {
	Repo         string
	Branch       string
	Dirty        string
	Remote       string
	LastCommit   string
	LastModified string
}

func CollectBranchStatus(repoLabel, branchPath, branchName string) BranchStatus {
	bs := BranchStatus{
		Repo:   repoLabel,
		Branch: branchName,
	}

	porcelain := git.StatusPorcelain(branchPath)
	if porcelain != "" {
		bs.Dirty = parseDirtyStatus(porcelain)
	} else {
		bs.Dirty = "clean"
	}

	upstream := git.Upstream(branchPath)
	if upstream != "" {
		ahead, behind := git.RevListLeftRight(branchPath)
		var parts []string
		if ahead > 0 {
			parts = append(parts, fmt.Sprintf("↑%d", ahead))
		}
		if behind > 0 {
			parts = append(parts, fmt.Sprintf("↓%d", behind))
		}
		if len(parts) > 0 {
			bs.Remote = strings.Join(parts, " ") + " " + upstream
		} else {
			bs.Remote = "≡ " + upstream
		}
	}

	bs.LastCommit = git.LastCommitDate(branchPath)

	newest := git.NewestFileTime(branchPath)
	if !newest.IsZero() {
		bs.LastModified = newest.Format("2006-01-02")
	} else {
		bs.LastModified = "n/a"
	}

	return bs
}

func parseDirtyStatus(porcelain string) string {
	lines := strings.Split(porcelain, "\n")

	reModified := regexp.MustCompile(`^.M`)
	reAdded := regexp.MustCompile(`^A`)
	reDeleted := regexp.MustCompile(`^.D`)
	reUntracked := regexp.MustCompile(`^\?\?`)

	var modified, added, deleted, untracked int
	for _, line := range lines {
		if line == "" {
			continue
		}
		if reModified.MatchString(line) {
			modified++
		}
		if reAdded.MatchString(line) {
			added++
		}
		if reDeleted.MatchString(line) {
			deleted++
		}
		if reUntracked.MatchString(line) {
			untracked++
		}
	}

	var parts []string
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%dM", modified))
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%dA", added))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("%dD", deleted))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d?", untracked))
	}
	return strings.Join(parts, " ")
}

func CollectRepoStatus(home, engArea, repo string) []BranchStatus {
	repoPath := filepath.Join(home, engArea, "repos", repo)

	gitDir := filepath.Join(repoPath, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return nil
	}

	repoLabel := engArea + "/repos/" + repo
	var rows []BranchStatus

	mainBranch, err := git.BranchCurrent(repoPath)
	if err == nil && mainBranch != "" {
		rows = append(rows, CollectBranchStatus(repoLabel, repoPath, mainBranch))
	}

	worktreesDir := filepath.Join(home, engArea, "worktrees", repo)
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return rows
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(worktreesDir, entry.Name())
		rows = append(rows, CollectBranchStatus(repoLabel, wtPath, entry.Name()))
	}

	return rows
}

func CollectStatus(home string) []BranchStatus {
	var all []BranchStatus

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
			rows := CollectRepoStatus(home, engArea, entry.Name())
			all = append(all, rows...)
		}
	}

	return all
}

var (
	styleClean = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleDirty = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	styleAhead = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	styleEven  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
	styleBold  = lipgloss.NewStyle().Bold(true)
)

func Render(rows []BranchStatus) string {
	headers := []string{"Repo", "Branch", "Status", "Remote", "Commit", "Modified"}

	data := make([][]string, len(rows))
	for i, r := range rows {
		data[i] = []string{r.Repo, r.Branch, r.Dirty, r.Remote, r.LastCommit, r.LastModified}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8"))).
		Headers(headers...).
		Rows(data...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return styleBold
			}

			switch col {
			case 2: // Status
				val := data[row][col]
				if val == "clean" {
					return styleClean
				}
				return styleDirty
			case 3: // Remote
				val := data[row][col]
				if strings.HasPrefix(val, "≡") {
					return styleEven
				}
				if strings.Contains(val, "↑") || strings.Contains(val, "↓") {
					return styleAhead
				}
				return styleDim
			case 4, 5: // dates
				return styleDim
			}

			return lipgloss.NewStyle()
		})

	return t.Render()
}
