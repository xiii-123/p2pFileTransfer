// Package p2p 提供反吸血虫 (Anti-Leecher) 机制
//
// AntiLeecher 功能:
//   - 定义反吸血虫接口
//   - 节点拒绝: 判断是否拒绝为特定节点提供数据
//   - 可扩展: 支持自定义反吸血虫策略
//
// 使用场景:
//   - 防止免费搭车: 拒绝只下载不上传的节点
//   - 激励共享: 优先为贡献资源的节点提供服务
//   - 网络健康: 维护 P2P 网络的平衡性
//
// 实现策略:
//   - DefaultAntiLeecher: 默认实现，不拒绝任何节点
//   - 自定义实现: 可基于上传/下载比例、共享时间等
//
// 使用示例:
//
//	type RatioAntiLeecher struct {
//	    minRatio float64  // 最小上传/下载比例
//	}
//
//	func (r *RatioAntiLeecher) Refuse(ctx context.Context, peerID peer.ID) bool {
//	    stats := getPeerStats(peerID)
//	    return stats.UploadRatio() < r.minRatio
//	}
//
// 注意事项:
//   - 当前实现为空接口，需要根据实际需求完善
//   - 过于严格的策略可能影响网络可用性
//   - 建议结合 ConnManager 的统计信息
package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
)

type AntiLeecher interface {
	Refuse(ctx context.Context, peerID peer.ID) bool
}

type DefaultAntiLeecher struct{}

func (d *DefaultAntiLeecher) Refuse(ctx context.Context, peerID peer.ID) bool {
	return false
}
