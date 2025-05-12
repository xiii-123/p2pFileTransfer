package p2p

import (
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

	// 超时与限制
	RequestTimeout = 5 * time.Second
	MaxChunkSize   = 4 * 1024 * 1024 // 4MB
)

const (
	FilePrefix = "D:\\Work-Files\\p2pFileTransfer\\files"
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
	logrus.Infof("Checking chunk existence on peer %s: %s", peerID, chunkHash)
	s, err := p.Host.NewStream(ctx, peerID, GetChunkExistProtocol)
	if err != nil {
		return false, fmt.Errorf("open stream: %w", err)
	}
	defer s.Close()

	req := RequestMessage{ChunkHash: chunkHash}
	if err := json.NewEncoder(s).Encode(req); err != nil {
		return false, fmt.Errorf("encode request: %w", err)
	}

	var res string
	if err := json.NewDecoder(s).Decode(&res); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}
	logrus.Infof("Chunk existence check response: %s", res)

	return res == "true", nil
}

// DownloadChunk 从指定 peer 下载 chunk 数据
func (p *P2PService) DownloadChunk(ctx context.Context, peerID peer.ID, chunkHash string) ([]byte, error) {
	s, err := p.Host.NewStream(ctx, peerID, GetChunkDataProtocol)
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	defer s.Close()

	req := RequestMessage{ChunkHash: chunkHash}
	if err := json.NewEncoder(s).Encode(req); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	limited := &io.LimitedReader{R: s, N: MaxChunkSize + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read chunk: %w", err)
	}
	if int64(len(data)) > MaxChunkSize {
		return nil, fmt.Errorf("chunk too large")
	}

	logrus.Infof("Chunk %s downloaded successfully (%d bytes)", chunkHash, len(data))
	return data, nil
}

// -----------------------------
// 服务端注册处理器
// -----------------------------

// RegisterChunkExistHandler 处理 chunk 存在性检查请求
func (p *P2PService) RegisterChunkExistHandler(ctx context.Context) {
	p.Host.SetStreamHandler(GetChunkExistProtocol, func(s network.Stream) {
		logrus.Infof("Received chunk existence check request from peer %s", s.Conn().RemotePeer())
		defer s.Close()
		var req RequestMessage
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			logrus.Errorf("Invalid exist check request: %v", err)
			return
		}

		chunkHash := strings.TrimSpace(req.ChunkHash)
		logrus.Infof("Checking chunk existence: %s", chunkHash)
		filePath := filepath.Join(FilePrefix, chunkHash)
		logrus.Infof("Checking file path: %s", filePath)
		fileInfo, err := os.Stat(filePath)
		exists := err == nil && fileInfo.Size() <= MaxChunkSize
		logrus.Infof("Chunk existence: %s, exists: %v", chunkHash, exists)

		resp := "false"
		if exists {
			resp = "true"
		}
		_ = json.NewEncoder(s).Encode(resp)
	})
}

// RegisterChunkDataHandler 处理 chunk 数据传输请求
func (p *P2PService) RegisterChunkDataHandler(ctx context.Context) {
	p.Host.SetStreamHandler(GetChunkDataProtocol, func(s network.Stream) {
		defer s.Close()
		peer := s.Conn().RemotePeer()
		logrus.Infof("Received chunk data request from peer %s", peer)
		if p.AntiLeecher.Refuse(ctx, peer) {
			logrus.Warnf("Refused chunk data request from peer %s", peer)
			return
		}
		var req RequestMessage
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			logrus.Errorf("Invalid data request: %v", err)
			return
		}
		filePath := filepath.Join(FilePrefix, req.ChunkHash)
		f, err := os.Open(filePath)
		if err != nil {
			logrus.Warnf("Chunk not found: %s", req.ChunkHash)
			return
		}
		defer f.Close()

		if _, err := io.Copy(s, f); err != nil {
			logrus.Errorf("Send chunk failed: %v", err)
		}
	})
}
