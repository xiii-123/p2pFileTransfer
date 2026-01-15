// Package main provides the CLI commands for P2P File Transfer System
package main

import (
	"github.com/spf13/cobra"
	"p2pFileTransfer/cmd/p2p/file"
)

var (
	// Version information (set via ldflags during build)
	version = "1.0.0"
	commit  = "unknown"
	date    = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "p2p",
	Short: "P2P File Transfer System",
	Long: `A comprehensive CLI tool for P2P file transfer system powered by libp2p.

This tool provides commands for:
  • File operations (upload, download, info)
  • DHT operations (put, get, lookup)
  • Peer management (list, info)
  • Configuration management (show, validate)
  • Server management

For more information, visit: https://github.com/yourusername/p2pFileTransfer`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add global flags here if needed

	// Register file commands
	rootCmd.AddCommand(file.FileCmd)
}
