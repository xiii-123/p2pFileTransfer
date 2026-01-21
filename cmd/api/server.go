package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"p2pFileTransfer/pkg/config"
	"p2pFileTransfer/pkg/p2p"
)

// Server HTTP服务器
type Server struct {
	server      *http.Server
	p2pService  *p2p.P2PService
	config      *config.Config
	router      *http.ServeMux
	mu          sync.RWMutex
	started     bool
}

// NewServer 创建新的HTTP服务器
func NewServer(cfg *config.Config) (*Server, error) {
	// 创建P2P服务
	ctx := context.Background()
	p2pSvc, err := p2p.NewP2PService(ctx, p2p.NewP2PConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P service: %w", err)
	}

	// 创建路由器
	router := http.NewServeMux()

	// 创建HTTP服务器
	srv := &Server{
		p2pService: p2pSvc,
		config:     cfg,
		router:     router,
		started:    false,
	}

	// 注册路由
	srv.registerRoutes()

	// 配置HTTP服务器
	srv.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      srv.corsMiddleware(router),
		ReadTimeout:  30 * time.Minute, // 文件上传需要更长时间
		WriteTimeout: 30 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	return srv, nil
}

// registerRoutes 注册所有路由
func (s *Server) registerRoutes() {
	// 健康检查
	s.router.HandleFunc("GET /api/health", s.handleHealth)

	// 文件操作
	s.router.HandleFunc("POST /api/v1/files/upload", s.handleFileUpload)
	s.router.HandleFunc("GET /api/v1/files/{cid}", s.handleFileInfo)
	s.router.HandleFunc("GET /api/v1/files/{cid}/download", s.handleFileDownload)

	// 分片操作
	s.router.HandleFunc("GET /api/v1/chunks/{hash}", s.handleChunkInfo)
	s.router.HandleFunc("GET /api/v1/chunks/{hash}/download", s.handleChunkDownload)

	// 节点管理
	s.router.HandleFunc("GET /api/v1/node/info", s.handleNodeInfo)
	s.router.HandleFunc("GET /api/v1/node/peers", s.handlePeerList)
	s.router.HandleFunc("POST /api/v1/node/connect", s.handlePeerConnect)

	// DHT操作
	s.router.HandleFunc("GET /api/v1/dht/providers/{key}", s.handleDHTFindProviders)
	s.router.HandleFunc("POST /api/v1/dht/announce", s.handleDHTAnnounce)
	s.router.HandleFunc("GET /api/v1/dht/value/{key}", s.handleDHTGetValue)
	s.router.HandleFunc("POST /api/v1/dht/value", s.handleDHTPutValue)
}

// Start 启动HTTP服务器
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	fmt.Printf("HTTP API server listening on :%d\n", s.config.HTTP.Port)
	fmt.Println("Available endpoints:")
	fmt.Println("  GET    /api/health")
	fmt.Println("  POST   /api/v1/files/upload")
	fmt.Println("  GET    /api/v1/files/{cid}")
	fmt.Println("  GET    /api/v1/files/{cid}/download")
	fmt.Println("  GET    /api/v1/chunks/{hash}")
	fmt.Println("  GET    /api/v1/chunks/{hash}/download")
	fmt.Println("  GET    /api/v1/node/info")
	fmt.Println("  GET    /api/v1/node/peers")
	fmt.Println("  POST   /api/v1/node/connect")
	fmt.Println("  GET    /api/v1/dht/providers/{key}")
	fmt.Println("  POST   /api/v1/dht/announce")
	fmt.Println("  GET    /api/v1/dht/value/{key}")
	fmt.Println("  POST   /api/v1/dht/value")
	fmt.Println()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// Shutdown 关闭HTTP服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.mu.Unlock()

	fmt.Println("Shutting down HTTP API server...")

	// 关闭HTTP服务器
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("HTTP server shutdown error: %w", err)
	}

	// 关闭P2P服务
	s.p2pService.Shutdown()

	fmt.Println("HTTP API server stopped")
	return nil
}

// corsMiddleware CORS中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
