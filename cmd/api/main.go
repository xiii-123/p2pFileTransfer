package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"p2pFileTransfer/pkg/config"
)

func main() {
	// 定义命令行参数
	configPath := flag.String("config", "", "Path to configuration file")
	httpPort := flag.Int("port", 8080, "HTTP server port")
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help information")

	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("P2P File Transfer HTTP API Server\n")
		fmt.Printf("Version: 1.0.0\n")
		fmt.Printf("Build: 2024-01-15\n")
		return
	}

	// 显示帮助信息
	if *showHelp {
		fmt.Printf("P2P File Transfer HTTP API Server\n\n")
		fmt.Printf("Usage: p2p-api [options]\n\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
		fmt.Printf("\nAPI Endpoints:\n")
		fmt.Printf("  GET    /api/health\n")
		fmt.Printf("  POST   /api/v1/files/upload\n")
		fmt.Printf("  GET    /api/v1/files/{cid}\n")
		fmt.Printf("  GET    /api/v1/files/{cid}/download\n")
		fmt.Printf("  GET    /api/v1/chunks/{hash}\n")
		fmt.Printf("  GET    /api/v1/chunks/{hash}/download\n")
		fmt.Printf("  GET    /api/v1/node/info\n")
		fmt.Printf("  GET    /api/v1/node/peers\n")
		fmt.Printf("  POST   /api/v1/node/connect\n")
		fmt.Printf("  GET    /api/v1/dht/providers/{key}\n")
		fmt.Printf("  POST   /api/v1/dht/announce\n")
		fmt.Printf("  GET    /api/v1/dht/value/{key}\n")
		fmt.Printf("  POST   /api/v1/dht/value\n")
		return
	}

	// 打印启动信息
	fmt.Printf("========================\n")
	fmt.Printf("P2P File Transfer HTTP API\n")
	fmt.Printf("========================\n\n")

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// 命令行参数覆盖配置文件
	if *httpPort != 8080 {
		cfg.HTTP.Port = *httpPort
	}

	// 创建HTTP服务器
	server, err := NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在goroutine中启动服务器
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// 等待信号或错误
	select {
	case <-sigChan:
		fmt.Printf("\nReceived shutdown signal\n")
	case err := <-errChan:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}

	// 优雅关闭
	ctx := context.Background()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		os.Exit(1)
	}
}
