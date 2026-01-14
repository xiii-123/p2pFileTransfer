// Package main 提供 P2P 文件传输服务的命令行入口
//
// 功能:
//   - 从配置文件或环境变量加载配置
//   - 启动 P2P 文件传输服务
//   - 处理系统信号，实现优雅关闭
//
// 使用示例:
//
//	// 使用默认配置文件 (config/config.yaml)
//	go run cmd/server/main.go
//
//	// 指定配置文件
//	go run cmd/server/main.go -config /path/to/config.yaml
//
//	// 使用环境变量覆盖配置
//	P2P_PORT=8000 P2P_LOG_LEVEL=debug go run cmd/server/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"p2pFileTransfer/pkg/config"
	"p2pFileTransfer/pkg/p2p"
)

var (
	// 版本信息
	version = "1.0.0"
	commit  = "unknown"
	date    = "unknown"

	// 命令行参数
	configPath  string
	showHelp    bool
	showVersion bool
)

func init() {
	// 设置命令行参数
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
}

func main() {
	flag.Parse()

	// 处理命令行参数
	if showHelp {
		printHelp()
		os.Exit(0)
	}

	if showVersion {
		printVersion()
		os.Exit(0)
	}

	// 打印欢迎信息
	printBanner()

	// 获取配置文件路径
	configFile := config.GetConfigPath(configPath)
	logrus.Infof("Loading configuration from: %s", configFile)

	// 加载配置
	cfg, err := config.Load(configFile)
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// 确保必要的目录存在
	if err := cfg.EnsureDirectories(); err != nil {
		logrus.Fatalf("Failed to create directories: %v", err)
	}

	// 配置日志
	setupLogging(cfg)

	logrus.Info("Configuration loaded successfully")
	logrus.Infof("Network: port=%d, insecure=%v", cfg.Network.Port, cfg.Network.Insecure)
	logrus.Infof("Storage: chunk_path=%s, block_size=%d", cfg.Storage.ChunkPath, cfg.Storage.BlockSize)
	logrus.Infof("Performance: max_concurrency=%d, max_retries=%d", cfg.Performance.MaxConcurrency, cfg.Performance.MaxRetries)

	// 转换为 P2PConfig
	p2pConfig := cfg.ToP2PConfig()

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建 P2P 服务
	logrus.Info("Starting P2P service...")
	service, err := p2p.NewP2PService(ctx, *p2pConfig)
	if err != nil {
		logrus.Fatalf("Failed to create P2P service: %v", err)
	}

	// 打印节点信息
	printNodeInfo(service)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待信号
	logrus.Info("P2P service is running. Press Ctrl+C to stop.")
	<-sigChan

	logrus.Info("Received shutdown signal, shutting down gracefully...")

	// 关闭服务
	if err := service.Shutdown(); err != nil {
		logrus.Errorf("Error during shutdown: %v", err)
		os.Exit(1)
	}

	logrus.Info("Shutdown complete. Goodbye!")
}

// setupLogging 配置日志系统
func setupLogging(cfg *config.Config) {
	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		logrus.Warnf("Invalid log level '%s', using 'info'", cfg.Logging.Level)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	// 设置日志格式
	if cfg.Logging.Format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
}

// printBanner 打印欢迎横幅
func printBanner() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   P2P File Transfer System                                    ║
║   Version: %s                                             	║
║                                                               ║
║   A decentralized file transfer system powered by libp2p      ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`, version)
}

// printNodeInfo 打印节点信息
func printNodeInfo(service *p2p.P2PService) {
	peerID := service.Host.ID()
	addrs := service.Host.Addrs()

	fmt.Println("\n=== Node Information ===")
	fmt.Printf("Peer ID: %s\n", peerID)
	fmt.Println("\nListen Addresses:")
	for _, addr := range addrs {
		fmt.Printf("  - %s/p2p/%s\n", addr, peerID)
	}
	fmt.Println("========================\n")
}

// printHelp 打印帮助信息
func printHelp() {
	fmt.Printf(`P2P File Transfer System v%s

Usage:
  go run cmd/server/main.go [options]

Options:
  -config <path>   Path to configuration file (default: config/config.yaml)
  -h, -help        Show this help message
  -v, -version     Show version information

Configuration:
  The service can be configured via:
  1. Configuration file (YAML)
  2. Environment variables (P2P_* prefix)

  See config/config.example.yaml for all available options.

Environment Variables:
  P2P_PORT              Network port (0 = random)
  P2P_INSECURE          Use insecure connection (true/false)
  P2P_LOG_LEVEL         Log level (debug, info, warn, error)
  P2P_CHUNK_PATH        Chunk storage path
  P2P_MAX_CONCURRENCY   Max concurrent downloads
  ... and more

Examples:
  # Start with default configuration
  go run cmd/server/main.go

  # Start with custom config
  go run cmd/server/main.go -config /etc/p2p/config.yaml

  # Start with environment variable overrides
  P2P_PORT=8000 P2P_LOG_LEVEL=debug go run cmd/server/main.go

For more information, visit: https://github.com/yourusername/p2pFileTransfer
`, version)
}

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("P2P File Transfer System\n")
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Build Date: %s\n", date)
}
