package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/manque-ai/internal"
	"github.com/manque-ai/pkg/ai"
	fileconfig "github.com/manque-ai/pkg/config"
	"github.com/manque-ai/pkg/discovery"
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
	localCmd.Flags().Bool("mock", false, "Run with mock AI response (for testing UI)")
	localCmd.Flags().Bool("no-discover", false, "Disable auto-discovery of repo practices")
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

	// 2b. Load file-based config (.manque.yml)
	cwd, err := os.Getwd()
	if err == nil {
		fileCfg, loadErr := fileconfig.LoadFromDirectory(cwd)
		if loadErr != nil {
			internal.Logger.Warn("Failed to load .manque.yml config", "error", loadErr)
		} else {
			// Merge file config into runtime config
			if fileCfg.Review.AutoApproveThreshold > 0 {
				config.AutoApproveThreshold = fileCfg.Review.AutoApproveThreshold
			}
			config.BlockOnCritical = fileCfg.Review.BlockOnCritical
			config.IgnorePatterns = fileCfg.Ignore

			// Convert path rules
			config.PathRules = make(map[string]internal.PathRule)
			for _, rule := range fileCfg.Rules {
				config.PathRules[rule.Path] = internal.PathRule{
					SeverityOverride: rule.SeverityOverride,
					ExtraRules:       rule.ExtraRules,
					Ignore:           rule.Ignore,
				}
			}
			internal.Logger.Debug("Loaded file config", "ignore_patterns", len(config.IgnorePatterns), "path_rules", len(config.PathRules))
		}
	}

	// 3. Discover repo practices (if enabled)
	noDiscover, err := cmd.Flags().GetBool("no-discover")
	if err != nil {
		internal.Logger.Warn("Could not parse --no-discover flag, defaulting to discovery enabled", "error", err)
		noDiscover = false
	}
	if !noDiscover && config.AutoDiscoverPractices {
		cwd, err := os.Getwd()
		if err != nil {
			internal.Logger.Warn("Could not get current directory for discovery", "error", err)
		} else {
			internal.Logger.Info("Discovering repo practices...")
			practices, err := discovery.Discover(cwd)
			if err != nil {
				internal.Logger.Warn("Failed to discover repo practices", "error", err)
			} else if practices.HasPractices() {
				config.DiscoveredPractices = practices.Combined
				internal.Logger.Info(practices.Summary())
				internal.Logger.Debug("Discovered practices content", "size", len(practices.Combined))
			} else {
				internal.Logger.Debug("No repo practices found")
			}
		}
	}

	// 4. Get Git Diff
	mock, err := cmd.Flags().GetBool("mock")
	if err != nil {
		internal.Logger.Warn("Could not parse --mock flag", "error", err)
		mock = false
	}
	var diffContent string

	if mock {
		internal.Logger.Info("Running in MOCK mode... skipping git diff")
		diffContent = "mock diff content"
	} else {
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
	
		diffContent = string(diffOut)
		if len(diffContent) == 0 {
			fmt.Println("No changes detected between branches.")
			return
		}
	
		internal.Logger.Debug("Diff retrieved", "size", len(diffContent))
	}

	// 3. Init Engine
	// We need to manually construct config or fix the validation issue.
	// Let's Assume LoadConfig succeeded (or we fix it in next step).
	
	engine, err := review.NewEngine(config)
	if err != nil {
		internal.Logger.Error("Failed to initialize engine", "error", err)
		return
	}

	// 4. Run Review
	var summary *ai.PRSummary
	var result *ai.ReviewResult
	
	if mock {
		internal.Logger.Info("Running in MOCK mode...")
		// Use manual PRSummary with fields that match ai/types.go
		summary = &ai.PRSummary{
			Description: "This is a **mock review** generated to demonstrate the terminal output format. In a real run, this would be generated by your chosen LLM.",
			Files: []struct {
				Filename string `json:"filename"`
				Summary  string `json:"summary"`
				Title    string `json:"title"`
			}{
				{Filename: "cmd/local.go", Summary: "Added mock mode for easier local verification and testing."},
				{Filename: "internal/config.go", Summary: "Refactored validation to support optional GitHub tokens."},
			},
		}
			result = &ai.ReviewResult{
			Review: ai.ReviewSummary{
				Score:            85,
				EstimatedEffort:  2,
				HasRelevantTests: true,
				SecurityConcerns: "None detected.",
			},
			Comments: []ai.Comment{
				{
					File:      "internal/payments/service/integration_test.go",
					StartLine: 104,
					EndLine:   106,
					Header:    "🟡 Remove duplicate line",
					Content:   "Line 105 is a duplicate of line 104. This will cause the payment to be stored twice.",
					Label:     "bug",
					HighlightedCode: "	r.payments[p.ID] = p\n	r.payments[p.ID] = p\n	return p, nil",
					SuggestedCode:   "	r.payments[p.ID] = p\n	return p, nil",
				},
				{
					File:      "internal/app/payments_initializer.go",
					StartLine: 22,
					EndLine:   26,
					Header:    "🔴 Missing validation for required environment variables",
					Content:   "The initializer reads MERCADOPAGO_ACCESS_TOKEN without validating it's set. An empty access token will cause all payment API calls to fail with unhelpful errors.",
					Label:     "security",
					Critical:  true,
					HighlightedCode: "func initializePayments(sqlDB *sql.DB) PaymentsComponents {\n	mpAccessToken := os.Getenv(\"MERCADOPAGO_ACCESS_TOKEN\")\n	mpWebhookSecret := os.Getenv(\"MERCADOPAGO_WEBHOOK_SECRET\")",
					SuggestedCode:   "func initializePayments(sqlDB *sql.DB) PaymentsComponents {\n	mpAccessToken := os.Getenv(\"MERCADOPAGO_ACCESS_TOKEN\")\n	if mpAccessToken == \"\" {\n		panic(\"MERCADOPAGO_ACCESS_TOKEN environment variable is required\")\n	}\n	mpWebhookSecret := os.Getenv(\"MERCADOPAGO_WEBHOOK_SECRET\")",
				},
			},
		}
	} else {
		internal.Logger.Info("Analyzing changes... (this may take a minute)")
		var err error
		summary, result, err = engine.Review(diffContent)
		if err != nil {
			internal.Logger.Error("Review extraction failed", "error", err)
			return
		}
	}

	// 5. Output
	output := review.FormatOutput(summary, result)
	fmt.Println("\n" + output)
}
