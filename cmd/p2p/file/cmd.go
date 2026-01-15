// Package file provides file operation commands for P2P File Transfer System
package file

import (
	"github.com/spf13/cobra"
)

// FileCmd represents the file command group
var FileCmd = &cobra.Command{
	Use:   "file",
	Short: "File operations",
	Long: `Manage files in the P2P network.

This command group provides operations for:
  • Uploading files to the network
  • Downloading files from the network
  • Viewing file metadata information`,
}
