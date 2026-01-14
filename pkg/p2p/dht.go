// Package p2p 提供 DHT (分布式哈希表) 相关功能
//
// DHT 功能:
//   - 节点发现: 通过 Kademlia DHT 协议发现其他节点
//   - 内容路由: 存储和检索键值对
//   - Provider 系统: Chunk 提供者公告和查询
//   - 自定义协议: Announce 和 Lookup 协议
//
// 主要功能:
//   - Put: 在 DHT 中存储键值对
//   - Get: 从 DHT 中检索值
//   - Announce: 向网络公告自己是某个 Chunk 的提供者
//   - Lookup: 查找特定 Chunk 的提供者
//
// 协议定义:
//   - p2pFileTransfer/Announce/1.0.0: Chunk 公告协议
//   - p2pFileTransfer/Lookup/1.0.0: 提供者查询协议
package p2p

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"io"
	"p2pFileTransfer/pkg/file"
	"sync"
	"time"
)

const (
	AnnounceProtocol = "p2pFileTransfer/Announce/1.0.0"
	LookupProtocol   = "p2pFileTransfer/Lookup/1.0.0"
	QueryMetaData    = "p2pFileTransfer/QueryMetaData/1.0.0"
	writeTimeout     = 5 * time.Second
	readTimeout      = 5 * time.Second
	ioTimeout        = 5 * time.Second
)

type announceMsg struct {
	ChunkHash string        `json:"chunk_hash"`
	PeerInfo  peer.AddrInfo `json:"peer_info"`
}

// lookupRequest is sent by clients to ask for providers of a given key.
type lookupRequest struct {
	Key string `json:"key"`
}

// lookupResponse is sent by handlers listing providers (may be empty).
type lookupResponse struct {
	Providers []peer.AddrInfo `json:"providers"`
}

// newDHT 创建一个 DHT 实例
// 参数:
//   - ctx: 上下文，用于控制生命周期
//   - host: 主机实例
//   - config: DHT 配置
//
// 返回值:
//   - *dht.IpfsDHT: DHT 实例
//   - error: 错误信息
func newDHT(ctx context.Context, host host.Host, config P2PConfig) (*dht.IpfsDHT, error) {
	opts := []dht.Option{
		dht.ProtocolPrefix(protocol.ID(config.ProtocolPrefix)),
		dht.NamespacedValidator(config.NameSpace, config.Validator),
	}

	if !config.EnableAutoRefresh {
		opts = append(opts, dht.DisableAutoRefresh())
	}

	// 如果没有引导节点，以服务器模式 ModeServer 启动
	//if len(config.BootstrapPeers) == 0 {
	opts = append(opts, dht.Mode(dht.ModeServer))
	logrus.Infoln("Start node as a bootstrap server. MultiAddr: ", GetHostAddress(host))
	//} else {
	//	opts = append(opts, dht.Mode(dht.ModeClient))
	//	logrus.Infoln("Start node as a client.")
	//}

	// 生成一个 DHT 实例
	kdht, err := dht.New(ctx, host, opts...)
	if err != nil {
		return nil, err
	}

	// 启动 DHT 服务
	if err = kdht.Bootstrap(ctx); err != nil {
		return nil, err
	}

	if len(config.BootstrapPeers) == 0 {
		return kdht, nil
	}

	// 遍历引导节点数组并尝试连接
	successCount := 0
	for _, peerAddr := range config.BootstrapPeers {
		peerinfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			logrus.Warnf("Invalid bootstrap peer address %q: %v", peerAddr, err)
			continue
		}

		if err := host.Connect(ctx, *peerinfo); err != nil {
			logrus.Warnf("Error while connecting to bootstrap node %q: %v", peerinfo, err)
			continue
		}

		// 连接成功
		successCount++
		logrus.Infof("Connection established with bootstrap node: %q", peerinfo)

		// 添加到路由表
		if added, err := kdht.RoutingTable().TryAddPeer(peerinfo.ID, true, true); err != nil {
			logrus.Warnf("Failed to add peer %q to routing table: %v", peerinfo.ID, err)
		} else if added {
			logrus.Debugf("Peer %q added to routing table", peerinfo.ID)
		}

		peers := kdht.RoutingTable().ListPeers()
		logrus.Infof("RoutingTable size: %d", len(peers))
	}

	// 如果没有任何一个引导节点连接成功，返回错误
	if successCount == 0 {
		return kdht, fmt.Errorf("failed to connect to any bootstrap nodes (attempted %d)", len(config.BootstrapPeers))
	}

	logrus.Infof("Successfully connected to %d/%d bootstrap nodes", successCount, len(config.BootstrapPeers))
	return kdht, nil
}

