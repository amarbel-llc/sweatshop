package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/amarbel-llc/sweatshop/internal/attach"
	"github.com/amarbel-llc/sweatshop/internal/clean"
	"github.com/amarbel-llc/sweatshop/internal/completions"
	"github.com/amarbel-llc/sweatshop/internal/merge"
	"github.com/amarbel-llc/sweatshop/internal/status"
	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

var rootCmd = &cobra.Command{
	Use:   "sweatshop",
	Short: "Shell-agnostic git worktree session manager",
	Long:  `sweatshop manages git worktree lifecycles: creating them, attaching to terminal sessions via zmx, and offering post-session workflows.`,
}

var attachCmd = &cobra.Command{
	Use:   "attach [target]",
	Short: "Attach to a worktree session",
	Long:  `Attach to an existing or new worktree session. Target format: [host:]<eng_area>/worktrees/<repo>/<branch>`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			sweatshopPath := cwd[len(home)+1:]

			if info, err := os.Stat(cwd); err == nil && info.IsDir() {
				return attach.Existing(sweatshopPath)
			}
			return attach.ToPath(sweatshopPath)
		}

		target := worktree.ParseTarget(args[0])

		if target.Host != "" {
			return attach.Remote(target.Host, target.Path)
		}

		fullPath := home + "/" + target.Path
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			return attach.Existing(target.Path)
		}

		return attach.ToPath(target.Path)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all repos and worktrees",
	Long:  `Scan all eng*/repos/ directories and display a styled table showing branch status, dirty state, remote tracking, and modification dates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		rows := status.CollectStatus(home)
		if len(rows) == 0 {
			log.Info("no repos found")
			return nil
		}

		fmt.Println(status.Render(rows))
		return nil
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge current worktree into main",
	Long:  `Run from inside a worktree. Merges the worktree branch into the main repo with --no-ff, removes the worktree, and detaches from zmx.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return merge.Run()
	},
}

var cleanInteractive bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove merged worktrees",
	Long:  `Scan all worktrees, identify those whose branches are fully merged into the main branch, and remove them. Use -i to interactively handle dirty worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		return clean.Run(home, cleanInteractive)
	},
}

var completionsCmd = &cobra.Command{
	Use:    "completions",
	Short:  "Generate tab-separated completions",
	Long:   `Output tab-separated completion entries for shell integration. Scans local and remote worktrees.`,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		completions.Local(home, os.Stdout)
		completions.Remote(home, os.Stdout)
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVarP(&cleanInteractive, "interactive", "i", false, "interactively discard changes in dirty merged worktrees")
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(completionsCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
