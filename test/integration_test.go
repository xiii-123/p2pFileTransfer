// Package main 提供集成测试
//
// 测试范围:
//   - P2P 服务创建和配置
//   - DHT 存储和检索 (单节点环境)
//   - Chunk 传输功能
//   - 文件下载功能 (需要稳定 DHT)
//   - 并发下载测试 (需要稳定 DHT)
//   - 节点选择器测试
//   - 连接管理器测试
//   - 错误处理测试
//   - 服务优雅关闭测试
//
// 运行测试:
//   - go test -v ./test: 运行所有测试
//   - go test -v -run TestP2P: 运行 P2P 相关测试
//   - go test -v -short: 跳过需要多节点的测试
//
// 测试辅助函数:
//   - createTestFile: 创建测试文件
//   - createTestChunk: 创建测试 Chunk
//   - setupTestNodes: 创建测试节点
//
// 注意事项:
//   - 部分测试需要稳定的 DHT 连接
//   - 在 CI 环境中可能需要跳过某些测试
//   - 使用 t.TempDir() 自动清理临时文件
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"p2pFileTransfer/pkg/file"
	"p2pFileTransfer/pkg/p2p"
)

// 测试辅助函数
func createTestFile(t *testing.T, size int64) ([]byte, string) {
	t.Helper()

	// 创建测试数据
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// 创建临时文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile.bin")

	err := os.WriteFile(testFile, data, 0644)
	require.NoError(t, err)

	return data, testFile
}

func createTestChunk(t *testing.T, data []byte) string {
	t.Helper()

	// 计算 chunk hash
	hash := sha256.Sum256(data)
	chunkHash := fmt.Sprintf("%x", hash)

	// 创建 chunk 存储目录
	chunkDir := t.TempDir()

	// 保存 chunk
	chunkPath := filepath.Join(chunkDir, chunkHash)
	err := os.WriteFile(chunkPath, data, 0644)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(chunkDir)
	})

	return chunkHash
}

// setupTestNodes 创建测试用的 P2P 节点
func setupTestNodes(t *testing.T, numNodes int) ([]*p2p.P2PService, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	nodes := make([]*p2p.P2PService, numNodes)

	// 创建第一个节点（作为 bootstrap）
	config1 := p2p.NewP2PConfig()
	config1.Port = 0 // 随机端口
	config1.ChunkStoragePath = t.TempDir()

	node1, err := p2p.NewP2PService(ctx, config1)
	require.NoError(t, err)

	nodes[0] = node1

	// 等待第一个节点启动
	time.Sleep(500 * time.Millisecond)

	// 不使用 bootstrap 连接，而是让所有节点独立运行
	// 然后通过直接连接方式建立连接
	for i := 1; i < numNodes; i++ {
		config := p2p.NewP2PConfig()
		config.Port = 0
		config.ChunkStoragePath = t.TempDir()
		// 不设置 BootstrapPeers，让节点独立运行

		node, err := p2p.NewP2PService(ctx, config)
		require.NoError(t, err)

		nodes[i] = node

		// 等待节点启动
		time.Sleep(200 * time.Millisecond)

		// 手动连接到第一个节点
		ctxTimeout, cancelConnect := context.WithTimeout(ctx, 5*time.Second)
		defer cancelConnect()

		// 构造第一个节点的 peer info
		peerInfo := &peer.AddrInfo{
			ID:    node1.Host.ID(),
			Addrs: node1.Host.Addrs(),
		}

		// 尝试连接
		err = node.Host.Connect(ctxTimeout, *peerInfo)
		if err != nil {
			t.Logf("Node %d failed to connect to bootstrap: %v", i, err)
		} else {
			t.Logf("Node %d connected to bootstrap node", i)
		}
	}

	// 等待 DHT 稳定
	time.Sleep(2 * time.Second)

	cleanup := func() {
		for _, node := range nodes {
			_ = node.Shutdown()
		}
		cancel()
	}

	return nodes, cleanup
}

