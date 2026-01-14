// Package main 提供多节点集成测试
//
// 测试范围:
//   - 多节点 DHT 存储和检索
//   - 多节点 Chunk 公告和查询
//   - 跨节点 Chunk 下载
//   - 跨节点文件下载
//   - 并发下载测试 (多 Chunk)
//   - 节点发现测试 (7 节点网络)
//   - DHT 数据持久化测试
//
// 运行测试:
//   - go test -v -run TestMultiNode: 运行所有多节点测试
//   - ./run_multinode_tests.bat: Windows 脚本
//   - ./run_multinode_tests.sh: Linux/macOS 脚本
//   - go test -short: 跳过所有多节点测试
//
// 网络拓扑:
//   - 全网状连接 (Fully Connected Mesh)
//   - 每个节点直接连接到所有其他节点
//   - 5 或 7 个节点
//
// 测试特点:
//   - 真实网络环境: 使用实际 libp2p 连接
//   - 自动清理: defer cleanup() 确保资源释放
//   - 超时控制: 每个测试 5-10 分钟超时
//   - 并发安全: 使用 sync.WaitGroup 协调
//
// 注意事项:
//   - 测试执行时间较长 (~55 秒)
//   - 占用较多系统资源
//   - 需要稳定的网络环境
//   - 适合在提交前运行，不适合开发时频繁运行
package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"p2pFileTransfer/pkg/p2p"
)

// setupMultiNodeNetwork 创建一个多节点网络用于测试
// 返回节点列表和清理函数
func setupMultiNodeNetwork(t *testing.T, numNodes int) ([]*p2p.P2PService, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	nodes := make([]*p2p.P2PService, numNodes)
	var wg sync.WaitGroup
	errChan := make(chan error, numNodes)

	// 创建所有节点
	for i := 0; i < numNodes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			config := p2p.NewP2PConfig()
			config.Port = 0 // 随机端口
			config.ChunkStoragePath = t.TempDir()

			node, err := p2p.NewP2PService(ctx, config)
			if err != nil {
				errChan <- fmt.Errorf("failed to create node %d: %w", idx, err)
				return
			}
			nodes[idx] = node
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		require.NoError(t, err)
	}

	// 确保所有节点都创建成功
	for i, node := range nodes {
		assert.NotNil(t, node, "node %d should not be nil", i)
	}

	// 等待所有节点完全启动
	time.Sleep(2 * time.Second)

	// 连接所有节点形成全网状网络
	connectAllNodes(t, nodes)

	// 等待DHT稳定
	time.Sleep(3 * time.Second)

	// 返回清理函数
	cleanup := func() {
		for _, node := range nodes {
			if node != nil {
				node.Shutdown()
			}
		}
		cancel()
	}

	return nodes, cleanup
}

// connectAllNodes 将所有节点互相连接形成全网状网络
func connectAllNodes(t *testing.T, nodes []*p2p.P2PService) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// 连接每对节点
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			wg.Add(1)
			go func(node1, node2 *p2p.P2PService, idx1, idx2 int) {
				defer wg.Done()

				// 获取node2的地址信息
				peerInfo := &peer.AddrInfo{
					ID:    node2.Host.ID(),
					Addrs: node2.Host.Addrs(),
				}

				// node1 连接到 node2
				err := node1.Host.Connect(ctx, *peerInfo)
				if err != nil {
					t.Logf("Warning: node %d failed to connect to node %d: %v", idx1, idx2, err)
				} else {
					t.Logf("Successfully connected node %d to node %d", idx1, idx2)
				}
			}(nodes[i], nodes[j], i, j)
		}
	}

	wg.Wait()
}

// TestMultiNodeDHTPutAndGet 多节点DHT存储和检索测试
func TestMultiNodeDHTPutAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	nodes, cleanup := setupMultiNodeNetwork(t, 5)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 在节点0上存储数据
	key := "test-key-multinode"
	value := []byte("test-value-multinode")

	err := nodes[0].Put(ctx, key, value)
	require.NoError(t, err, "Failed to put value in DHT")

	t.Logf("Stored key-value pair in node 0")

	// 等待数据传播
	time.Sleep(2 * time.Second)

	// 从节点4检索数据
	retrieved, err := nodes[4].Get(ctx, key)
	require.NoError(t, err, "Failed to get value from DHT")
	assert.Equal(t, string(value), retrieved, "Retrieved value should match stored value")

	t.Logf("Successfully retrieved value from node 4")
}

