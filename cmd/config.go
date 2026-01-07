package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/igcodinap/manque-ai/pkg/userconfig"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage manque-ai configuration",
	Long:  `Configure LLM provider, API key, and model for manque-ai.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive configuration wizard",
	Long:  `Guides you through setting up manque-ai with your preferred LLM provider.`,
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)
		config := &userconfig.UserConfig{}

		fmt.Println("🔧 manque-ai Configuration Wizard")
		fmt.Println("─────────────────────────────────")
		fmt.Println()

		// Provider selection
		fmt.Println("Select your LLM provider:")
		fmt.Println("  1) openrouter (recommended - free tier available)")
		fmt.Println("  2) openai")
		fmt.Println("  3) anthropic")
		fmt.Println("  4) google")
		fmt.Print("\nChoice [1]: ")
		
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		
		switch choice {
		case "", "1":
			config.Provider = "openrouter"
		case "2":
			config.Provider = "openai"
		case "3":
			config.Provider = "anthropic"
		case "4":
			config.Provider = "google"
		default:
			fmt.Println("❌ Invalid choice, using openrouter")
			config.Provider = "openrouter"
		}

		// API key
		fmt.Printf("\nEnter your %s API key: ", config.Provider)
		apiKey, _ := reader.ReadString('\n')
		config.APIKey = strings.TrimSpace(apiKey)

		if config.APIKey == "" {
			fmt.Println("⚠️  No API key provided. You'll need to set it later or via LLM_API_KEY env var.")
		}

		// Model selection
		defaultModel := getDefaultModel(config.Provider)
		fmt.Printf("\nEnter model name [%s]: ", defaultModel)
		model, _ := reader.ReadString('\n')
		model = strings.TrimSpace(model)
		
		if model == "" {
			config.Model = defaultModel
		} else {
			config.Model = model
		}

		// Save
		if err := config.Save(); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}

		path, _ := userconfig.ConfigPath()
		fmt.Println()
		fmt.Println("✅ Configuration saved!")
		fmt.Printf("   File: %s\n", path)
		fmt.Println()
		fmt.Println("You can now run: manque-ai local")
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := userconfig.Load()
		if err != nil {
			fmt.Printf("❌ Failed to load config: %v\n", err)
			os.Exit(1)
		}

		path, _ := userconfig.ConfigPath()
		
		fmt.Println("📋 manque-ai Configuration")
		fmt.Println("──────────────────────────")
		fmt.Printf("Config file: %s\n\n", path)
		
		provider := config.Provider
		if provider == "" {
			provider = "(not set, default: openrouter)"
		}
		fmt.Printf("Provider: %s\n", provider)
		fmt.Printf("API Key:  %s\n", config.MaskedAPIKey())
		
		model := config.Model
		if model == "" {
			model = "(not set, default: mistralai/mistral-7b-instruct:free)"
		}
		fmt.Printf("Model:    %s\n", model)
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Valid keys:
  - provider (or LLM_PROVIDER): openrouter, openai, anthropic, google
  - api_key (or LLM_API_KEY): your API key
  - model (or LLM_MODEL): the model to use`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key, value := args[0], args[1]

		config, err := userconfig.Load()
		if err != nil {
			fmt.Printf("❌ Failed to load config: %v\n", err)
			os.Exit(1)
		}

		if err := config.Set(key, value); err != nil {
			fmt.Printf("❌ %v\n", err)
			os.Exit(1)
		}

		if err := config.Save(); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Set %s\n", key)
	},
}

var configClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		if err := userconfig.Clear(); err != nil {
			fmt.Printf("❌ Failed to clear config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ Configuration cleared")
	},
}

func getDefaultModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-4o"
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "google":
		return "gemini-2.0-flash"
	case "openrouter":
		return "mistralai/mistral-7b-instruct:free"
	default:
		return "mistralai/mistral-7b-instruct:free"
	}
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configClearCmd)
}