// TestP2PServiceCreation 测试 P2P 服务创建
func TestP2PServiceCreation(t *testing.T) {
	tests := []struct {
		name    string
		config  p2p.P2PConfig
		wantErr bool
	}{
		{
			name: "default config",
			config: p2p.NewP2PConfig(),
			wantErr: false,
		},
		{
			name: "custom port",
			config: func() p2p.P2PConfig {
				c := p2p.NewP2PConfig()
				c.Port = 0
				return c
			}(),
			wantErr: false,
		},
		{
			name: "custom timeouts",
			config: func() p2p.P2PConfig {
				c := p2p.NewP2PConfig()
				c.RequestTimeout = 10
				c.DataTimeout = 60
				c.DHTTimeout = 20
				return c
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			service, err := p2p.NewP2PService(ctx, tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, service)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, service)
				assert.NotNil(t, service.Host)
				assert.NotNil(t, service.DHT)
				assert.NotNil(t, service.ConnManager)

				_ = service.Shutdown()
			}
		})
	}
}

// TestDHTPutAndGet 测试 DHT 存储和获取
func TestDHTPutAndGet(t *testing.T) {
	t.Skip("DHT operations require multiple connected peers - skipping in single-node test environment")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建单个节点进行测试（不需要 bootstrap）
	config := p2p.NewP2PConfig()
	config.Port = 0
	config.ChunkStoragePath = t.TempDir()

	node, err := p2p.NewP2PService(ctx, config)
	require.NoError(t, err)
	defer node.Shutdown()

	// 等待节点启动
	time.Sleep(1 * time.Second)

	// 测试 Put 操作
	key := "test-key"
	value := []byte("test-value")

	err = node.Put(ctx, key, value)
	require.NoError(t, err)

	// 测试 Get 操作
	retrieved, err := node.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, string(value), retrieved)
}

// TestChunkExistenceCheck 测试 Chunk 存在性检查
func TestChunkExistenceCheck(t *testing.T) {
	t.Skip("DHT announce/lookup require multiple connected peers - skipping in single-node test environment")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建单个节点
	config := p2p.NewP2PConfig()
	config.Port = 0
	config.ChunkStoragePath = t.TempDir()

	node, err := p2p.NewP2PService(ctx, config)
	require.NoError(t, err)
	defer node.Shutdown()

	// 在节点上创建测试 chunk
	testData := []byte("hello world")
	chunkHash := createTestChunk(t, testData)

	// Announce 到 DHT
	err = node.Announce(ctx, chunkHash)
	require.NoError(t, err)

	// 等待传播
	time.Sleep(1 * time.Second)

	// Lookup 查询
	providers, err := node.Lookup(ctx, chunkHash)
	require.NoError(t, err)
	t.Logf("Found %d providers for chunk", len(providers))
}

// TestChunkDownload 测试 Chunk 下载
func TestChunkDownload(t *testing.T) {
	t.Skip("Chunk download requires DHT provider lookup - skipping in single-node test environment")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建单个节点
	config := p2p.NewP2PConfig()
	config.Port = 0
	config.ChunkStoragePath = t.TempDir()

	node, err := p2p.NewP2PService(ctx, config)
	require.NoError(t, err)
	defer node.Shutdown()

	// 在节点上创建测试 chunk
	testData := []byte("hello world, this is a test chunk")
	chunkHash := createTestChunk(t, testData)

	// 确保 chunk 在节点的存储目录中
	chunkPath := filepath.Join(node.Config.ChunkStoragePath, chunkHash)
	err = os.WriteFile(chunkPath, testData, 0644)
	require.NoError(t, err)

	// Announce 到 DHT
	err = node.Announce(ctx, chunkHash)
	require.NoError(t, err)

	// 等待传播
	time.Sleep(1 * time.Second)

	// 从自身节点下载（测试本地存储）
	providers, err := node.Lookup(ctx, chunkHash)
	require.NoError(t, err)

	if len(providers) > 0 {
		downloadedData, err := node.DownloadChunk(ctx, providers[0].ID, chunkHash)
		require.NoError(t, err)
		assert.Equal(t, testData, downloadedData)
	} else {
		t.Skip("No providers found, skipping download test")
	}
}