// PutValue 向 DHT 中存储一个键值对
// 参数:
//   - ctx: 上下文，用于控制生命周期
//   - key: 键
//   - value: 值
//
// 返回值:
//   - error: 错误信息
func (d *P2PService) Put(ctx context.Context, key string, value []byte) error {
	key = "/" + d.Config.NameSpace + "/" + key
	err := d.DHT.PutValue(ctx, key, value)
	if err != nil {
		return xerrors.Errorf("failed to put value: %w", err)
	}
	logrus.Infof("Stored key-value pair: %s -> %s", key, value)
	return nil
}

// GetValue 从 DHT 中获取一个键值对
// 参数:
//   - ctx: 上下文，用于控制生命周期
//   - key: 键
//
// 返回值:
//   - string: 值
//   - error: 错误信息
func (d *P2PService) Get(ctx context.Context, key string) (string, error) {
	key = "/" + d.Config.NameSpace + "/" + key
	value, err := d.DHT.GetValue(ctx, key)
	if err != nil {
		return "", xerrors.Errorf("failed to get value: %w", err)
	}
	logrus.Infof("Retrieved value for key %s: %s", key, string(value))
	return string(value), nil
}

// TODO : 实现 QueryMetaData 方法
func (d *P2PService) QueryMetaData(ctx context.Context, key string) (*file.MetaData, error) {
	return nil, errors.New("QueryMetaData is not implemented yet")
}

func (d *P2PService) QueryMetaDataHandler() {
	// 未实现，保留占位符
}

// Announce 向网络中的节点宣布一个 fileInfo
// 参数:
//   - ctx: 上下文，用于控制生命周期
//   - fileInfo: 要宣布的 fileInfo
//
// 返回值:
//   - error: 错误信息
const (
	MaxAnnounceMessageSize = 1024 // 1KB
)

func (d *P2PService) Announce(ctx context.Context, chunkHash string) error {
	// 参数校验
	if len(chunkHash) == 0 {
		return errors.New("empty chunk hash")
	}

	// 使用带超时的 context 来获取 closest peers
	getPeersCtx, getPeersCancel := context.WithTimeout(ctx, 3*time.Second)
	defer getPeersCancel()

	// 使用通道接收结果（带缓冲，防止 goroutine 阻塞）
	resultChan := make(chan []peer.ID, 1)
	peers := []peer.ID{}

	go func() {
		closestPeers, err := d.DHT.GetClosestPeers(getPeersCtx, chunkHash)
		if err != nil {
			logrus.Debugf("GetClosestPeers failed: %v", err)
			closestPeers = []peer.ID{}
		}
		select {
		case resultChan <- closestPeers:
		case <-getPeersCtx.Done():
			// Context 已取消，不发送结果
			return
		}
	}()
	logrus.Info("getting closest peers...")
	select {
	case <-getPeersCtx.Done(): // 超时或取消
		if getPeersCtx.Err() == context.DeadlineExceeded {
			logrus.Warn("get closest peers timed out after 3 seconds, returning empty list")
		}
		peers = []peer.ID{} // 返回空切片
	case peers = <-resultChan: // 正常结果
		// channel 有缓冲，无需显式关闭
	}

	logrus.Infof("found %d closest peers", len(peers))

	// 如果没有找到peer，至少通知自己
	if len(peers) == 0 {
		// 添加到DHT提供者存储
		logrus.Info("no peers found, adding self as provider for chunk")
		if err := d.DHT.ProviderStore().AddProvider(ctx, []byte(chunkHash), peer.AddrInfo{
			ID:    d.Host.ID(),
			Addrs: d.Host.Addrs(),
		}); err != nil {
			logrus.WithFields(logrus.Fields{
				"chunk":    chunkHash,
				"provider": d.Host.ID(),
				"error":    err,
				"action":   "add_provider",
			}).Error("failed to add provider to DHT")
			return nil
		}
	}

	var (
		wg          sync.WaitGroup
		successChan = make(chan struct{}, len(peers))
		errChan     = make(chan error, len(peers))
		lowerCtx    context.Context
		cancel      context.CancelFunc
	)
	lowerCtx, cancel = context.WithCancel(ctx)
	defer cancel()

	// 对每个peer并行处理
	for _, p := range peers {
		wg.Add(1)
		go func(peerID peer.ID) {
			defer wg.Done()

			// 创建带超时的上下文
			peerCtx, peerCancel := context.WithTimeout(lowerCtx, writeTimeout)
			defer peerCancel()

			// 尝试建立流
			s, err := d.Host.NewStream(peerCtx, peerID, AnnounceProtocol)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"peer":   peerID,
					"error":  err,
					"action": "open_stream",
				}).Debug("failed to open stream to peer")
				return
			}
			defer s.Close()

			// 准备消息
			msg := announceMsg{
				ChunkHash: chunkHash,
				PeerInfo: peer.AddrInfo{
					ID:    d.Host.ID(),
					Addrs: d.Host.Addrs(),
				},
			}

			data, err := json.Marshal(msg)
			if err != nil {
				logrus.WithError(err).Error("failed to marshal announce message")
				return
			}
			data = append(data, '\n')

			// 设置写超时
			s.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err = s.Write(data); err != nil {
				logrus.WithFields(logrus.Fields{
					"peer":   peerID,
					"error":  err,
					"action": "write_message",
				}).Debug("failed to write announce message")
				return
			}

			// 成功发送
			successChan <- struct{}{}
			logrus.WithFields(logrus.Fields{
				"peer":   peerID,
				"chunk":  chunkHash,
				"action": "announce_success",
			}).Info("announced chunk to peer")
		}(p)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(successChan)
		close(errChan)
	}()

	// 检查是否有成功
	select {
	case <-successChan:
		return nil
	case <-lowerCtx.Done():
		return lowerCtx.Err()
	case err := <-errChan:
		return err
	}
}

