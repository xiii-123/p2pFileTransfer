// Package main provides the unified CLI entry point for P2P File Transfer System
//
// This CLI tool provides comprehensive command-line interface for:
//   - File operations (upload, download, info)
//   - DHT operations (put, get, lookup)
//   - Peer management (list, info)
//   - Configuration management (show, validate)
//   - Server management (start P2P service)
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
