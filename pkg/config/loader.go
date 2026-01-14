// Package config 提供 P2P 文件传输系统的配置加载功能
//
// 核心功能:
//   - 从 YAML 文件加载配置
//   - 支持环境变量覆盖
//   - 配置验证
//   - 转换为 P2PConfig 结构
//
// 使用示例:
//   // 加载配置
//   cfg, err := config.Load("config/config.yaml")
//   if err != nil {
//       log.Fatal(err)
//   }
//
//   // 转换为 P2PConfig
//   p2pConfig := cfg.ToP2PConfig()
//
// 配置优先级:
//   1. 环境变量（最高优先级）
//   2. 配置文件
//   3. 默认值（最低优先级）
//
// 环境变量命名规则:
//   - 配置项使用 P2P_ 前缀
//   - 使用大写字母和下划线
//   - 例如: P2P_PORT, P2P_MAX_RETRIES
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/viper"
	"p2pFileTransfer/pkg/p2p"
)

// Config 包含所有配置项
type Config struct {
	Network      NetworkConfig      `mapstructure:"network"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Performance  PerformanceConfig  `mapstructure:"performance"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	AntiLeecher  AntiLeecherConfig  `mapstructure:"anti_leecher"`
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	Port           int      `mapstructure:"port"`
	Insecure       bool     `mapstructure:"insecure"`
	Seed           int64    `mapstructure:"seed"`
	BootstrapPeers []string `mapstructure:"bootstrap_peers"`
	ProtocolPrefix string   `mapstructure:"protocol_prefix"`
	AutoRefresh    bool     `mapstructure:"auto_refresh"`
	NameSpace      string   `mapstructure:"namespace"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	ChunkPath    string `mapstructure:"chunk_path"`
	BlockSize    uint   `mapstructure:"block_size"`
	BufferNumber uint   `mapstructure:"buffer_number"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxRetries     int  `mapstructure:"max_retries"`
	MaxConcurrency int  `mapstructure:"max_concurrency"`
	RequestTimeout int  `mapstructure:"request_timeout"`
	DataTimeout    int  `mapstructure:"data_timeout"`
	DHTTimeout     int  `mapstructure:"dht_timeout"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// AntiLeecherConfig 反吸血虫配置
type AntiLeecherConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	MinSuccessRate   float64  `mapstructure:"min_success_rate"`
	MinRequests      int      `mapstructure:"min_requests"`
	BlacklistTimeout int      `mapstructure:"blacklist_timeout"`
}

// Load 从配置文件加载配置
// 如果配置文件不存在，返回默认配置
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 配置文件路径
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 尝试查找配置文件
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/p2p-file-transfer")
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件未找到，使用默认值
			fmt.Println("Config file not found, using defaults")
		} else {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// 绑定环境变量
	bindEnvVars(v)

	// 解析到结构体
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认配置值
func setDefaults(v *viper.Viper) {
	// 网络配置默认值
	v.SetDefault("network.port", 0)
	v.SetDefault("network.insecure", false)
	v.SetDefault("network.seed", int64(0))
	v.SetDefault("network.bootstrap_peers", []string{})
	v.SetDefault("network.protocol_prefix", "/p2p-file-transfer")
	v.SetDefault("network.auto_refresh", true)
	v.SetDefault("network.namespace", "p2p-file-transfer")

	// 存储配置默认值
	v.SetDefault("storage.chunk_path", "files")
	v.SetDefault("storage.block_size", 256*1024) // 256KB
	v.SetDefault("storage.buffer_number", 16)

	// 性能配置默认值
	v.SetDefault("performance.max_retries", 3)
	v.SetDefault("performance.max_concurrency", 16)
	v.SetDefault("performance.request_timeout", 5)
	v.SetDefault("performance.data_timeout", 30)
	v.SetDefault("performance.dht_timeout", 10)

	// 日志配置默认值
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")

	// 反吸血虫配置默认值
	v.SetDefault("anti_leecher.enabled", true)
	v.SetDefault("anti_leecher.min_success_rate", 0.5)
	v.SetDefault("anti_leecher.min_requests", 10)
	v.SetDefault("anti_leecher.blacklist_timeout", 3600) // 1 hour
}

// bindEnvVars 绑定环境变量
func bindEnvVars(v *viper.Viper) {
	// 设置环境变量前缀
	v.SetEnvPrefix("P2P")
	v.AutomaticEnv()

	// 绑定各个配置项到环境变量
	bindings := map[string]string{
		"network.port":              "PORT",
		"network.insecure":          "INSECURE",
		"network.seed":              "SEED",
		"network.bootstrap_peers":   "BOOTSTRAP_PEERS",
		"network.protocol_prefix":   "PROTOCOL_PREFIX",
		"network.auto_refresh":      "AUTO_REFRESH",
		"network.namespace":         "NAMESPACE",
		"storage.chunk_path":        "CHUNK_PATH",
		"storage.block_size":        "BLOCK_SIZE",
		"storage.buffer_number":     "BUFFER_NUMBER",
		"performance.max_retries":   "MAX_RETRIES",
		"performance.max_concurrency": "MAX_CONCURRENCY",
		"performance.request_timeout": "REQUEST_TIMEOUT",
		"performance.data_timeout":  "DATA_TIMEOUT",
		"performance.dht_timeout":   "DHT_TIMEOUT",
		"logging.level":             "LOG_LEVEL",
		"logging.format":            "LOG_FORMAT",
		"anti_leecher.enabled":      "ANTI_LEECHER_ENABLED",
		"anti_leecher.min_success_rate": "MIN_SUCCESS_RATE",
		"anti_leecher.min_requests": "MIN_REQUESTS",
		"anti_leecher.blacklist_timeout": "BLACKLIST_TIMEOUT",
	}

	for configKey, envKey := range bindings {
		if err := v.BindEnv(configKey, "P2P_"+envKey); err != nil {
			fmt.Printf("Warning: failed to bind env var %s: %v\n", envKey, err)
		}
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证网络配置
	if c.Network.Port < 0 || c.Network.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 0-65535)", c.Network.Port)
	}

	// 验证存储配置
	if c.Storage.ChunkPath == "" {
		return fmt.Errorf("chunk_path cannot be empty")
	}

	if c.Storage.BlockSize < 1024 || c.Storage.BlockSize > 4*1024*1024 {
		return fmt.Errorf("invalid block_size: %d (must be 1KB-4MB)", c.Storage.BlockSize)
	}

	if c.Storage.BufferNumber < 1 || c.Storage.BufferNumber > 256 {
		return fmt.Errorf("invalid buffer_number: %d (must be 1-256)", c.Storage.BufferNumber)
	}

	// 验证性能配置
	if c.Performance.MaxRetries < 0 || c.Performance.MaxRetries > 100 {
		return fmt.Errorf("invalid max_retries: %d (must be 0-100)", c.Performance.MaxRetries)
	}

	if c.Performance.MaxConcurrency < 1 || c.Performance.MaxConcurrency > 1024 {
		return fmt.Errorf("invalid max_concurrency: %d (must be 1-1024)", c.Performance.MaxConcurrency)
	}

	if c.Performance.RequestTimeout < 1 || c.Performance.RequestTimeout > 3600 {
		return fmt.Errorf("invalid request_timeout: %d (must be 1-3600)", c.Performance.RequestTimeout)
	}

	if c.Performance.DataTimeout < 1 || c.Performance.DataTimeout > 7200 {
		return fmt.Errorf("invalid data_timeout: %d (must be 1-7200)", c.Performance.DataTimeout)
	}

	if c.Performance.DHTTimeout < 1 || c.Performance.DHTTimeout > 3600 {
		return fmt.Errorf("invalid dht_timeout: %d (must be 1-3600)", c.Performance.DHTTimeout)
	}

	// 验证日志配置
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log_level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log_format: %s (must be json or text)", c.Logging.Format)
	}

	// 验证反吸血虫配置
	if c.AntiLeecher.MinSuccessRate < 0.0 || c.AntiLeecher.MinSuccessRate > 1.0 {
		return fmt.Errorf("invalid min_success_rate: %.2f (must be 0.0-1.0)", c.AntiLeecher.MinSuccessRate)
	}

	if c.AntiLeecher.MinRequests < 1 || c.AntiLeecher.MinRequests > 10000 {
		return fmt.Errorf("invalid min_requests: %d (must be 1-10000)", c.AntiLeecher.MinRequests)
	}

	return nil
}

// ToP2PConfig 转换为 P2PConfig
func (c *Config) ToP2PConfig() *p2p.P2PConfig {
	cfg := p2p.NewP2PConfig()

	cfg.Port = c.Network.Port
	cfg.Insecure = c.Network.Insecure
	cfg.Seed = c.Network.Seed
	cfg.ProtocolPrefix = c.Network.ProtocolPrefix
	cfg.EnableAutoRefresh = c.Network.AutoRefresh
	cfg.NameSpace = c.Network.NameSpace
	cfg.ChunkStoragePath = c.Storage.ChunkPath
	cfg.MaxRetries = c.Performance.MaxRetries
	cfg.MaxConcurrency = c.Performance.MaxConcurrency
	cfg.RequestTimeout = c.Performance.RequestTimeout
	cfg.DataTimeout = c.Performance.DataTimeout
	cfg.DHTTimeout = c.Performance.DHTTimeout

	// 解析 bootstrap peers
	if len(c.Network.BootstrapPeers) > 0 {
		bootstrapPeers, err := parseBootstrapPeers(c.Network.BootstrapPeers)
		if err != nil {
			fmt.Printf("Warning: failed to parse bootstrap peers: %v\n", err)
		} else {
			cfg.BootstrapPeers = bootstrapPeers
		}
	}

	return &cfg
}

// parseBootstrapPeers 解析 bootstrap 节点地址
func parseBootstrapPeers(peerStrs []string) ([]multiaddr.Multiaddr, error) {
	var peers []multiaddr.Multiaddr
	for _, peerStr := range peerStrs {
		peerStr = strings.TrimSpace(peerStr)
		if peerStr == "" {
			continue
		}

		m, err := multiaddr.NewMultiaddr(peerStr)
		if err != nil {
			return nil, fmt.Errorf("invalid multiaddr %q: %w", peerStr, err)
		}

		// 验证地址包含 peer ID
		_, err = peerInfoFromAddr(m)
		if err != nil {
			return nil, fmt.Errorf("invalid peer address %q: %w", peerStr, err)
		}

		peers = append(peers, m)
	}

	return peers, nil
}

// peerInfoFromAddr 从 multiaddr 提取 peer 信息
func peerInfoFromAddr(m multiaddr.Multiaddr) (peer.AddrInfo, error) {
	info, err := peer.AddrInfoFromP2pAddr(m)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	return *info, nil
}

// EnsureDirectories 确保必要的目录存在
func (c *Config) EnsureDirectories() error {
	// 创建 chunk 存储目录
	if err := os.MkdirAll(c.Storage.ChunkPath, 0755); err != nil {
		return fmt.Errorf("failed to create chunk directory: %w", err)
	}

	return nil
}

// GetConfigPath 获取配置文件路径
// 按优先级搜索：
// 1. 命令行指定的路径
// 2. 当前目录的 config.yaml
// 3. config/ 目录的 config.yaml
// 4. /etc/p2p-file-transfer/config.yaml
func GetConfigPath(cmdLinePath string) string {
	if cmdLinePath != "" {
		return cmdLinePath
	}

	// 检查可能的配置文件位置
	paths := []string{
		"config.yaml",
		filepath.Join("config", "config.yaml"),
		"/etc/p2p-file-transfer/config.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 返回默认路径
	return "config/config.yaml"
}
