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

	var selector PeerSelector
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
	rand.Seed(time.Now().UnixNano())
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

	resultCh := make(chan result, len(peers))
	for _, p := range peers {
		go func(p peer.ID) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
			defer cancel()
			stream, err := s.Host.NewStream(ctx, p, "/ping/1.0.0")
			if err != nil {
				resultCh <- result{p, 0, err}
				return
			}
			stream.Close()
			resultCh <- result{p, time.Since(start), nil}
		}(p)
	}

	var best peer.ID
	bestRTT := time.Hour

	for i := 0; i < len(peers); i++ {
		res := <-resultCh
		if res.err != nil {
			continue
		}
		if res.rtt < bestRTT {
			best = res.peer
			bestRTT = res.rtt
		}
	}

	if best == "" {
		return "", errors.New("no reachable peers")
	}
	return best, nil
}
