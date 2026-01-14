// Package p2p 提供 Chunk (数据块) 传输功能
//
// Chunk 功能:
//   - Chunk 存在性检查: 验证目标节点是否拥有特定 Chunk
//   - Chunk 数据下载: 从提供者节点下载 Chunk 数据
//   - 流式传输: 使用 32KB 缓冲区进行高效数据传输
//   - 超时控制: 支持请求超时和数据传输超时
//
// 协议定义:
//   - /p2pFileTransfer/getChunk/exists/1.0.0: Chunk 存在性检查协议
//   - /p2pFileTransfer/getChunk/data/1.0.0: Chunk 数据下载协议
//
// 常量配置:
//   - MaxChunkSize: 4MB (最大 Chunk 大小)
//   - ReadBufferSize: 32KB (读取缓冲区大小)
//   - DefaultDataTimeout: 30秒 (数据传输超时)
//
// 使用流程:
//   1. CheckChunkExists: 检查 Chunk 是否存在
//   2. DownloadChunk: 下载完整 Chunk 数据
//   3. 验证 SHA256 哈希确保数据完整性
package p2p

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// 协议名定义
	GetChunkExistProtocol = "/p2pFileTransfer/getChunk/exists/1.0.0"
	GetChunkDataProtocol  = "/p2pFileTransfer/getChunk/data/1.0.0"

	// 超时与限制（保留常量作为回退值）
	DefaultRequestTimeout = 5 * time.Second
	DefaultDataTimeout    = 30 * time.Second // 数据传输超时时间（更长）
	MaxChunkSize          = 4 * 1024 * 1024  // 4MB
	ReadBufferSize        = 32 * 1024        // 32KB 读取缓冲区
)

// 请求结构体
type RequestMessage struct {
	ChunkHash string `json:"chunkHash"`
}

// -----------------------------
// 客户端方法
// -----------------------------

// CheckChunkExists 确认指定 peer 是否拥有 chunk
func (p *P2PService) CheckChunkExists(ctx context.Context, peerID peer.ID, chunkHash string) (bool, error) {
	logrus.Debugf("Checking chunk existence on peer %s: %s", peerID, chunkHash)

	startTime := time.Now()

	// 检查是否允许创建流
	if !p.ConnManager.AcquireStream(peerID) {
		p.ConnManager.RecordFailure(peerID)
		return false, NewRetryableError(fmt.Errorf("peer %s has too many active streams", peerID))
	}
	defer p.ConnManager.ReleaseStream(peerID)

	// 为客户端操作添加超时控制
	requestTimeout := DefaultRequestTimeout
	if p.Config.RequestTimeout > 0 {
		requestTimeout = time.Duration(p.Config.RequestTimeout) * time.Second
	}
	clientCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	s, err := p.Host.NewStream(clientCtx, peerID, GetChunkExistProtocol)
	if err != nil {
		p.ConnManager.RecordFailure(peerID)
		return false, NewRetryableError(fmt.Errorf("open stream: %w", err))
	}
	defer s.Close()

	// 设置读写超时
	s.SetReadDeadline(time.Now().Add(requestTimeout))
	s.SetWriteDeadline(time.Now().Add(requestTimeout))

	req := RequestMessage{ChunkHash: chunkHash}
	if err := json.NewEncoder(s).Encode(req); err != nil {
		p.ConnManager.RecordFailure(peerID)
		return false, NewRetryableError(fmt.Errorf("encode request: %w", err))
	}

	var res string
	if err := json.NewDecoder(s).Decode(&res); err != nil {
		p.ConnManager.RecordFailure(peerID)
		return false, NewRetryableError(fmt.Errorf("decode response: %w", err))
	}

	// 记录成功请求
	responseTime := time.Since(startTime)
	p.ConnManager.RecordSuccess(peerID, responseTime)

	logrus.Debugf("Chunk existence check response: %s (took %v)", res, responseTime)

	return res == "true", nil
}

// DownloadChunk 从指定 peer 下载 chunk 数据
func (p *P2PService) DownloadChunk(ctx context.Context, peerID peer.ID, chunkHash string) ([]byte, error) {
	startTime := time.Now()

	// 检查是否允许创建流
	if !p.ConnManager.AcquireStream(peerID) {
		p.ConnManager.RecordFailure(peerID)
		return nil, NewRetryableError(fmt.Errorf("peer %s has too many active streams", peerID))
	}
	defer p.ConnManager.ReleaseStream(peerID)

	// 为客户端操作添加超时控制
	dataTimeout := DefaultDataTimeout
	if p.Config.DataTimeout > 0 {
		dataTimeout = time.Duration(p.Config.DataTimeout) * time.Second
	}
	clientCtx, cancel := context.WithTimeout(ctx, dataTimeout)
	defer cancel()

	s, err := p.Host.NewStream(clientCtx, peerID, GetChunkDataProtocol)
	if err != nil {
		p.ConnManager.RecordFailure(peerID)
		return nil, NewRetryableError(fmt.Errorf("open stream: %w", err))
	}
	defer s.Close()

	// 设置读写超时
	s.SetReadDeadline(time.Now().Add(dataTimeout))
	s.SetWriteDeadline(time.Now().Add(dataTimeout))

	req := RequestMessage{ChunkHash: chunkHash}
	if err := json.NewEncoder(s).Encode(req); err != nil {
		p.ConnManager.RecordFailure(peerID)
		return nil, NewRetryableError(fmt.Errorf("encode request: %w", err))
	}

	// 使用缓冲区流式读取，避免一次性分配大内存
	buffer := make([]byte, 0, ReadBufferSize)
	chunkBuffer := make([]byte, ReadBufferSize)
	totalRead := int64(0)

	for {
		n, err := s.Read(chunkBuffer)
		if n > 0 {
			totalRead += int64(n)
			if totalRead > MaxChunkSize {
				p.ConnManager.RecordFailure(peerID)
				return nil, fmt.Errorf("chunk size exceeds limit: %d > %d", totalRead, MaxChunkSize)
			}
			buffer = append(buffer, chunkBuffer[:n]...)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			p.ConnManager.RecordFailure(peerID)
			return nil, NewRetryableError(fmt.Errorf("read chunk: %w", err))
		}
	}

	if totalRead == 0 {
		p.ConnManager.RecordFailure(peerID)
		return nil, fmt.Errorf("chunk is empty")
	}

	// 记录成功请求
	responseTime := time.Since(startTime)
	p.ConnManager.RecordSuccess(peerID, responseTime)

	logrus.Debugf("Chunk %s downloaded successfully (%d bytes, took %v)", chunkHash, totalRead, responseTime)
	return buffer, nil
}