// TestMultiNodeChunkAnnounceAndLookup 多节点Chunk公告和查询测试
func TestMultiNodeChunkAnnounceAndLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	nodes, cleanup := setupMultiNodeNetwork(t, 5)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 在节点0上创建测试chunk
	testData := []byte("hello world from multi-node test")
	chunkHash := createTestChunk(t, testData)

	// 将chunk存储到节点0
	chunkPath := nodes[0].Config.ChunkStoragePath + "/" + chunkHash
	err := os.WriteFile(chunkPath, testData, 0644)
	require.NoError(t, err, "Failed to write chunk file")

	// 节点0公告chunk
	err = nodes[0].Announce(ctx, chunkHash)
	require.NoError(t, err, "Failed to announce chunk")

	t.Logf("Node 0 announced chunk: %s", chunkHash)

	// 等待公告传播
	time.Sleep(3 * time.Second)

	// 节点4查询chunk的提供者
	providers, err := nodes[4].Lookup(ctx, chunkHash)
	require.NoError(t, err, "Failed to lookup providers")
	assert.NotEmpty(t, providers, "Should find at least one provider")

	t.Logf("Node 4 found %d providers for chunk", len(providers))

	// 验证节点0在提供者列表中
	found := false
	node0ID := nodes[0].Host.ID()
	for _, p := range providers {
		if p.ID == node0ID {
			found = true
			t.Logf("Found node 0 in provider list")
			break
		}
	}
	assert.True(t, found, "Node 0 should be in provider list")
}

// TestMultiNodeChunkDownload 多节点Chunk下载测试
func TestMultiNodeChunkDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	nodes, cleanup := setupMultiNodeNetwork(t, 5)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 在节点0上创建测试chunk
	testData := []byte("test chunk data for multi-node download")
	chunkHash := createTestChunk(t, testData)

	// 将chunk存储到节点0
	chunkPath := nodes[0].Config.ChunkStoragePath + "/" + chunkHash
	err := os.WriteFile(chunkPath, testData, 0644)
	require.NoError(t, err, "Failed to write chunk file")

	// 节点0公告chunk
	err = nodes[0].Announce(ctx, chunkHash)
	require.NoError(t, err, "Failed to announce chunk")

	t.Logf("Node 0 announced chunk: %s", chunkHash)

	// 等待公告传播
	time.Sleep(3 * time.Second)

	// 节点4查询chunk的提供者
	providers, err := nodes[4].Lookup(ctx, chunkHash)
	require.NoError(t, err, "Failed to lookup providers")
	require.NotEmpty(t, providers, "Should find at least one provider")

	// 使用第一个提供者的ID下载chunk
	providerPeerID := providers[0].ID

	// 节点4检查chunk是否存在
	exists, err := nodes[4].CheckChunkExists(ctx, providerPeerID, chunkHash)
	require.NoError(t, err, "Failed to check chunk existence")
	assert.True(t, exists, "Chunk should exist")

	t.Logf("Node 4 confirmed chunk exists")

	// 节点4下载chunk
	downloadedData, err := nodes[4].DownloadChunk(ctx, providerPeerID, chunkHash)
	require.NoError(t, err, "Failed to download chunk")
	assert.Equal(t, testData, downloadedData, "Downloaded data should match original data")

	t.Logf("Node 4 successfully downloaded chunk (%d bytes)", len(downloadedData))
}

// TestMultiNodeFileDownload 多节点文件下载测试
func TestMultiNodeFileDownload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	nodes, cleanup := setupMultiNodeNetwork(t, 5)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 在节点0上创建测试文件
	fileData, rootHash := createTestFile(t, 1024) // 1KB file

	t.Logf("Created test file with root hash: %s", rootHash)

	// 获取文件的所有chunk hashes（从Merkle树获取）
	// 这里简化处理：直接使用root hash作为文件标识
	// 在实际实现中，应该解析Merkle树获取所有chunk哈希

	// 由于API限制，这里测试单chunk下载
	testChunkHash := createTestChunk(t, fileData)
	chunkPath := nodes[0].Config.ChunkStoragePath + "/" + testChunkHash
	err := os.WriteFile(chunkPath, fileData, 0644)
	require.NoError(t, err, "Failed to write chunk")

	// 节点0公告chunk
	err = nodes[0].Announce(ctx, testChunkHash)
	require.NoError(t, err, "Failed to announce chunk")

	t.Logf("Node 0 announced chunk: %s", testChunkHash)

	// 等待公告传播
	time.Sleep(3 * time.Second)

	// 节点4下载chunk
	providers, err := nodes[4].Lookup(ctx, testChunkHash)
	require.NoError(t, err, "Failed to lookup providers")
	require.NotEmpty(t, providers, "Should find providers")

	providerPeerID := providers[0].ID
	downloadedData, err := nodes[4].DownloadChunk(ctx, providerPeerID, testChunkHash)
	require.NoError(t, err, "Failed to download chunk")
	assert.Equal(t, fileData, downloadedData, "Downloaded data should match")

	t.Logf("Node 4 successfully downloaded chunk from multi-node network")
}