// TestFileDownloadOrdered 测试顺序文件下载
func TestFileDownloadOrdered(t *testing.T) {
	t.Skip("Skipping multi-node test in CI environment - requires stable DHT connection")

	nodes, cleanup := setupTestNodes(t, 2)
	defer cleanup()

	ctx := context.Background()

	// 创建测试文件元数据
	fileSize := int64(1024 * 100) // 100KB
	testData, _ := createTestFile(t, fileSize)

	// 分割文件为 chunks
	chunkSize := 1024 * 10 // 10KB per chunk
	var chunks []file.ChunkData

	for i := 0; i < len(testData); i += chunkSize {
		end := i + chunkSize
		if end > len(testData) {
			end = len(testData)
		}

		chunkData := testData[i:end]
		hash := sha256.Sum256(chunkData)

		// 保存 chunk
		chunkHash := fmt.Sprintf("%x", hash[:])
		chunkPath := filepath.Join(nodes[0].Config.ChunkStoragePath, chunkHash)
		err := os.WriteFile(chunkPath, chunkData, 0644)
		require.NoError(t, err)

		chunks = append(chunks, file.ChunkData{
			ChunkSize: len(chunkData),
			ChunkHash: hash[:],
		})
	}

	// 创建文件元数据
	metaData := &file.MetaData{
		FileName:   "test.bin",
		FileSize:   uint64(fileSize),
		Leaves:     chunks,
	}

	// 序列化元数据
	metaJSON, err := json.Marshal(metaData)
	require.NoError(t, err)

	// 存储元数据到 DHT
	fileHash := fmt.Sprintf("%x", sha256.Sum256(metaJSON))
	err = nodes[0].Put(ctx, fileHash, metaJSON)
	require.NoError(t, err)

	// 发布所有 chunks
	for _, chunk := range chunks {
		chunkHashStr := fmt.Sprintf("%x", chunk.ChunkHash)
		err = nodes[0].Announce(ctx, chunkHashStr)
		require.NoError(t, err)
	}

	// 等待传播
	time.Sleep(3 * time.Second)

	// 从第二个节点下载
	outputFile := filepath.Join(t.TempDir(), "downloaded.bin")
	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()

	err = nodes[1].GetFileOrdered(ctx, fileHash, f)
	require.NoError(t, err)

	// 验证下载的文件
	downloadedData, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, testData, downloadedData, "Downloaded data should match original")
}

// TestFileDownloadWithProgress 测试带进度报告的文件下载
func TestFileDownloadWithProgress(t *testing.T) {
	t.Skip("Skipping multi-node test in CI environment - requires stable DHT connection")

	nodes, cleanup := setupTestNodes(t, 2)
	defer cleanup()

	ctx := context.Background()

	// 创建测试文件
	fileSize := int64(1024 * 50) // 50KB
	testData, _ := createTestFile(t, fileSize)

	// 分割文件为 chunks
	chunkSize := 1024 * 10 // 10KB per chunk
	var chunks []file.ChunkData

	for i := 0; i < len(testData); i += chunkSize {
		end := i + chunkSize
		if end > len(testData) {
			end = len(testData)
		}

		chunkData := testData[i:end]
		hash := sha256.Sum256(chunkData)

		// 保存 chunk
		chunkHash := fmt.Sprintf("%x", hash[:])
		chunkPath := filepath.Join(nodes[0].Config.ChunkStoragePath, chunkHash)
		err := os.WriteFile(chunkPath, chunkData, 0644)
		require.NoError(t, err)

		chunks = append(chunks, file.ChunkData{
			ChunkSize: len(chunkData),
			ChunkHash: hash[:],
		})
	}

	// 创建文件元数据
	metaData := &file.MetaData{
		FileName:   "test.bin",
		FileSize:   uint64(fileSize),
		Leaves:     chunks,
	}

	metaJSON, err := json.Marshal(metaData)
	require.NoError(t, err)

	fileHash := fmt.Sprintf("%x", sha256.Sum256(metaJSON))
	err = nodes[0].Put(ctx, fileHash, metaJSON)
	require.NoError(t, err)

	// 发布所有 chunks
	for _, chunk := range chunks {
		chunkHashStr := fmt.Sprintf("%x", chunk.ChunkHash)
		err = nodes[0].Announce(ctx, chunkHashStr)
		require.NoError(t, err)
	}

	// 等待传播
	time.Sleep(3 * time.Second)

	// 从第二个节点下载（带进度）
	outputFile := filepath.Join(t.TempDir(), "downloaded.bin")
	f, err := os.Create(outputFile)
	require.NoError(t, err)
	defer f.Close()

	progressUpdates := 0
	var lastProgress int64

	err = nodes[1].GetFileOrderedWithProgress(ctx, fileHash, f,
		func(downloaded, total int64, chunkIndex, totalChunks int) {
			progressUpdates++
			lastProgress = downloaded
			t.Logf("Progress: %d/%d bytes, chunk %d/%d", downloaded, total, chunkIndex, totalChunks)
		})

	require.NoError(t, err)
	assert.Greater(t, progressUpdates, 0, "Should receive progress updates")
	assert.Equal(t, fileSize, lastProgress, "Final progress should match file size")

	// 验证文件内容
	downloadedData, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, testData, downloadedData)
}

