// Package p2p 提供文件下载功能
//
// GetFile 功能:
//   - 顺序下载: 按顺序下载所有 Chunk
//   - 随机下载: 随机顺序下载 Chunk（适合并发）
//   - 进度报告: 实时报告下载进度
//   - 流式下载: 大文件流式处理，降低内存占用
//   - 错误重试: 智能重试机制，区分可重试和不可重试错误
//
// 核心机制:
//   - 工作池: 固定 16 个 worker，避免 goroutine 爆炸
//   - 指数退避: 重试延迟 500ms → 1s → 2s → 4s → 8s → 10s
//   - 错误分类: RetryableError 标记可重试的网络错误
//   - 连接管理: 使用 ConnManager 限制并发和统计性能
//
// 下载模式:
//   - GetFileOrdered: 顺序下载，保证 Chunk 顺序
//   - GetFileRandom: 随机下载，最大化并发效率
//   - GetFileOrderedWithProgress: 顺序下载 + 进度回调
//   - GetFileRandomWithProgress: 随机下载 + 进度回调
//
// 使用示例:
//
//	// 创建进度回调
//	progressCB := func(downloaded, total int64, chunkIndex, totalChunks int) {
//	    fmt.Printf("Progress: %d/%d bytes (%d/%d chunks)\n",
//	        downloaded, total, chunkIndex, totalChunks)
//	}
//
//	// 随机下载文件（推荐）
//	err := service.GetFileRandomWithProgress(ctx, fileHash, writer, progressCB)
//	if err != nil {
//	    return err
//	}
//
// 性能指标:
//   - 并发数: 16 个 Chunk 同时下载
//   - 内存占用: < 100MB (100MB 文件流式下载)
//   - Chunk 大小: 最大 4MB
package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"p2pFileTransfer/pkg/file"
	"sync"
	"time"
)

// 错误类型定义
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err error) error {
	return &RetryableError{Err: err}
}

// IsRetryable 判断错误是否可重试
// 可重试错误：网络超时、连接拒绝、临时网络故障等
// 不可重试错误：文件权限错误、磁盘空间不足、数据损坏等
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return true
	}

	// 检查底层错误类型
	targetErr := errors.Unwrap(err)
	if targetErr == nil {
		targetErr = err
	}

	// 网络相关错误通常可重试
	if errors.Is(targetErr, context.DeadlineExceeded) ||
	   errors.Is(targetErr, context.Canceled) {
		return true
	}

	// OS 错误检查
	var osErr *os.PathError
	if errors.As(targetErr, &osErr) {
		// 权限错误和文件不存在错误不可重试
		if osErr.Err == os.ErrPermission ||
		   errors.Is(osErr.Err, os.ErrExist) ||
		   errors.Is(osErr.Err, os.ErrNotExist) {
			return false
		}
	}

	// 默认情况下，未知错误可重试（保守策略）
	return true
}


const (
	// 保留常量用于回退值
	ConcurrentLimit   = 16
	MaxRetries        = 3
	DefaultDHTTimeout = 10 * time.Second // 默认 DHT 超时 10 秒
)

// 重试配置（从 P2PConfig 获取）
type retryConfig struct {
	maxRetries   int
	initialDelay time.Duration // 初始延迟
	maxDelay     time.Duration // 最大延迟
	multiplier   float64       // 延迟倍数
}

// ProgressCallback 下载进度回调函数
// downloaded: 已下载的字节数
// total: 总字节数
// chunkIndex: 当前完成的 chunk 索引
// totalChunks: 总 chunk 数
type ProgressCallback func(downloaded, total int64, chunkIndex, totalChunks int)

// downloadProgress 下载进度信息
type downloadProgress struct {
	downloaded      int64
	total           int64
	completedChunks int
	totalChunks     int
	callback        ProgressCallback
	mu              sync.Mutex
}

// addDownloaded 增加已下载字节数
func (dp *downloadProgress) addDownloaded(bytes int64) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.downloaded += bytes
	if dp.callback != nil {
		dp.callback(dp.downloaded, dp.total, dp.completedChunks, dp.totalChunks)
	}
}

