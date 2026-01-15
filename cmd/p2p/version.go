// Package main provides the CLI commands for P2P File Transfer System
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Display version information including version number, commit hash, and build date.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("P2P File Transfer System\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build Date: %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
