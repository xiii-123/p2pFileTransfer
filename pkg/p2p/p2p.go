// Package p2p 提供了基于 libp2p 的点对点文件传输功能
//
// 核心功能:
//   - DHT (分布式哈希表): 用于节点发现和内容路由
//   - Chunk 传输: 将文件分块并在 P2P 网络中传输
//   - 连接管理: 管理节点连接、统计和黑名单
//   - 节点选择: 支持随机和轮询两种节点选择策略
//   - 反吸血虫: 防止只下载不上传的节点
//
// 主要组件:
//   - P2PService: 核心服务，整合所有功能
//   - ConnManager: 连接管理器，限制并发连接数
//   - PeerSelector: 节点选择器接口
//   - AntiLeecher: 反吸血虫机制
//
// 使用示例:
//
//	config := p2p.NewP2PConfig()
//	config.Port = 0  // 随机端口
//	config.ChunkStoragePath = "/tmp/chunks"
//
//	service, err := p2p.NewP2PService(context.Background(), config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer service.Shutdown()
package p2p

import (
	"context"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"p2pFileTransfer/pkg/file"
	"time"
)

// 默认的 ProtocolPrefix 和 Validator 配置
var defaultPrefix = "/default"

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

type P2PService struct {
	Host         host.Host
	DHT          *dht.IpfsDHT
	Config       *P2PConfig
	PeerSelector PeerSelector
	AntiLeecher  AntiLeecher
	FSAdapter    file.LocalFileSystemAdapter
	ConnManager  *ConnManager // 连接管理器
	Ctx          context.Context     // 服务上下文，用于优雅关闭
	Cancel       context.CancelFunc  // 取消函数
}

type P2PConfig struct {
	Port              int
	Insecure          bool
	Seed              int64
	BootstrapPeers    []multiaddr.Multiaddr
	ProtocolPrefix    string
	EnableAutoRefresh bool
	NameSpace         string
	Validator         record.Validator
	ChunkStoragePath  string  // Chunk 文件存储路径
	MaxRetries        int     // 最大重试次数
	MaxConcurrency    int     // 最大并发下载数
	RequestTimeout    int     // 请求超时时间（秒）
	DataTimeout       int     // 数据传输超时时间（秒）
	DHTTimeout        int     // DHT 操作超时时间（秒）
}

// NewP2PConfig 返回一个包含默认配置的 P2PConfig 实例
// 返回值:
//   - P2PConfig: 包含默认配置的 P2PConfig 实例
func NewP2PConfig() P2PConfig {
	return P2PConfig{
		// 此处Port设为0，即可随机分配一个端口；指定可能会导致端口占用，从而连接失败
		Port:              0,
		Insecure:          false,
		Seed:              0,
		ProtocolPrefix:    defaultPrefix,
		EnableAutoRefresh: true,
		NameSpace:         "v",
		Validator:         blankValidator{}, // 使用默认的 blankValidator
		ChunkStoragePath:  "files",          // 默认使用相对路径 ./files
		MaxRetries:        3,                // 默认重试3次
		MaxConcurrency:    16,               // 默认最大并发16
		RequestTimeout:    5,                // 默认请求超时5秒
		DataTimeout:       30,               // 默认数据传输超时30秒
		DHTTimeout:        10,               // 默认DHT操作超时10秒
	}
}

// NewP2PService 创建并启动 DHT 服务
// 参数:
//   - ctx: 上下文，用于控制生命周期
//   - config: DHT 配置
//
// 返回值:
//   - *P2PService: DHT 服务实例
//   - error: 错误信息
func NewP2PService(ctx context.Context, config P2PConfig) (*P2PService, error) {
	host, err := newBasicHost(config.Port, config.Insecure, config.Seed)
	if err != nil {
		return nil, xerrors.Errorf("failed to create host: %w", err)
	}

	kdht, err := newDHT(ctx, host, config)
	if err != nil {
		return nil, xerrors.Errorf("failed to create DHT instance: %w", err)
	}

	// 创建可取消的上下文用于服务生命周期管理
	serviceCtx, cancel := context.WithCancel(context.Background())

	p := &P2PService{
		Host:         host,
		DHT:          kdht,
		Config:       &config,
		PeerSelector: &RandomPeerSelector{},
		AntiLeecher:  &DefaultAntiLeecher{},
		FSAdapter:    file.LocalFileSystemAdapter{},
		ConnManager:  NewConnManager(5, 10*time.Minute), // 每个节点最多5个并发流，黑名单超时10分钟
		Ctx:          serviceCtx,
		Cancel:       cancel,
	}
	p.AnnounceHandler(ctx)
	p.LookupHandler(ctx)
	p.RegisterChunkExistHandler(ctx)
	p.RegisterChunkDataHandler(ctx)
	return p, nil
}

func (p *P2PService) GetMaddr() []multiaddr.Multiaddr {
	return p.Host.Addrs()
}

// Shutdown 优雅关闭 P2P 服务
// 关闭 Host，取消所有正在进行的操作
func (p *P2PService) Shutdown() error {
	logrus.Info("Shutting down P2P service...")

	// 1. 取消服务上下文，通知所有 goroutine 退出
	if p.Cancel != nil {
		p.Cancel()
	}

	// 2. 关闭 libp2p Host（会关闭所有连接和监听器）
	if p.Host != nil {
		if err := p.Host.Close(); err != nil {
			logrus.Errorf("Error closing host: %v", err)
			return err
		}
	}

	logrus.Info("P2P service shutdown complete")
	return nil
}