// completeChunk 完成一个 chunk
func (dp *downloadProgress) completeChunk(chunkSize int64) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.completedChunks++
	dp.downloaded += chunkSize
	if dp.callback != nil {
		dp.callback(dp.downloaded, dp.total, dp.completedChunks, dp.totalChunks)
	}
}

// getRetryConfig 从配置获取重试配置
func (p *P2PService) getRetryConfig() *retryConfig {
	return &retryConfig{
		maxRetries:   p.Config.MaxRetries,
		initialDelay: 500 * time.Millisecond, // 初始 500ms
		maxDelay:     10 * time.Second,       // 最大 10s
		multiplier:   2.0,                    // 每次翻倍
	}
}

// calculateDelay 计算指数退避延迟
func (rc *retryConfig) calculateDelay(attempt int) time.Duration {
	// 使用整数运算进行指数增长
	multiplier := 1 << uint(attempt-1)
	delay := time.Duration(int64(rc.initialDelay) * int64(multiplier))
	if delay > rc.maxDelay {
		delay = rc.maxDelay
	}
	// 添加随机抖动，避免惊群效应
	jitter := time.Duration(float64(delay) * 0.1 * (randFloat64()*2 - 1))
	return delay + jitter
}

func randFloat64() float64 {
	// 简单的随机数生成，实际可以使用更好的随机源
	return float64(time.Now().UnixNano()%1000) / 1000.0
}

