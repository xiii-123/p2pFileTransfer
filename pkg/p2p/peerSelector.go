// Package p2p 提供节点选择策略
//
// PeerSelector 功能:
//   - 定义节点选择器接口
//   - 随机选择: RandomPeerSelector
//   - 轮询选择: RoundRobinPeerSelector
//   - 可扩展: 支持自定义选择策略
//
// 选择策略:
//   - Random: 随机选择一个节点，适合均匀分布请求
//   - RoundRobin: 依次选择每个节点，适合负载均衡
//
// 使用示例:
//
//	selector := &RandomPeerSelector{}
//	selected, err := selector.SelectPeer(peers)
//	if err != nil {
//	    return err
//	}
//	// 使用 selected 节点进行传输
//
// 注意事项:
//   - Go 1.20+ 不需要手动调用 rand.Seed()
//   - 轮询选择器在多个 goroutine 使用时不保证顺序
package p2p

import (
	"context"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"math/rand"
	"sync"
	"time"
)

type PeerSelector interface {
	SelectPeer(peers []peer.ID) (peer.ID, error)
}

func (d *P2PService) SelectAvailablePeer(ctx context.Context, peers []peer.ID, chunkHash string) (peer.ID, error) {
	if len(peers) == 0 {
		return "", errors.New("no peers available")
	}

	// 创建副本，防止修改原 peers 切片
	availablePeers := append([]peer.ID(nil), peers...)

	selector := d.PeerSelector
	for len(availablePeers) > 0 {
		// 选择一个 peer
		selected, err := selector.SelectPeer(availablePeers)
		if err != nil {
			return "", fmt.Errorf("failed to select peer: %w", err)
		}

		// 验证该 peer 是否拥有 chunk
		ok, err := d.CheckChunkExists(ctx, selected, chunkHash)
		if err != nil {
			logrus.Warnf("Peer %s check failed for chunk %s: %v", selected, chunkHash, err)
		}
		if ok {
			return selected, nil
		}

		// 排除该 peer
		availablePeers = removePeer(availablePeers, selected)
	}

	return "", fmt.Errorf("no available peers found with chunk %s", chunkHash)
}

func removePeer(peers []peer.ID, target peer.ID) []peer.ID {
	result := make([]peer.ID, 0, len(peers)-1)
	for _, p := range peers {
		if p != target {
			result = append(result, p)
		}
	}
	return result
}

type RandomPeerSelector struct{}

func (s *RandomPeerSelector) SelectPeer(peers []peer.ID) (peer.ID, error) {
	if len(peers) == 0 {
		return "", errors.New("no peers available")
	}
	// Go 1.20+ 自动使用随机种子，不需要手动调用 rand.Seed
	return peers[rand.Intn(len(peers))], nil
}

type RoundRobinPeerSelector struct {
	mu    sync.Mutex
	index int
}

func (s *RoundRobinPeerSelector) SelectPeer(peers []peer.ID) (peer.ID, error) {
	if len(peers) == 0 {
		return "", errors.New("no peers available")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	selected := peers[s.index%len(peers)]
	s.index++
	return selected, nil
}

type LatencyBasedSelector struct {
	Host    host.Host
	Timeout time.Duration
}

func (s *LatencyBasedSelector) SelectPeer(peers []peer.ID) (peer.ID, error) {
	if len(peers) == 0 {
		return "", errors.New("no peers available")
	}

	type result struct {
		peer peer.ID
		rtt  time.Duration
		err  error
	}

	// 创建父 context 用于控制所有 goroutine
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout*time.Minute)
	defer cancel() // 确保在函数返回时取消所有 goroutine

	resultCh := make(chan result, len(peers))
	for _, p := range peers {
		go func(p peer.ID) {
			start := time.Now()
			// 为每个 goroutine 创建独立的超时 context
			pingCtx, pingCancel := context.WithTimeout(ctx, s.Timeout)
			defer pingCancel()

			stream, err := s.Host.NewStream(pingCtx, p, "/ping/1.0.0")
			if err != nil {
				select {
				case resultCh <- result{p, 0, err}:
				case <-ctx.Done():
					// 父 context 已取消，放弃发送结果
				}
				return
			}
			stream.Close()

			select {
			case resultCh <- result{p, time.Since(start), nil}:
			case <-ctx.Done():
				// 父 context 已取消，放弃发送结果
			}
		}(p)
	}

	var best peer.ID
	bestRTT := time.Hour

	// 收集所有结果或直到 context 取消
	for i := 0; i < len(peers); i++ {
		select {
		case res := <-resultCh:
			if res.err != nil {
				continue
			}
			if res.rtt < bestRTT {
				best = res.peer
				bestRTT = res.rtt
			}
		case <-ctx.Done():
			// 超时或取消，退出循环
			break
		}
	}

	if best == "" {
		return "", errors.New("no reachable peers")
	}
	return best, nil
}
