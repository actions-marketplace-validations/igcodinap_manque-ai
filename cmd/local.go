package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/manque-ai/internal"
	"github.com/manque-ai/pkg/review"
	"github.com/spf13/cobra"
)

var (
	baseBranch string
	headBranch string
)

var localCmd = &cobra.Command{
	Use:   "local",
	Short: "Run AI review locally on git changes",
	Long:  `Analyzes changes between two git branches (or commits) and outputs an AI review to the terminal.`,
	Run:   runLocalReview,
}

func init() {
	rootCmd.AddCommand(localCmd)
	localCmd.Flags().StringVar(&baseBranch, "base", "main", "Base branch to compare against")
	localCmd.Flags().StringVar(&headBranch, "head", "HEAD", "Head branch (changes source)")
}

func runLocalReview(cmd *cobra.Command, args []string) {
	// 1. Initialize Logger
	debug, _ := cmd.Flags().GetBool("debug")
	internal.InitLogger(debug)

	// 2. Load Config
	config, err := internal.LoadConfig()
	if err != nil {
		internal.Logger.Error("Failed to load configuration", "error", err)
		return
	}
	
	// For local review, GH_TOKEN is optional
	config.SkipGitHubValidation = true
	if err := config.Validate(); err != nil {
		internal.Logger.Error("Invalid configuration", "error", err)
		return
	}

	// 3. Get Git Diff
	internal.Logger.Info("Getting git diff...", "base", baseBranch, "head", headBranch)
	
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		internal.Logger.Error("Git not found in PATH")
		return
	}

	// Run git diff
	// Use merge-base to find common ancestor for better diff
	mergeBaseCmd := exec.Command("git", "merge-base", baseBranch, headBranch)
	mergeBaseOut, err := mergeBaseCmd.Output()
	if err != nil {
		internal.Logger.Error("Failed to find merge base. Are branches valid?", "error", err)
		return
	}
	commonAncestor := strings.TrimSpace(string(mergeBaseOut))

	diffCmd := exec.Command("git", "diff", commonAncestor, headBranch)
	diffOut, err := diffCmd.Output()
	if err != nil {
		internal.Logger.Error("Failed to git diff", "error", err)
		return
	}

	diffContent := string(diffOut)
	if len(diffContent) == 0 {
		fmt.Println("No changes detected between branches.")
		return
	}

	internal.Logger.Debug("Diff retrieved", "size", len(diffContent))

	// 3. Init Engine
	// We need to manually construct config or fix the validation issue.
	// Let's Assume LoadConfig succeeded (or we fix it in next step).
	
	engine, err := review.NewEngine(config)
	if err != nil {
		internal.Logger.Error("Failed to initialize engine", "error", err)
		return
	}

	// 4. Run Review
	internal.Logger.Info("Analyzing changes... (this may take a minute)")
	summary, result, err := engine.Review(diffContent)
	if err != nil {
		internal.Logger.Error("Review extraction failed", "error", err)
		return
	}

	// 5. Output
	output := review.FormatOutput(summary, result)
	fmt.Println("\n" + output)
}
