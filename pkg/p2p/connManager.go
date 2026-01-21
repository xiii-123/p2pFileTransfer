// Package p2p 提供连接管理功能
//
// ConnManager 功能:
//   - 并发控制: 限制每个节点的并发连接数
//   - 统计记录: 记录请求成功率、响应时间等
//   - 黑名单机制: 自动拉黑成功率低的节点
//   - 清理功能: 自动清理长时间未使用的节点信息
//
// 主要组件:
//   - ConnManager: 连接管理器核心
//   - PeerConnInfo: 节点连接信息和统计
//   - ConnStats: 连接统计数据
//
// 使用场景:
//   - 防止资源耗尽: 限制单个节点的并发流数量
//   - 负载均衡: 避免向故障节点发送请求
//   - 性能优化: 优先使用性能好的节点
//   - 自动恢复: 清理过期节点信息
//
// 黑名单策略:
//   - 成功率 < 50%
//   - 请求次数 >= 10
//   - 自动标记为黑名单节点
package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
)

// ConnStats 连接统计信息
type ConnStats struct {
	TotalRequests   int64
	SuccessfulReqs  int64
	FailedReqs      int64
	LastSuccessTime time.Time
	LastFailureTime time.Time
	AvgResponseTime time.Duration
}

// PeerConnInfo 节点连接信息
type PeerConnInfo struct {
	mu            sync.RWMutex
	stats         ConnStats
	activeStreams int
	isBlacklisted bool
}

// ConnManager 连接管理器
type ConnManager struct {
	mu               sync.RWMutex
	peerInfo         map[peer.ID]*PeerConnInfo
	maxStreams       int // 每个节点的最大并发流数
	blacklistTimeout time.Duration
}

// NewConnManager 创建连接管理器
func NewConnManager(maxStreams int, blacklistTimeout time.Duration) *ConnManager {
	return &ConnManager{
		peerInfo:         make(map[peer.ID]*PeerConnInfo),
		maxStreams:       maxStreams,
		blacklistTimeout: blacklistTimeout,
	}
}

// RecordSuccess 记录成功请求
func (cm *ConnManager) RecordSuccess(p peer.ID, responseTime time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	info, exists := cm.peerInfo[p]
	if !exists {
		info = &PeerConnInfo{}
		cm.peerInfo[p] = info
	}

	info.mu.Lock()
	defer info.mu.Unlock()

	info.stats.TotalRequests++
	info.stats.SuccessfulReqs++
	info.stats.LastSuccessTime = time.Now()

	// 更新平均响应时间（简单移动平均）
	if info.stats.AvgResponseTime == 0 {
		info.stats.AvgResponseTime = responseTime
	} else {
		info.stats.AvgResponseTime = (info.stats.AvgResponseTime*9 + responseTime) / 10
	}
}

// RecordFailure 记录失败请求
func (cm *ConnManager) RecordFailure(p peer.ID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	info, exists := cm.peerInfo[p]
	if !exists {
		info = &PeerConnInfo{}
		cm.peerInfo[p] = info
	}

	info.mu.Lock()
	defer info.mu.Unlock()

	info.stats.TotalRequests++
	info.stats.FailedReqs++
	info.stats.LastFailureTime = time.Now()
}

// AcquireStream 尝试获取流配额（原子操作，避免竞态条件）
func (cm *ConnManager) AcquireStream(p peer.ID) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 获取或创建节点信息（在锁内完成，避免并发创建）
	info, exists := cm.peerInfo[p]
	if !exists {
		info = &PeerConnInfo{}
		cm.peerInfo[p] = info
	}

	// 使用 info.mu 保护内部状态
	info.mu.Lock()
	defer info.mu.Unlock()

	// 检查是否在黑名单中
	if info.isBlacklisted {
		// 检查是否已经过了黑名单超时时间
		if time.Since(info.stats.LastFailureTime) > cm.blacklistTimeout {
			info.isBlacklisted = false
		} else {
			return false
		}
	}

	// 检查并发流限制
	if info.activeStreams >= cm.maxStreams {
		return false
	}

	info.activeStreams++
	return true
}

// ReleaseStream 释放流配额（原子操作）
func (cm *ConnManager) ReleaseStream(p peer.ID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	info, exists := cm.peerInfo[p]
	if !exists {
		return
	}

	info.mu.Lock()
	defer info.mu.Unlock()
	if info.activeStreams > 0 {
		info.activeStreams--
	}
}

// GetPeerStats 获取节点统计信息
func (cm *ConnManager) GetPeerStats(p peer.ID) *ConnStats {
	cm.mu.RLock()
	info, exists := cm.peerInfo[p]
	cm.mu.RUnlock()

	if !exists {
		return nil
	}

	info.mu.RLock()
	defer info.mu.RUnlock()

	statsCopy := info.stats
	return &statsCopy
}

// GetSuccessRate 获取节点成功率
func (cm *ConnManager) GetSuccessRate(p peer.ID) float64 {
	stats := cm.GetPeerStats(p)
	if stats == nil || stats.TotalRequests == 0 {
		return 0.0
	}
	return float64(stats.SuccessfulReqs) / float64(stats.TotalRequests)
}

// ShouldBlacklist 判断是否应该将节点加入黑名单
// 当失败率超过阈值且失败次数足够多时返回 true
func (cm *ConnManager) ShouldBlacklist(p peer.ID, threshold float64, minRequests int64) bool {
	// 在持有锁的情况下完成所有检查和修改，避免竞态条件
	cm.mu.Lock()
	defer cm.mu.Unlock()

	info, exists := cm.peerInfo[p]
	if !exists {
		return false
	}

	// 读取统计信息
	info.mu.RLock()
	statsCopy := info.stats
	totalRequests := statsCopy.TotalRequests
	successfulReqs := statsCopy.SuccessfulReqs
	info.mu.RUnlock()

	if totalRequests < minRequests {
		return false
	}

	successRate := float64(successfulReqs) / float64(totalRequests)
	if successRate < threshold {
		// 加入黑名单
		info.mu.Lock()
		info.isBlacklisted = true
		info.mu.Unlock()
		logrus.Warnf("Peer %s blacklisted due to low success rate: %.2f%% (%d/%d)",
			p, successRate*100, successfulReqs, totalRequests)
		return true
	}

	return false
}

// CleanupOldPeers 清理长时间未使用的节点信息
func (cm *ConnManager) CleanupOldPeers(maxIdleTime time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	for p, info := range cm.peerInfo {
		info.mu.RLock()
		lastActivity := info.stats.LastSuccessTime
		if info.stats.LastFailureTime.After(lastActivity) {
			lastActivity = info.stats.LastFailureTime
		}
		isActive := info.activeStreams > 0
		info.mu.RUnlock()

		// 如果节点长时间未活动且没有活跃流，则删除
		if !isActive && now.Sub(lastActivity) > maxIdleTime {
			delete(cm.peerInfo, p)
			logrus.Debugf("Cleaned up inactive peer: %s", p)
		}
	}
}

// GetTotalPeers 获取管理的节点总数
func (cm *ConnManager) GetTotalPeers() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.peerInfo)
}