// TestFileDownloadStreaming 测试流式文件下载
func TestFileDownloadStreaming(t *testing.T) {
	t.Skip("Skipping multi-node test in CI environment - requires stable DHT connection")

	nodes, cleanup := setupTestNodes(t, 2)
	defer cleanup()

	ctx := context.Background()

	// 创建较大的测试文件
	fileSize := int64(1024 * 200) // 200KB
	testData, _ := createTestFile(t, fileSize)

	// 分割文件为 chunks
	chunkSize := 1024 * 10 // 10KB per chunk
	var chunks []file.ChunkData

	for i := 0; i < len(testData); i += chunkSize {
		end := i + chunkSize
		if end > len(testData) {
			end = len(testData)
		}

		chunkData := testData[i:end]
		hash := sha256.Sum256(chunkData)

		// 保存 chunk
		chunkHash := fmt.Sprintf("%x", hash[:])
		chunkPath := filepath.Join(nodes[0].Config.ChunkStoragePath, chunkHash)
		err := os.WriteFile(chunkPath, chunkData, 0644)
		require.NoError(t, err)

		chunks = append(chunks, file.ChunkData{
			ChunkSize: len(chunkData),
			ChunkHash: hash[:],
		})
	}

	// 创建文件元数据
	metaData := &file.MetaData{
		FileName:   "test.bin",
		FileSize:   uint64(fileSize),
		Leaves:     chunks,
	}

	metaJSON, err := json.Marshal(metaData)
	require.NoError(t, err)

	fileHash := fmt.Sprintf("%x", sha256.Sum256(metaJSON))
	err = nodes[0].Put(ctx, fileHash, metaJSON)
	require.NoError(t, err)

	// 发布所有 chunks
	for _, chunk := range chunks {
		chunkHashStr := fmt.Sprintf("%x", chunk.ChunkHash)
		err = nodes[0].Announce(ctx, chunkHashStr)
		require.NoError(t, err)
	}

	// 等待传播
	time.Sleep(3 * time.Second)

	// 流式下载
	outputFile := filepath.Join(t.TempDir(), "streamed.bin")
	f, err := os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	defer f.Close()

	err = nodes[1].DownloadFileStreaming(ctx, fileHash, f,
		func(downloaded, total int64, chunkIndex, totalChunks int) {
			t.Logf("Streaming progress: %d/%d bytes (%.2f%%)",
				downloaded, total, float64(downloaded)/float64(total)*100)
		})

	require.NoError(t, err)

	// 验证文件
	downloadedData, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, testData, downloadedData)
}

// TestConcurrentDownloads 测试并发下载
func TestConcurrentDownloads(t *testing.T) {
	t.Skip("Skipping multi-node test in CI environment - requires stable DHT connection")

	nodes, cleanup := setupTestNodes(t, 3)
	defer cleanup()

	ctx := context.Background()

	// 创建多个测试文件
	numFiles := 5
	fileSize := int64(1024 * 20) // 20KB per file

	type fileData struct {
		hash     string
		testData []byte
	}

	files := make([]fileData, numFiles)

	for i := 0; i < numFiles; i++ {
		testData := make([]byte, fileSize)
		for j := range testData {
			testData[j] = byte((i + j) % 256)
		}

		// 分割为 chunks
		chunkSize := 1024 * 10
		var chunks []file.ChunkData

		for k := 0; k < len(testData); k += chunkSize {
			end := k + chunkSize
			if end > len(testData) {
				end = len(testData)
			}

			chunkData := testData[k:end]
			hash := sha256.Sum256(chunkData)

			chunkHash := fmt.Sprintf("%x", hash[:])
			chunkPath := filepath.Join(nodes[0].Config.ChunkStoragePath, chunkHash)
			err := os.WriteFile(chunkPath, chunkData, 0644)
			require.NoError(t, err)

			chunks = append(chunks, file.ChunkData{
				ChunkSize: len(chunkData),
				ChunkHash: hash[:],
			})
		}

		// 创建元数据
		metaData := &file.MetaData{
			FileName:   fmt.Sprintf("file%d.bin", i),
			FileSize:   uint64(fileSize),
			Leaves:     chunks,
		}

		metaJSON, _ := json.Marshal(metaData)
		fileHash := fmt.Sprintf("%x", sha256.Sum256(metaJSON))

		err := nodes[0].Put(ctx, fileHash, metaJSON)
		require.NoError(t, err)

		// 发布 chunks
		for _, chunk := range chunks {
			chunkHashStr := fmt.Sprintf("%x", chunk.ChunkHash)
			err = nodes[0].Announce(ctx, chunkHashStr)
			require.NoError(t, err)
		}

		files[i] = fileData{
			hash:     fileHash,
			testData: testData,
		}
	}

	// 等待传播
	time.Sleep(3 * time.Second)

	// 并发下载所有文件
	errChan := make(chan error, numFiles)

	for i, file := range files {
		go func(idx int, fh string, data []byte) {
			outputPath := filepath.Join(t.TempDir(), fmt.Sprintf("downloaded%d.bin", idx))
			f, err := os.Create(outputPath)
			if err != nil {
				errChan <- err
				return
			}
			defer f.Close()

			err = nodes[1].GetFileOrdered(ctx, fh, f)
			if err != nil {
				errChan <- err
				return
			}

			// 验证
			downloaded, err := os.ReadFile(outputPath)
			if err != nil {
				errChan <- err
				return
			}

			if string(downloaded) != string(data) {
				errChan <- fmt.Errorf("file %d data mismatch", idx)
				return
			}

			errChan <- nil
		}(i, file.hash, file.testData)
	}

	// 收集结果
	for i := 0; i < numFiles; i++ {
		err := <-errChan
		assert.NoError(t, err, fmt.Sprintf("File %d should download successfully", i))
	}

	t.Log("All files downloaded successfully")
}