// TestMultiNodeConcurrentDownloads 多节点并发下载测试
func TestMultiNodeConcurrentDownloads(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	nodes, cleanup := setupMultiNodeNetwork(t, 5)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 在节点0上创建多个测试chunk
	numChunks := 3
	chunkHashes := make([]string, numChunks)
	chunkData := make([][]byte, numChunks)

	for i := 0; i < numChunks; i++ {
		data := []byte(fmt.Sprintf("test chunk %d data for concurrent download", i))
		chunkData[i] = data
		chunkHash := createTestChunk(t, data)
		chunkHashes[i] = chunkHash

		// 存储到节点0
		chunkPath := nodes[0].Config.ChunkStoragePath + "/" + chunkHash
		err := os.WriteFile(chunkPath, data, 0644)
		require.NoError(t, err, "Failed to write chunk %d", i)

		// 公告
		err = nodes[0].Announce(ctx, chunkHash)
		require.NoError(t, err, "Failed to announce chunk %d", i)

		t.Logf("Node 0 announced chunk %d/%d: %s", i+1, numChunks, chunkHash)
	}

	t.Logf("Node 0 announced all %d chunks", numChunks)

	// 等待公告传播
	time.Sleep(3 * time.Second)

	// 从不同节点并发下载
	var wg sync.WaitGroup
	errChan := make(chan error, numChunks)
	successCount := 0

	for i := 0; i < numChunks; i++ {
		wg.Add(1)
		go func(idx int, chunkHash string, expectedData []byte) {
			defer wg.Done()

			// 使用不同的节点下载
			nodeIdx := (idx + 1) % len(nodes)

			// 查询提供者
			providers, err := nodes[nodeIdx].Lookup(ctx, chunkHash)
			if err != nil {
				errChan <- fmt.Errorf("chunk %d lookup failed on node %d: %w", idx, nodeIdx, err)
				return
			}

			if len(providers) == 0 {
				errChan <- fmt.Errorf("chunk %d: no providers found on node %d", idx, nodeIdx)
				return
			}

			// 下载chunk
			providerPeerID := providers[0].ID
			downloadedData, err := nodes[nodeIdx].DownloadChunk(ctx, providerPeerID, chunkHash)
			if err != nil {
				errChan <- fmt.Errorf("chunk %d download failed on node %d: %w", idx, nodeIdx, err)
				return
			}

			// 验证数据
			if string(downloadedData) != string(expectedData) {
				errChan <- fmt.Errorf("chunk %d: data mismatch on node %d", idx, nodeIdx)
				return
			}

			successCount++
			t.Logf("Successfully downloaded chunk %d on node %d", idx, nodeIdx)
		}(i, chunkHashes[i], chunkData[i])
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		require.NoError(t, err)
	}

	assert.Equal(t, numChunks, successCount, "Should have downloaded all chunks")
	t.Logf("Successfully downloaded all %d chunks concurrently", numChunks)
}

// TestMultiNodePeerDiscovery 多节点发现测试
func TestMultiNodePeerDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	numNodes := 7
	nodes, cleanup := setupMultiNodeNetwork(t, numNodes)
	defer cleanup()

	// 检查每个节点能否发现其他节点
	for i, node := range nodes {
		peerCount := 0
		for j := 0; j < numNodes; j++ {
			if i == j {
				continue
			}

			// 检查节点i是否连接到节点j
			connectedness := node.Host.Network().Connectedness(nodes[j].Host.ID())
			if connectedness == 1 { // Connected
				peerCount++
			}
		}

		t.Logf("Node %d is connected to %d/%d other nodes", i, peerCount, numNodes-1)
		assert.Greater(t, peerCount, 0, "Node %d should be connected to at least one peer", i)
	}
}

// TestMultiNodeDHTPersistence 多节点DHT数据持久化测试
func TestMultiNodeDHTPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-node test in short mode")
	}

	nodes, cleanup := setupMultiNodeNetwork(t, 5)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 在节点0存储数据
	key := "persistence-test-key"
	value := []byte("persistence-test-value")

	err := nodes[0].Put(ctx, key, value)
	require.NoError(t, err, "Failed to put value")

	t.Logf("Node 0 stored key: %s", key)

	// 等待数据传播到其他节点
	time.Sleep(3 * time.Second)

	// 从多个节点尝试检索
	successCount := 0
	for i := 1; i < len(nodes); i++ {
		retrieved, err := nodes[i].Get(ctx, key)
		if err == nil && string(retrieved) == string(value) {
			successCount++
			t.Logf("Node %d successfully retrieved the value", i)
		} else {
			t.Logf("Node %d failed to retrieve value: %v", i, err)
		}
	}

	t.Logf("Value was successfully retrieved from %d/%d nodes", successCount, len(nodes)-1)
	assert.Greater(t, successCount, 0, "At least one node should have retrieved the value")
}
