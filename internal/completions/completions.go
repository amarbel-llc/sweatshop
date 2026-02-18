package completions

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

func Local(home string, w io.Writer) {
	pattern := filepath.Join(home, "eng*")
	areas, _ := filepath.Glob(pattern)

	for _, areaPath := range areas {
		info, err := os.Stat(areaPath)
		if err != nil || !info.IsDir() {
			continue
		}
		area := filepath.Base(areaPath)

		reposDir := filepath.Join(areaPath, "repos")
		repos, err := os.ReadDir(reposDir)
		if err != nil {
			continue
		}

		for _, repo := range repos {
			if !repo.IsDir() {
				continue
			}
			fmt.Fprintf(w, "%s/worktrees/%s/\tnew worktree\n", area, repo.Name())

			worktreeDir := filepath.Join(home, area, "worktrees", repo.Name())
			worktrees, err := os.ReadDir(worktreeDir)
			if err != nil {
				continue
			}
			for _, wt := range worktrees {
				if !wt.IsDir() {
					continue
				}
				wtPath := filepath.Join(worktreeDir, wt.Name())
				if !worktree.IsWorktree(wtPath) {
					continue
				}
				fmt.Fprintf(w, "%s/worktrees/%s/%s\texisting worktree\n", area, repo.Name(), wt.Name())
			}
		}
	}
}

func Remote(home string, w io.Writer) {
	hosts := remoteHosts(home)
	for _, host := range hosts {
		scanRemoteHost(host, w)
	}
}

func remoteHosts(home string) []string {
	if envHosts := os.Getenv("SWEATSHOP_REMOTE_HOSTS"); envHosts != "" {
		return strings.Split(envHosts, ":")
	}

	configFile := filepath.Join(home, ".config", "sweatshop", "remotes")
	f, err := os.Open(configFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	var hosts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			hosts = append(hosts, line)
		}
	}
	return hosts
}

func scanRemoteHost(host string, w io.Writer) {
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=2", host,
		`find ~/eng*/worktrees -mindepth 2 -maxdepth 3 -type d 2>/dev/null | sed "s|^$HOME/||"`)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		remotePath := strings.TrimSpace(scanner.Text())
		if remotePath == "" {
			continue
		}
		parts := strings.Split(remotePath, "/")
		if len(parts) == 3 {
			fmt.Fprintf(w, "%s:%s/\tremote: new worktree\n", host, remotePath)
		} else if len(parts) == 4 {
			fmt.Fprintf(w, "%s:%s\tremote: existing worktree\n", host, remotePath)
		}
	}
}