// TestPeerSelector 测试节点选择器
func TestPeerSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector p2p.PeerSelector
	}{
		{"random selector", &p2p.RandomPeerSelector{}},
		{"round-robin selector", &p2p.RoundRobinPeerSelector{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试 peer IDs
			peers := []peer.ID{
				peer.ID("QmYXt4nUjUEV2qihnVsN38iuMHnKvAKKrXhVCN1KLz7beH"),
				peer.ID("QmZ4tf3R7aHJtsfgdTSQhBmhCyuBuvAbRoxWaD9HsQhfi"),
				peer.ID("QmesqkFZtpDAN61Doixfk4whEaxHTGnCp2dE397wBfHf1n"),
			}

			// 测试选择
			selected, err := tt.selector.SelectPeer(peers)
			assert.NoError(t, err)
			assert.Contains(t, peers, selected)

			// 多次选择应该都返回有效 peer
			for i := 0; i < 10; i++ {
				selected, err := tt.selector.SelectPeer(peers)
				assert.NoError(t, err)
				assert.Contains(t, peers, selected)
			}
		})
	}

	t.Run("empty peers", func(t *testing.T) {
		selector := &p2p.RandomPeerSelector{}
		_, err := selector.SelectPeer([]peer.ID{})
		assert.Error(t, err)
	})
}

// TestConnManager 测试连接管理器
func TestConnManager(t *testing.T) {
	cm := p2p.NewConnManager(3, 10*time.Second)

	peer1 := peer.ID("QmYXt4nUjUEV2qihnVsN38iuMHnKvAKKrXhVCN1KLz7beH")
	peer2 := peer.ID("QmZ4tf3R7aHJtsfgdTSQhBmhCyuBuvAbRoxWaD9HsQhfi")

	t.Run("acquire and release streams", func(t *testing.T) {
		// 应该能够获取多个流
		assert.True(t, cm.AcquireStream(peer1))
		assert.True(t, cm.AcquireStream(peer1))
		assert.True(t, cm.AcquireStream(peer1))

		// 超过限制
		assert.False(t, cm.AcquireStream(peer1))

		// 释放一个
		cm.ReleaseStream(peer1)
		assert.True(t, cm.AcquireStream(peer1))

		// 清理
		cm.ReleaseStream(peer1)
		cm.ReleaseStream(peer1)
		cm.ReleaseStream(peer1)
	})

	t.Run("record success and failure", func(t *testing.T) {
		cm.RecordSuccess(peer2, 100*time.Millisecond)
		cm.RecordSuccess(peer2, 200*time.Millisecond)
		cm.RecordFailure(peer2)

		stats := cm.GetPeerStats(peer2)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(3), stats.TotalRequests)
		assert.Equal(t, int64(2), stats.SuccessfulReqs)
		assert.Equal(t, int64(1), stats.FailedReqs)
	})

	t.Run("success rate", func(t *testing.T) {
		rate := cm.GetSuccessRate(peer2)
		assert.InDelta(t, 0.666, rate, 0.01)
	})

	t.Run("blacklist", func(t *testing.T) {
		// 添加更多失败
		for i := 0; i < 10; i++ {
			cm.RecordFailure(peer2)
		}

		// 检查是否应该被加入黑名单
		blacklisted := cm.ShouldBlacklist(peer2, 0.5, 5)
		assert.True(t, blacklisted, "Peer with low success rate should be blacklisted")
	})
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	t.Run("is retryable - network errors", func(t *testing.T) {
		netErr := context.DeadlineExceeded
		assert.True(t, p2p.IsRetryable(netErr), "Network timeout should be retryable")
	})

	t.Run("is retryable - retryable error wrapper", func(t *testing.T) {
		wrappedErr := p2p.NewRetryableError(fmt.Errorf("temporary failure"))
		assert.True(t, p2p.IsRetryable(wrappedErr), "Wrapped retryable error should be retryable")
	})

	t.Run("is not retryable - nil error", func(t *testing.T) {
		assert.False(t, p2p.IsRetryable(nil), "Nil error should not be retryable")
	})
}

