// Package main provides the CLI commands for P2P File Transfer System
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"p2pFileTransfer/pkg/config"
	"p2pFileTransfer/pkg/p2p"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the P2P service",
	Long: `Start the P2P file transfer service.

This command starts a P2P node that can:
  • Connect to other peers in the network
  • Store and serve file chunks
  • Participate in DHT routing
  • Upload and download files

The service will continue running until interrupted with Ctrl+C.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return startServer(cmd)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add flags from server command
	serverCmd.Flags().StringP("config", "c", "", "Path to configuration file")
}

func startServer(cmd *cobra.Command) error {
	// Get config file path from flag or use default
	configPath, _ := cmd.Flags().GetString("config")
	configFile := config.GetConfigPath(configPath)

	logrus.Infof("Loading configuration from: %s", configFile)

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Ensure necessary directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Setup logging
	setupLogging(cfg)

	logrus.Info("Configuration loaded successfully")
	logrus.Infof("Network: port=%d, insecure=%v", cfg.Network.Port, cfg.Network.Insecure)
	logrus.Infof("Storage: chunk_path=%s, block_size=%d", cfg.Storage.ChunkPath, cfg.Storage.BlockSize)
	logrus.Infof("Performance: max_concurrency=%d, max_retries=%d", cfg.Performance.MaxConcurrency, cfg.Performance.MaxRetries)

	// Convert to P2PConfig
	p2pConfig := cfg.ToP2PConfig()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create P2P service
	logrus.Info("Starting P2P service...")
	service, err := p2p.NewP2PService(ctx, *p2pConfig)
	if err != nil {
		return fmt.Errorf("failed to create P2P service: %w", err)
	}

	// Print node information
	printNodeInfo(service)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for signal
	logrus.Info("P2P service is running. Press Ctrl+C to stop.")
	<-sigChan

	logrus.Info("Received shutdown signal, shutting down gracefully...")

	// Shutdown service
	if err := service.Shutdown(); err != nil {
		logrus.Errorf("Error during shutdown: %v", err)
		return err
	}

	logrus.Info("Shutdown complete. Goodbye!")
	return nil
}

// setupLogging configures the logging system
func setupLogging(cfg *config.Config) {
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		logrus.Warnf("Invalid log level '%s', using 'info'", cfg.Logging.Level)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	if cfg.Logging.Format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
}

// printNodeInfo prints node information to console
func printNodeInfo(service *p2p.P2PService) {
	peerID := service.Host.ID()
	addrs := service.Host.Addrs()

	fmt.Println("\n=== Node Information ===")
	fmt.Printf("Peer ID: %s\n", peerID)
	fmt.Println("\nListen Addresses:")
	for _, addr := range addrs {
		fmt.Printf("  - %s/p2p/%s\n", addr, peerID)
	}
	fmt.Println("========================")
}