// -----------------------------
// 服务端注册处理器
// -----------------------------

// RegisterChunkExistHandler 处理 chunk 存在性检查请求
func (p *P2PService) RegisterChunkExistHandler(ctx context.Context) {
	p.Host.SetStreamHandler(GetChunkExistProtocol, func(s network.Stream) {
		peerID := s.Conn().RemotePeer()
		defer s.Close()

		// 检查服务是否已关闭
		select {
		case <-p.Ctx.Done():
			logrus.Debug("Service is shutting down, ignoring chunk exist check")
			return
		default:
		}

		// 从配置获取超时时间，使用默认值作为回退
		requestTimeout := DefaultRequestTimeout
		if p.Config.RequestTimeout > 0 {
			requestTimeout = time.Duration(p.Config.RequestTimeout) * time.Second
		}

		// 设置读取超时，防止恶意节点长时间占用连接
		s.SetReadDeadline(time.Now().Add(requestTimeout))
		s.SetWriteDeadline(time.Now().Add(requestTimeout))

		var req RequestMessage
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			logrus.Errorf("Invalid exist check request from %s: %v", peerID, err)
			return
		}

		chunkHash := strings.TrimSpace(req.ChunkHash)
		filePath := filepath.Join(p.Config.ChunkStoragePath, chunkHash)
		fileInfo, err := os.Stat(filePath)
		exists := err == nil && fileInfo.Size() <= MaxChunkSize

		resp := "false"
		if exists {
			resp = "true"
			logrus.Debugf("Chunk %s exists, served to peer %s", chunkHash, peerID)
		}

		if err := json.NewEncoder(s).Encode(resp); err != nil {
			logrus.Errorf("Failed to send response to %s: %v", peerID, err)
			return
		}
	})
}

// RegisterChunkDataHandler 处理 chunk 数据传输请求
func (p *P2PService) RegisterChunkDataHandler(ctx context.Context) {
	p.Host.SetStreamHandler(GetChunkDataProtocol, func(s network.Stream) {
		defer s.Close()
		peerID := s.Conn().RemotePeer()

		// 检查服务是否已关闭
		select {
		case <-p.Ctx.Done():
			logrus.Debug("Service is shutting down, ignoring chunk data request")
			return
		default:
		}

		// 从配置获取超时时间，使用默认值作为回退
		requestTimeout := DefaultRequestTimeout
		dataTimeout := DefaultDataTimeout
		if p.Config.RequestTimeout > 0 {
			requestTimeout = time.Duration(p.Config.RequestTimeout) * time.Second
		}
		if p.Config.DataTimeout > 0 {
			dataTimeout = time.Duration(p.Config.DataTimeout) * time.Second
		}

		// 设置读取请求超时
		s.SetReadDeadline(time.Now().Add(requestTimeout))

		if p.AntiLeecher.Refuse(ctx, peerID) {
			logrus.Warnf("Refused chunk data request from peer %s (leecher)", peerID)
			return
		}
		var req RequestMessage
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			logrus.Errorf("Invalid data request from %s: %v", peerID, err)
			return
		}

		filePath := filepath.Join(p.Config.ChunkStoragePath, req.ChunkHash)
		f, err := os.Open(filePath)
		if err != nil {
			logrus.Warnf("Chunk %s not found, requested by %s", req.ChunkHash, peerID)
			return
		}
		defer f.Close()

		// 为数据传输设置更长的超时时间
		s.SetWriteDeadline(time.Now().Add(dataTimeout))

		// 使用 buffered writer 优化性能
		bufferedWriter := bufio.NewWriterSize(s, ReadBufferSize)
		written, err := io.Copy(bufferedWriter, f)
		if err != nil {
			logrus.Errorf("Send chunk %s to %s failed: %v", req.ChunkHash, peerID, err)
			return
		}
		// 确保所有数据都刷新到网络
		if err := bufferedWriter.Flush(); err != nil {
			logrus.Errorf("Flush chunk data to %s failed: %v", peerID, err)
			return
		}

		logrus.Infof("Chunk %s (%d bytes) sent successfully to peer %s", req.ChunkHash, written, peerID)
	})
}
