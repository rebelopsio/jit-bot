package cmd

import (
	"log/slog"

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
		slog.Info("JIT Server Version Information",
			"version", version,
			"commit", commit,
			"built", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