func (p *P2PService) loadMetaData(ctx context.Context, fileHash string) (*file.MetaData, error) {
	metaInfo, err := p.Get(ctx, fileHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	var metaData file.MetaData
	if err := json.Unmarshal([]byte(metaInfo), &metaData); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	logrus.Infof("Metadata parsed: FileName=%s, FileSize=%d, Leaves=%d chunks",
		metaData.FileName, metaData.FileSize, len(metaData.Leaves))

	return &metaData, nil
}

func (p *P2PService) downloadChunksConcurrently(
	ctx context.Context,
	leaves []file.ChunkData,
	concurrency int,
	handleChunk func(i int, chunk file.ChunkData, offset int64, data []byte) error,
	progress *downloadProgress,
) error {
	errCh := make(chan error, len(leaves))

	// 使用配置的并发数，如果传入的 concurrency 为 0，则使用配置的值
	if concurrency <= 0 {
		concurrency = p.Config.MaxConcurrency
		if concurrency <= 0 {
			concurrency = ConcurrentLimit
		}
	}

	retryCfg := p.getRetryConfig()

	// 使用工作池模式，限制 goroutine 创建数量
	type chunkTask struct {
		index       int
		chunk       file.ChunkData
		offset      int64
		chunkHashStr string
	}

	// 创建任务通道
	taskCh := make(chan chunkTask, len(leaves))
	// 启动 worker goroutines（固定数量）
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskCh {
				// 检查 context 是否已取消
				select {
				case <-ctx.Done():
					errCh <- fmt.Errorf("chunk %d: context canceled", task.index)
					continue
				default:
				}

				// 为 DHT 操作添加超时控制
				dhtTimeout := DefaultDHTTimeout
				if p.Config.DHTTimeout > 0 {
					dhtTimeout = time.Duration(p.Config.DHTTimeout) * time.Second
				}
				dhtCtx, cancel := context.WithTimeout(ctx, dhtTimeout)
				defer cancel()

				peers, err := p.DHT.GetClosestPeers(dhtCtx, task.chunkHashStr)
				if err != nil {
					if dhtCtx.Err() == context.DeadlineExceeded {
						errCh <- fmt.Errorf("chunk %d: get peers timed out after %v", task.index, dhtTimeout)
					} else {
						errCh <- fmt.Errorf("chunk %d: get peers failed: %w", task.index, err)
					}
					continue
				}
				if len(peers) == 0 {
					errCh <- fmt.Errorf("chunk %d: no peers found", task.index)
					continue
				}

				// 带指数退避的重试逻辑
				var lastErr error
				for attempt := 0; attempt < retryCfg.maxRetries; attempt++ {
					if attempt > 0 {
						// 检查错误是否可重试
						if !IsRetryable(lastErr) {
							logrus.Warnf("Chunk %d encountered non-retryable error: %v", task.index, lastErr)
							break
						}
						delay := retryCfg.calculateDelay(attempt)
						logrus.Warnf("Retrying chunk %d after %v (attempt %d/%d)",
							task.index, delay, attempt, retryCfg.maxRetries)
						time.Sleep(delay)
					}

					// 从可用 peers 中选择一个（失败后排除，尝试下一个）
					availablePeers := make([]peer.ID, len(peers))
					copy(availablePeers, peers)
					var selectedPeer peer.ID

					for len(availablePeers) > 0 {
						// 选择一个 peer
						selectedPeer, err = p.PeerSelector.SelectPeer(availablePeers)
						if err != nil {
							lastErr = fmt.Errorf("chunk %d: select peer failed: %w", task.index, err)
							break
						}

						// 验证该 peer 是否拥有 chunk
						hasChunk, err := p.CheckChunkExists(ctx, selectedPeer, task.chunkHashStr)
						if err != nil {
							logrus.Warnf("Peer %s check failed for chunk %d: %v", selectedPeer, task.index, err)
							lastErr = err
							// 移除该 peer，尝试下一个
							availablePeers = removePeer(availablePeers, selectedPeer)
							continue
						}
						if !hasChunk {
							// 移除该 peer，尝试下一个
							availablePeers = removePeer(availablePeers, selectedPeer)
							continue
						}

						// 下载 chunk
						chunkData, err := p.DownloadChunk(ctx, selectedPeer, task.chunkHashStr)
						if err != nil {
							logrus.Warnf("Download chunk %d from %s failed: %v", task.index, selectedPeer, err)
							lastErr = fmt.Errorf("chunk %d: download from peer %s failed: %w", task.index, selectedPeer, err)
							// 移除该 peer，尝试下一个
							availablePeers = removePeer(availablePeers, selectedPeer)
							continue
						}

						// 验证 chunk 哈希
						hash := sha256.Sum256(chunkData)
						if !bytesEqual(hash[:], task.chunk.ChunkHash) {
							logrus.Warnf("Chunk %d hash mismatch from peer %s", task.index, selectedPeer)
							lastErr = fmt.Errorf("chunk %d: hash validation failed", task.index)
							// 移除该 peer，尝试下一个
							availablePeers = removePeer(availablePeers, selectedPeer)
							continue
						}

						// 处理 chunk（写入文件等）
						if err := handleChunk(task.index, task.chunk, task.offset, chunkData); err != nil {
							// 这是本地错误（如写入文件），重试没有意义
							errCh <- fmt.Errorf("chunk %d: handle failed: %w", task.index, err)
							return
						}

						// 更新进度
						if progress != nil {
							progress.completeChunk(int64(task.chunk.ChunkSize))
						}

						// 成功，退出所有循环
						logrus.Debugf("Chunk %d downloaded successfully (attempt %d)", task.index, attempt+1)
						return
					}

					// 如果所有 peers 都试过了，跳出重试循环
					if len(availablePeers) == 0 {
						break
					}
				}

				// 所有重试都失败
				errCh <- lastErr
			}
		}()
	}

	// 提交所有任务
	var offset int64 = 0
	for i, chunk := range leaves {
		chunkOffset := offset
		offset += int64(chunk.ChunkSize)
		taskCh <- chunkTask{
			index:       i,
			chunk:       chunk,
			offset:      chunkOffset,
			chunkHashStr: string(chunk.ChunkHash),
		}
	}
	close(taskCh)

	// 等待所有 worker 完成
	wg.Wait()
	close(errCh)

	// 收集所有错误
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	// 如果有错误，返回聚合的错误信息
	if len(errors) > 0 {
		// 如果只有一个错误，直接返回
		if len(errors) == 1 {
			return errors[0]
		}
		// 多个错误，返回聚合错误
		return fmt.Errorf("%d chunks failed to download: %w (and %d more errors)",
			len(errors), errors[0], len(errors)-1)
	}
	return nil
}