func (d *P2PService) AnnounceHandler(ctx context.Context) {
	d.Host.SetStreamHandler(AnnounceProtocol, func(s network.Stream) {
		defer s.Close()

		// 检查服务是否已关闭
		select {
		case <-d.Ctx.Done():
			logrus.Debug("Service is shutting down, ignoring announce request")
			return
		default:
		}

		// 设置读超时
		s.SetReadDeadline(time.Now().Add(readTimeout))

		// 限制读取大小防止DoS攻击
		rdr := bufio.NewReader(io.LimitReader(s, MaxAnnounceMessageSize))
		line, err := rdr.ReadBytes('\n')
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"remotePeer": s.Conn().RemotePeer(),
				"error":      err,
				"action":     "read_message",
			}).Warn("failed to read announce message")
			return
		}

		// 解析消息
		var msg announceMsg
		if err := json.Unmarshal(bytes.TrimSpace(line), &msg); err != nil {
			logrus.WithFields(logrus.Fields{
				"remotePeer": s.Conn().RemotePeer(),
				"error":      err,
				"action":     "parse_message",
			}).Warn("invalid announce message format")
			return
		}

		// 验证消息
		if len(msg.ChunkHash) == 0 {
			logrus.WithField("remotePeer", s.Conn().RemotePeer()).Warn("received empty chunk hash")
			return
		}

		// 添加到DHT提供者存储
		if err := d.DHT.ProviderStore().AddProvider(ctx, []byte(msg.ChunkHash), msg.PeerInfo); err != nil {
			logrus.WithFields(logrus.Fields{
				"chunk":    msg.ChunkHash,
				"provider": msg.PeerInfo.ID,
				"error":    err,
				"action":   "add_provider",
			}).Error("failed to add provider to DHT")
			return
		}

		logrus.WithFields(logrus.Fields{
			"chunk":    msg.ChunkHash,
			"provider": msg.PeerInfo.ID,
			"action":   "provider_added",
		}).Info("added new provider for chunk")
	})
}