// TestServiceShutdown 测试服务优雅关闭
func TestServiceShutdown(t *testing.T) {
	ctx := context.Background()

	config := p2p.NewP2PConfig()
	config.Port = 0

	service, err := p2p.NewP2PService(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, service)

	// 测试关闭
	err = service.Shutdown()
	assert.NoError(t, err)

	// 再次关闭应该不会出错
	err = service.Shutdown()
	assert.NoError(t, err)
}

// BenchmarkChunkDownload 性能测试：Chunk 下载
func BenchmarkChunkDownload(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := p2p.NewP2PConfig()
	config.Port = 0
	config.ChunkStoragePath = b.TempDir()

	node, err := p2p.NewP2PService(ctx, config)
	if err != nil {
		b.Fatal(err)
	}
	defer node.Shutdown()

	// 创建测试 chunk
	testData := make([]byte, 1024*100) // 100KB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	chunkHash := createTestChunk(&testing.T{}, testData)
	chunkPath := filepath.Join(node.Config.ChunkStoragePath, chunkHash)
	_ = os.WriteFile(chunkPath, testData, 0644)

	err = node.Announce(ctx, chunkHash)
	if err != nil {
		b.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	// 直接使用节点自身 ID 测试本地读取
	providers, _ := node.Lookup(ctx, chunkHash)
	if len(providers) == 0 {
		b.Skip("No providers found")
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = node.DownloadChunk(ctx, providers[0].ID, chunkHash)
	}
}

// BenchmarkFileDownload 性能测试：文件下载
func BenchmarkFileDownload(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	config := p2p.NewP2PConfig()
	config.Port = 0
	config.ChunkStoragePath = b.TempDir()

	node, err := p2p.NewP2PService(ctx, config)
	if err != nil {
		b.Fatal(err)
	}
	defer node.Shutdown()

	// 准备测试文件
	fileSize := int64(1024 * 500) // 500KB
	testData, _ := createTestFile(&testing.T{}, fileSize)

	chunkSize := 1024 * 10
	var chunks []file.ChunkData

	for i := 0; i < len(testData); i += chunkSize {
		end := i + chunkSize
		if end > len(testData) {
			end = len(testData)
		}

		chunkData := testData[i:end]
		hash := sha256.Sum256(chunkData)

		chunkHash := fmt.Sprintf("%x", hash[:])
		chunkPath := filepath.Join(node.Config.ChunkStoragePath, chunkHash)
		_ = os.WriteFile(chunkPath, chunkData, 0644)

		chunks = append(chunks, file.ChunkData{
			ChunkSize: len(chunkData),
			ChunkHash: hash[:],
		})
	}

	metaData := &file.MetaData{
		FileName: "bench.bin",
		FileSize: uint64(fileSize),
		Leaves:   chunks,
	}

	metaJSON, _ := json.Marshal(metaData)
	fileHash := fmt.Sprintf("%x", sha256.Sum256(metaJSON))

	_ = node.Put(ctx, fileHash, metaJSON)

	for _, chunk := range chunks {
		chunkHashStr := fmt.Sprintf("%x", chunk.ChunkHash)
		_ = node.Announce(ctx, chunkHashStr)
	}

	time.Sleep(2 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("bench%d.bin", i))
		f, _ := os.Create(outputPath)
		_ = node.GetFileOrdered(ctx, fileHash, f)
		f.Close()
		os.Remove(outputPath)
	}
}