// bytesEqual 比较两个字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (p *P2PService) GetFileOrdered(ctx context.Context, fileHash string, f io.ReadWriter) error {
	return p.GetFileOrderedWithProgress(ctx, fileHash, f, nil)
}

func (p *P2PService) GetFileRandom(ctx context.Context, fileHash string, f io.ReadWriter) error {
	return p.GetFileRandomWithProgress(ctx, fileHash, f, nil)
}

// GetFileOrderedWithProgress 下载文件（顺序写入）并支持进度回调
func (p *P2PService) GetFileOrderedWithProgress(ctx context.Context, fileHash string, f io.ReadWriter, progressCB ProgressCallback) error {
	metaData, err := p.loadMetaData(ctx, fileHash)
	if err != nil {
		return err
	}

	// 创建进度跟踪器
	var progress *downloadProgress
	if progressCB != nil {
		progress = &downloadProgress{
			total:        int64(metaData.FileSize),
			totalChunks:  len(metaData.Leaves),
			callback:     progressCB,
		}
	}

	// 使用 0 让 downloadChunksConcurrently 自动使用配置的并发数
	results := make([][]byte, len(metaData.Leaves))
	err = p.downloadChunksConcurrently(ctx, metaData.Leaves, 0, func(i int, chunk file.ChunkData, _ int64, data []byte) error {
		results[i] = data
		return nil
	}, progress)
	if err != nil {
		return err
	}

	for i, data := range results {
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write chunk %d failed: %w", i, err)
		}
	}
	logrus.Infof("File %s downloaded successfully (ordered)", metaData.FileName)
	return nil
}

// GetFileRandomWithProgress 下载文件（随机位置写入）并支持进度回调
func (p *P2PService) GetFileRandomWithProgress(ctx context.Context, fileHash string, f io.ReadWriter, progressCB ProgressCallback) error {
	writeAtFile, ok := f.(io.WriterAt)
	if !ok {
		return fmt.Errorf("target writer does not support WriterAt")
	}

	metaData, err := p.loadMetaData(ctx, fileHash)
	if err != nil {
		return err
	}

	// 创建进度跟踪器
	var progress *downloadProgress
	if progressCB != nil {
		progress = &downloadProgress{
			total:        int64(metaData.FileSize),
			totalChunks:  len(metaData.Leaves),
			callback:     progressCB,
		}
	}

	// 使用 0 让 downloadChunksConcurrently 自动使用配置的并发数
	err = p.downloadChunksConcurrently(ctx, metaData.Leaves, 0, func(i int, chunk file.ChunkData, offset int64, data []byte) error {
		if _, err := writeAtFile.WriteAt(data, offset); err != nil {
			return fmt.Errorf("chunk %d: write failed at offset %d: %w", i, offset, err)
		}
		return nil
	}, progress)
	if err != nil {
		return err
	}

	logrus.Infof("File %s downloaded successfully (random)", metaData.FileName)
	return nil
}

// DownloadFileStreaming 流式下载文件，直接写入目标文件，适合大文件
// 此方法不在内存中保存所有 chunks，而是在下载完每个 chunk 后立即写入
func (p *P2PService) DownloadFileStreaming(
	ctx context.Context,
	fileHash string,
	target io.WriterAt,
	progressCB ProgressCallback,
) error {
	metaData, err := p.loadMetaData(ctx, fileHash)
	if err != nil {
		return err
	}

	// 创建进度跟踪器
	var progress *downloadProgress
	if progressCB != nil {
		progress = &downloadProgress{
			total:        int64(metaData.FileSize),
			totalChunks:  len(metaData.Leaves),
			callback:     progressCB,
		}
	}

	// 使用 0 让 downloadChunksConcurrently 自动使用配置的并发数
	return p.downloadChunksConcurrently(ctx, metaData.Leaves, 0, func(i int, chunk file.ChunkData, offset int64, data []byte) error {
		// 直接写入目标文件，不在内存中缓存
		if _, err := target.WriteAt(data, offset); err != nil {
			return fmt.Errorf("write chunk %d at offset %d failed: %w", i, offset, err)
		}
		return nil
	}, progress)
}
