package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Print the version information for the JIT server.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("JIT Server:\n")
		fmt.Printf("  Version:  %s\n", version)
		fmt.Printf("  Commit:   %s\n", commit)
		fmt.Printf("  Built:    %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
