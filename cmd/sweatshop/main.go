package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/amarbel-llc/sweatshop/internal/clean"
	"github.com/amarbel-llc/sweatshop/internal/completions"
	"github.com/amarbel-llc/sweatshop/internal/executor"
	"github.com/amarbel-llc/sweatshop/internal/merge"
	"github.com/amarbel-llc/sweatshop/internal/perms"
	"github.com/amarbel-llc/sweatshop/internal/pull"
	"github.com/amarbel-llc/sweatshop/internal/shop"
	"github.com/amarbel-llc/sweatshop/internal/status"
	"github.com/amarbel-llc/sweatshop/internal/worktree"
)

var outputFormat string
var createRepo string

var rootCmd = &cobra.Command{
	Use:   "sweatshop",
	Short: "Shell-agnostic git worktree session manager",
	Long:  `sweatshop manages git worktree lifecycles: opening shops (creating worktrees + sessions), and offering close shop workflows (rebase, merge, cleanup, push).`,
}

var createCmd = &cobra.Command{
	Use:   "create <target>",
	Short: "Create a worktree without attaching",
	Long:  `Create a new worktree and apply sweatfile settings. Does not start a session. Target can be a convention path (<eng_area>/worktrees/<repo>/<branch>), an absolute path, or a relative path. Non-convention paths require --repo.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		rp, err := worktree.ResolvePath(home, args[0], createRepo)
		if err != nil {
			return err
		}

		return shop.Create(rp)
	},
}

var attachCmd = &cobra.Command{
	Use:     "attach <target> [claude args...]",
	Aliases: []string{"open"},
	Short:   "Create (if needed) and attach to a worktree session",
	Long:    `Create a worktree if it doesn't exist, then attach to a session. Target can be a convention path ([host:]<eng_area>/worktrees/<repo>/<branch>), an absolute path, or a relative path. Non-convention paths require --repo. If additional arguments are provided, claude is launched with those arguments instead of a shell.`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		format := outputFormat
		if format == "" {
			format = "tap"
		}

		exec := executor.ShellExecutor{}

		var claudeArgs []string
		if len(args) >= 2 {
			claudeArgs = args[1:]
		}

		target := worktree.ParseTarget(args[0])

		if target.Host != "" {
			return shop.OpenRemote(target.Host, target.Path)
		}

		rp, err := worktree.ResolvePath(home, target.Path, createRepo)
		if err != nil {
			return err
		}

		return shop.Attach(exec, rp, format, claudeArgs)
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

		format := outputFormat
		if format == "" {
			format = "table"
		}

		rows := status.CollectStatus(home)
		if len(rows) == 0 {
			log.Info("no repos found")
			return nil
		}

		if format == "tap" {
			status.RenderTap(rows, os.Stdout)
		} else {
			fmt.Println(status.Render(rows))
		}
		return nil
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge current worktree into main",
	Long:  `Run from inside a worktree. Merges the worktree branch into the main repo with --ff-only and removes the worktree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return merge.Run(executor.ShellExecutor{})
	},
}

var cleanInteractive bool

var pullDirty bool

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull repos and rebase worktrees",
	Long:  `Pull all clean repos, then rebase all clean worktrees onto their repo's default branch. Use -d to include dirty repos and worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return pull.Run(home, pullDirty)
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove merged worktrees",
	Long:  `Scan all worktrees, identify those whose branches are fully merged into the main branch, and remove them. Use -i to interactively handle dirty worktrees.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		format := outputFormat
		if format == "" {
			format = "tap"
		}

		return clean.Run(home, cleanInteractive, format)
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
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "", "output format: tap or table")
	createCmd.Flags().StringVar(&createRepo, "repo", "", "absolute path to the base git repository")
	attachCmd.Flags().StringVar(&createRepo, "repo", "", "absolute path to the base git repository")
	cleanCmd.Flags().BoolVarP(&cleanInteractive, "interactive", "i", false, "interactively discard changes in dirty merged worktrees")
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(completionsCmd)
	pullCmd.Flags().BoolVarP(&pullDirty, "dirty", "d", false, "include dirty repos and worktrees")
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(perms.NewPermsCmd())
}

func main() {
	rootCmd.Use = filepath.Base(os.Args[0])
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
