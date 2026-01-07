package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update manque-ai to the latest version",
	Long:  `Updates manque-ai to the latest version available on GitHub using 'go install'.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("📦 Updating manque-ai...")
		
		c := exec.Command("go", "install", "github.com/igcodinap/manque-ai@latest")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		
		if err := c.Run(); err != nil {
			fmt.Printf("❌ Update failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("✅ Update successful!")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