// Lookup streams a JSON request to each of the closest peers and returns
// the first non-empty provider list it finds.
func (d *P2PService) Lookup(ctx context.Context, key string) ([]peer.AddrInfo, error) {
	// 1. 获取最近的 peers（带 3s 超时）
	ctxGetPeers, cancelGetPeers := context.WithTimeout(ctx, 3*time.Second)
	defer cancelGetPeers()

	peers, err := d.DHT.GetClosestPeers(ctxGetPeers, key)
	if err != nil {
		if ctxGetPeers.Err() == context.DeadlineExceeded {
			logrus.Warn("get closest peers timed out after 3s")
		}
		return nil, fmt.Errorf("failed to get closest peers: %w", err)
	}
	logrus.WithField("count", len(peers)).Info("closest peers found")

	// 2. 准备请求数据（提前处理，避免每个 goroutine 重复处理）
	req := lookupRequest{Key: key}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}
	reqBytes = append(reqBytes, '\n')

	// 3. 并发查询所有 peers
	var (
		wg                      sync.WaitGroup
		resultChan              = make(chan []peer.AddrInfo, 1)           // 结果通道（缓冲防止 goroutine 泄漏）
		ctxLookup, cancelLookup = context.WithTimeout(ctx, 5*time.Second) // 总查询超时
	)
	defer cancelLookup()

	for _, p := range peers {
		wg.Add(1)
		go func(peerID peer.ID) {
			defer wg.Done()

			// 每个查询单独控制超时
			peerCtx, cancelPeer := context.WithTimeout(ctxLookup, ioTimeout)
			defer cancelPeer()

			// 尝试建立流
			s, err := d.Host.NewStream(peerCtx, peerID, LookupProtocol)
			if err != nil {
				logrus.WithFields(logrus.Fields{"peer": peerID, "err": err}).Warn("open stream failed")
				return
			}
			defer s.Close()

			// 设置读写截止时间
			deadline := time.Now().Add(ioTimeout)
			s.SetWriteDeadline(deadline)
			s.SetReadDeadline(deadline)

			// 发送请求
			if _, err := s.Write(reqBytes); err != nil {
				logrus.WithFields(logrus.Fields{"peer": peerID, "err": err}).Warn("write request failed")
				return
			}

			// 读取响应
			buf := bufio.NewReader(s)
			line, err := buf.ReadBytes('\n')
			if err != nil {
				logrus.WithFields(logrus.Fields{"peer": peerID, "err": err}).Warn("read response failed")
				return
			}

			// 解析响应
			var resp lookupResponse
			if err := json.Unmarshal(bytes.TrimSpace(line), &resp); err != nil {
				logrus.WithFields(logrus.Fields{"peer": peerID, "err": err}).Warn("invalid response JSON")
				return
			}

			// 返回有效结果（非阻塞方式）
			if len(resp.Providers) > 0 {
				select {
				case resultChan <- resp.Providers: // 只发送第一个有效结果
				default:
				}
			}
		}(p)
	}

	// 4. 等待结果或超时
	go func() {
		wg.Wait()
		close(resultChan) // 所有 goroutine 结束后关闭通道
	}()

	select {
	case providers := <-resultChan:
		logrus.WithField("providers", len(providers)).Info("providers found")
		return providers, nil
	case <-ctxLookup.Done():
		logrus.Warn("lookup timed out")
		return nil, errors.New("lookup timed out")
	}
}

// LookupHandler registers the JSON-based handler for lookupProtocol.
func (d *P2PService) LookupHandler(ctx context.Context) {
	d.Host.SetStreamHandler(LookupProtocol, func(s network.Stream) {
		defer s.Close()

		// 检查服务是否已关闭
		select {
		case <-d.Ctx.Done():
			logrus.Debug("Service is shutting down, ignoring lookup request")
			return
		default:
		}

		s.SetReadDeadline(time.Now().Add(ioTimeout))
		buf := bufio.NewReader(s)

		// read and parse request
		line, err := buf.ReadBytes('\n')
		if err != nil {
			logrus.WithError(err).Error("failed to read lookup request")
			return
		}
		var req lookupRequest
		if err := json.Unmarshal(bytes.TrimSpace(line), &req); err != nil {
			logrus.WithError(err).Error("invalid lookup request JSON")
			return
		}
		logrus.WithField("key", req.Key).Info("lookup request received")

		// find providers
		ps := d.DHT.ProviderStore()
		providers, err := ps.GetProviders(ctx, []byte(req.Key))
		if err != nil {
			logrus.WithError(err).Error("provider lookup failed")
			return
		}

		// build and send response
		resp := lookupResponse{Providers: providers}
		respBytes, err := json.Marshal(resp)
		if err != nil {
			logrus.WithError(err).Error("marshal lookup response failed")
			return
		}
		respBytes = append(respBytes, '\n')

		s.SetWriteDeadline(time.Now().Add(ioTimeout))
		if _, err := s.Write(respBytes); err != nil {
			logrus.WithError(err).Error("failed to write lookup response")
			return
		}
		logrus.WithField("providers", len(providers)).Info("lookup response sent")
	})
}

// addrInfosToMaddrs converts AddrInfo list to multiaddrs including peer IDs.
func addrInfosToMaddrs(infos []peer.AddrInfo) ([]multiaddr.Multiaddr, error) {
	var out []multiaddr.Multiaddr
	for _, ai := range infos {
		for _, m := range ai.Addrs {
			ma, err := multiaddr.NewMultiaddr(m.String() + "/p2p/" + ai.ID.String())
			if err != nil {
				logrus.WithError(err).Warn("failed to build multiaddr")
				continue
			}
			out = append(out, ma)
		}
	}
	return out, nil
}
