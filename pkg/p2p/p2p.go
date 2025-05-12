package p2p

import (
	"context"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/xerrors"
	"p2pFileTransfer/pkg/file"
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

	p := &P2PService{
		Host:         host,
		DHT:          kdht,
		Config:       &config,
		PeerSelector: &RandomPeerSelector{},
		AntiLeecher:  &DefaultAntiLeecher{},
		FSAdapter:    file.LocalFileSystemAdapter{},
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
