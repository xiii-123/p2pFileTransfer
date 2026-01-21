package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"p2pFileTransfer/pkg/chameleonMerkleTree"
	"p2pFileTransfer/pkg/file"
)

const (
	// MaxUploadSize 最大上传文件大小 (100GB)
	MaxUploadSize = 100 << 30

	// DefaultBlockSize 默认分块大小 (256KB)
	DefaultBlockSize = 262144

	// DefaultBufferNumber 默认缓冲区数量
	DefaultBufferNumber = 16
)

// APIResponse 统一的API响应格式
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// respondJSON 发送JSON响应
func (s *Server) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondError 发送错误响应
func (s *Server) respondError(w http.ResponseWriter, statusCode int, message string) {
	s.respondJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}

// respondSuccess 发送成功响应
func (s *Server) respondSuccess(w http.ResponseWriter, data interface{}) {
	s.respondJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"status": "ok",
		"service": "p2p-file-transfer-api",
	})
}

// handleFileUpload 文件上传
func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 解析表单（最大100GB）
	err := r.ParseMultipartForm(MaxUploadSize)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get file: %v", err))
		return
	}
	defer file.Close()

	// 获取参数
	treeType := r.FormValue("tree_type")
	if treeType == "" {
		treeType = "chameleon" // 默认使用chameleon
	}
	description := r.FormValue("description")

	// 验证 tree_type 参数
	allowedTypes := map[string]bool{"chameleon": true, "regular": true}
	if !allowedTypes[treeType] {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid tree_type '%s'. Must be 'chameleon' or 'regular'", treeType))
		return
	}

	// 根据树类型上传文件
	var result map[string]interface{}
	if treeType == "chameleon" {
		result, err = s.uploadFileChameleon(r.Context(), file, header.Filename, description)
	} else {
		result, err = s.uploadFileRegular(r.Context(), file, header.Filename, description)
	}

	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Upload failed: %v", err))
		return
	}

	s.respondSuccess(w, result)
}

// uploadFileChameleon 使用Chameleon Merkle Tree上传文件
func (s *Server) uploadFileChameleon(ctx context.Context, fileReader io.Reader, fileName, description string) (map[string]interface{}, error) {
	// 生成密钥对
	_, pubKey := chameleonMerkleTree.NewChameleonKeyPair()

	// 创建临时文件（避免将整个文件加载到内存）
	tmpFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // 确保临时文件被删除
	defer tmpFile.Close()

	// 流式复制上传内容到临时文件
	size, err := io.Copy(tmpFile, fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	// 重置文件指针到开头
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	// 构建Chameleon Merkle Tree
	config := &chameleonMerkleTree.MerkleConfig{
		BlockSize:    DefaultBlockSize,
		BufferNumber: DefaultBufferNumber,
	}

	cmt, err := chameleonMerkleTree.NewChameleonMerkleTree(tmpFile, config, pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to build merkle tree: %w", err)
	}

	// 获取根哈希（CID）
	cid := cmt.GetChameleonHash()
	cidHex := hex.EncodeToString(cid)

	// 获取所有分块哈希
	chunkHashes := cmt.GetAllLeavesHashes()

	// 保存所有分块到本地存储
	chunkPath := s.config.Storage.ChunkPath
	buffer := make([]byte, config.BlockSize) // 重用缓冲区，避免频繁分配

	for i, chunkHash := range chunkHashes {
		// 重置文件指针到chunk起始位置
		offset := int64(i) * int64(config.BlockSize)
		if _, err := tmpFile.Seek(offset, 0); err != nil {
			return nil, fmt.Errorf("failed to seek to chunk %d: %w", i, err)
		}

		// 读取chunk数据
		n, err := tmpFile.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i, err)
		}
		chunkData := buffer[:n]

		// 保存chunk到文件
		chunkFile := filepath.Join(chunkPath, hex.EncodeToString(chunkHash))
		if err := os.WriteFile(chunkFile, chunkData, 0644); err != nil {
			return nil, fmt.Errorf("failed to save chunk %d: %w", i, err)
		}

		// Announce到DHT
		chunkHashStr := hex.EncodeToString(chunkHash)
		if err := s.p2pService.Announce(ctx, chunkHashStr); err != nil {
			return nil, fmt.Errorf("failed to announce chunk %d: %w", i, err)
		}
	}

	// 生成元数据
	metadata := &file.MetaData{
		RootHash:    cid,
		RandomNum:   cmt.GetRandomNumber().Serialize(),
		PublicKey:   cmt.GetPublicKey().Serialize(),
		Description: description,
		FileName:    fileName,
		FileSize:    uint64(size),
		Encryption:  "none",
		TreeType:    "chameleon",
		Leaves:      convertToChunkData(chunkHashes, int(config.BlockSize)),
	}

	// 保存元数据
	if err := s.saveMetadata(cidHex, metadata); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cid":        cidHex,
		"fileName":   fileName,
		"treeType":   "chameleon",
		"chunkCount": len(chunkHashes),
		"fileSize":   size,
		"message":    "File uploaded successfully with Chameleon Merkle Tree",
	}, nil
}

// uploadFileRegular 使用Regular Merkle Tree上传文件
func (s *Server) uploadFileRegular(ctx context.Context, fileReader io.Reader, fileName, description string) (map[string]interface{}, error) {
	// 创建临时文件（避免将整个文件加载到内存）
	tmpFile, err := os.CreateTemp("", "upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // 确保临时文件被删除
	defer tmpFile.Close()

	// 流式复制上传内容到临时文件
	size, err := io.Copy(tmpFile, fileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	// 重置文件指针到开头
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	// 构建Merkle Tree配置
	config := &chameleonMerkleTree.MerkleConfig{
		BlockSize:    DefaultBlockSize,
		BufferNumber: DefaultBufferNumber,
	}

	// 使用标准Merkle Tree（不使用Chameleon哈希）
	rootNode, err := chameleonMerkleTree.BuildMerkleTreeFromFileRW(tmpFile, config)
	if err != nil {
		return nil, fmt.Errorf("failed to build merkle tree: %w", err)
	}

	// 获取根哈希（CID）
	cid := rootNode.Hash
	cidHex := hex.EncodeToString(cid)

	// 获取所有叶子节点哈希
	chunkHashes := getAllLeafHashes(rootNode)

	// 保存所有分块到本地存储
	chunkPath := s.config.Storage.ChunkPath
	buffer := make([]byte, config.BlockSize) // 重用缓冲区，避免频繁分配

	for i, chunkHash := range chunkHashes {
		// 重置文件指针到chunk起始位置
		offset := int64(i) * int64(config.BlockSize)
		if _, err := tmpFile.Seek(offset, 0); err != nil {
			return nil, fmt.Errorf("failed to seek to chunk %d: %w", i, err)
		}

		// 读取chunk数据
		n, err := tmpFile.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i, err)
		}
		chunkData := buffer[:n]

		// 保存chunk到文件
		chunkFile := filepath.Join(chunkPath, hex.EncodeToString(chunkHash))
		if err := os.WriteFile(chunkFile, chunkData, 0644); err != nil {
			return nil, fmt.Errorf("failed to save chunk %d: %w", i, err)
		}

		// Announce到DHT
		chunkHashStr := hex.EncodeToString(chunkHash)
		if err := s.p2pService.Announce(ctx, chunkHashStr); err != nil {
			return nil, fmt.Errorf("failed to announce chunk %d: %w", i, err)
		}
	}

	// 生成元数据
	metadata := &file.MetaData{
		RootHash:     cid,
		Description:  description,
		FileName:     fileName,
		FileSize:     uint64(size),
		Encryption:   "none",
		TreeType:     "regular",
		Leaves:       convertToChunkData(chunkHashes, int(config.BlockSize)),
	}

	// 保存元数据
	if err := s.saveMetadata(cidHex, metadata); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cid":        cidHex,
		"fileName":   fileName,
		"treeType":   "regular",
		"chunkCount": len(chunkHashes),
		"fileSize":   size,
		"message":    "File uploaded successfully with Regular Merkle Tree",
	}, nil
}

// getAllLeafHashes 从Merkle树根节点获取所有叶子节点哈希
func getAllLeafHashes(root *chameleonMerkleTree.MerkleNode) [][]byte {
	var hashes [][]byte
	var queue []*chameleonMerkleTree.MerkleNode
	queue = append(queue, root)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if node.Left == nil && node.Right == nil {
			// 叶子节点
			hashes = append(hashes, node.Hash)
		}

		if node.Left != nil {
			queue = append(queue, node.Left)
		}
		if node.Right != nil {
			queue = append(queue, node.Right)
		}
	}

	return hashes
}

// handleFileInfo 文件信息查询
func (s *Server) handleFileInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从URL路径中提取CID
	cid := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	cid = strings.TrimSuffix(cid, "/download")
	cid = strings.Trim(cid, "/")

	if cid == "" {
		s.respondError(w, http.StatusBadRequest, "CID is required")
		return
	}

	// 读取元数据
	metadata, err := s.loadMetadata(cid)
	if err != nil {
		s.respondError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	s.respondSuccess(w, metadata)
}

// handleFileDownload 文件下载
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从URL路径中提取CID
	cid := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	cid = strings.TrimSuffix(cid, "/download")
	cid = strings.Trim(cid, "/")

	if cid == "" {
		s.respondError(w, http.StatusBadRequest, "CID is required")
		return
	}

	// 读取元数据
	metadata, err := s.loadMetadata(cid)
	if err != nil {
		s.respondError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", metadata.FileName))

	// 首先尝试从本地chunk文件重组
	if err := s.downloadFromLocalChunks(w, metadata); err == nil {
		return
	}

	// 如果本地chunk不可用，尝试从P2P网络下载
	ctx := r.Context()
	var buf bytes.Buffer
	if err := s.p2pService.GetFileOrdered(ctx, cid, &buf); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Download failed: %v", err))
		return
	}

	// 写入文件数据
	w.Write(buf.Bytes())
}

// downloadFromLocalChunks 从本地chunk文件重组下载
func (s *Server) downloadFromLocalChunks(w http.ResponseWriter, metadata *file.MetaData) error {
	chunkPath := s.config.Storage.ChunkPath

	// 读取所有chunk并组装
	for _, leaf := range metadata.Leaves {
		chunkFile := filepath.Join(chunkPath, hex.EncodeToString(leaf.ChunkHash))
		data, err := os.ReadFile(chunkFile)
		if err != nil {
			return fmt.Errorf("failed to read chunk %s from %s: %w", hex.EncodeToString(leaf.ChunkHash), chunkFile, err)
		}

		// 写入响应
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
	}

	return nil
}

// handleNodeInfo 节点信息
func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	peerID := s.p2pService.Host.ID().String()
	addresses := s.p2pService.Host.Addrs()

	addrs := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addrs = append(addrs, addr.String()+"/p2p/"+peerID)
	}

	s.respondSuccess(w, map[string]interface{}{
		"peerID":    peerID,
		"addresses": addrs,
		"protocols": s.p2pService.Host.Mux().Protocols(),
	})
}

// handlePeerList 获取连接的对等节点列表
func (s *Server) handlePeerList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	peers := s.p2pService.Host.Network().Peers()
	peerList := make([]map[string]interface{}, 0, len(peers))

	for _, peerID := range peers {
		peerInfo := map[string]interface{}{
			"peerID": peerID.String(),
		}

		// 获取连接状态
		if s.p2pService.Host.Network().Connectedness(peerID) == 1 { // Connected
			conns := s.p2pService.Host.Network().ConnsToPeer(peerID)
			if len(conns) > 0 {
				peerInfo["address"] = conns[0].RemoteMultiaddr().String()
				peerInfo["direction"] = conns[0].Stat().Direction.String()
			}
		}

		peerList = append(peerList, peerInfo)
	}

	s.respondSuccess(w, map[string]interface{}{
		"count": len(peerList),
		"peers": peerList,
	})
}

// handlePeerConnect 连接到对等节点
func (s *Server) handlePeerConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Address string `json:"address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.Address == "" {
		s.respondError(w, http.StatusBadRequest, "Address is required")
		return
	}

	s.respondError(w, http.StatusNotImplemented, "Connection feature not implemented")
}

// handleDHTFindProviders 查找DHT提供者
func (s *Server) handleDHTFindProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从URL路径中提取key
	key := strings.TrimPrefix(r.URL.Path, "/api/v1/dht/providers/")
	key = strings.Trim(key, "/")

	if key == "" {
		s.respondError(w, http.StatusBadRequest, "Key is required")
		return
	}

	ctx := r.Context()
	providers, err := s.p2pService.Lookup(ctx, key)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to find providers: %v", err))
		return
	}

	providerList := make([]string, 0, len(providers))
	for _, provider := range providers {
		providerList = append(providerList, provider.String())
	}

	s.respondSuccess(w, map[string]interface{}{
		"key":       key,
		"count":     len(providerList),
		"providers": providerList,
	})
}

// handleDHTAnnounce DHT公告
func (s *Server) handleDHTAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Key string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.Key == "" {
		s.respondError(w, http.StatusBadRequest, "Key is required")
		return
	}

	ctx := r.Context()
	if err := s.p2pService.Announce(ctx, req.Key); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Announce failed: %v", err))
		return
	}

	s.respondSuccess(w, map[string]interface{}{
		"key":     req.Key,
		"message": "Announced successfully",
	})
}

// handleDHTGetValue 获取DHT值
func (s *Server) handleDHTGetValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从URL路径中提取key
	key := strings.TrimPrefix(r.URL.Path, "/api/v1/dht/value/")
	key = strings.Trim(key, "/")

	if key == "" {
		s.respondError(w, http.StatusBadRequest, "Key is required")
		return
	}

	ctx := r.Context()
	value, err := s.p2pService.Get(ctx, key)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get value: %v", err))
		return
	}

	s.respondSuccess(w, map[string]interface{}{
		"key":   key,
		"value": value,
	})
}

// handleDHTPutValue 设置DHT值
func (s *Server) handleDHTPutValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.Key == "" || req.Value == "" {
		s.respondError(w, http.StatusBadRequest, "Key and value are required")
		return
	}

	ctx := r.Context()
	if err := s.p2pService.Put(ctx, req.Key, []byte(req.Value)); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to put value: %v", err))
		return
	}

	s.respondSuccess(w, map[string]interface{}{
		"key":     req.Key,
		"message": "Value stored successfully",
	})
}

// handleChunkDownload 根据hash下载单个分片
//
// 功能说明:
//   - 支持从本地存储或P2P网络下载指定hash的分片数据
//   - 优先从本地存储读取，本地不存在时从P2P网络下载
//   - P2P下载的分片会自动缓存到本地存储
//   - 通过响应头 X-Chunk-Source 标识数据来源
//
// 路由: GET /api/v1/chunks/{hash}/download
//
// 请求参数:
//   - hash: 分片的哈希值（hex编码），从URL路径中提取
//
// 响应头:
//   - Content-Type: application/octet-stream
//   - Content-Disposition: attachment; filename="{hash}.bin"
//   - X-Chunk-Source: 数据来源标识
//     * "local" - 从本地存储读取
//     * "p2p-downloaded" - 从P2P网络下载并已缓存到本地
//     * "p2p" - 从P2P网络下载（缓存失败）
//
// 响应体: 分片二进制数据
//
// 错误响应:
//   - 400 Bad Request - 无效的hash格式
//   - 404 Not Found - 分片不存在（本地和P2P网络都找不到）
//   - 500 Internal Server Error - P2P下载失败
//
// 使用场景:
//   1. 断点续传 - 根据需要下载特定分片
//   2. 部分访问 - 只需要文件的部分内容
//   3. 并行下载 - 多个分片同时下载提高速度
//   4. 带宽优化 - 只下载需要的分片
//
// 示例:
//   curl http://localhost:8080/api/v1/chunks/abc123.../download -o chunk.bin
func (s *Server) handleChunkDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从URL路径中提取chunk hash
	chunkHash := strings.TrimPrefix(r.URL.Path, "/api/v1/chunks/")
	chunkHash = strings.TrimSuffix(chunkHash, "/download")
	chunkHash = strings.Trim(chunkHash, "/")

	if chunkHash == "" {
		s.respondError(w, http.StatusBadRequest, "Chunk hash is required")
		return
	}

	// 验证hash格式（应该是hex编码的）
	_, err := hex.DecodeString(chunkHash)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid chunk hash format: %v", err))
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.bin\"", chunkHash))

	// 优先从本地存储读取
	chunkPath := filepath.Join(s.config.Storage.ChunkPath, chunkHash)
	data, err := os.ReadFile(chunkPath)
	if err == nil {
		// 本地存在，直接返回
		w.Header().Set("X-Chunk-Source", "local")
		w.Write(data)
		return
	}

	// 本地不存在，尝试从P2P网络下载
	ctx := r.Context()

	// 查找拥有该chunk的peer
	providers, err := s.p2pService.Lookup(ctx, chunkHash)
	if err != nil || len(providers) == 0 {
		s.respondError(w, http.StatusNotFound, fmt.Sprintf("Chunk not found locally or in P2P network: %v", err))
		return
	}

	// 尝试从找到的peer下载chunk
	var downloadErr error
	for _, provider := range providers {
		// 使用P2P服务下载chunk (provider.ID 是 peer.ID 类型)
		data, downloadErr = s.p2pService.DownloadChunk(ctx, provider.ID, chunkHash)
		if downloadErr == nil {
			// 下载成功，保存到本地存储以便下次使用
			if saveErr := os.WriteFile(chunkPath, data, 0644); saveErr == nil {
				w.Header().Set("X-Chunk-Source", "p2p-downloaded")
			} else {
				w.Header().Set("X-Chunk-Source", "p2p")
			}
			w.Write(data)
			return
		}
	}

	// 所有peer都下载失败
	s.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to download chunk from P2P network: %v", downloadErr))
}

// handleChunkInfo 查询分片信息
//
// 功能说明:
//   - 查询指定hash的分片在本地和P2P网络中的可用性
//   - 返回分片是否在本地存在、文件大小
//   - 返回P2P网络中的提供者数量和列表
//
// 路由: GET /api/v1/chunks/{hash}
//
// 请求参数:
//   - hash: 分片的哈希值（hex编码），从URL路径中提取
//
// 响应格式:
//   {
//     "success": true,
//     "data": {
//       "hash": "分片哈希",
//       "local": true/false,          // 是否在本地存储中存在
//       "size": 262144,               // 分片大小（字节），仅local=true时有值
//       "p2p_providers": 3,           // P2P网络中提供者数量
//       "providers": [                // 提供者的Peer ID列表
//         "12D3KooW...",
//         "QmXxx..."
//       ]
//     }
//   }
//
// 错误响应:
//   - 400 Bad Request - 无效的hash格式
//
// 使用场景:
//   1. 检查分片是否可用
//   2. 查找P2P网络中的提供者
//   3. 决定从哪里下载分片
//
// 示例:
//   curl http://localhost:8080/api/v1/chunks/abc123...
func (s *Server) handleChunkInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 从URL路径中提取chunk hash
	chunkHash := strings.TrimPrefix(r.URL.Path, "/api/v1/chunks/")
	chunkHash = strings.TrimSuffix(chunkHash, "/info")
	chunkHash = strings.Trim(chunkHash, "/")

	if chunkHash == "" {
		s.respondError(w, http.StatusBadRequest, "Chunk hash is required")
		return
	}

	// 验证hash格式
	_, err := hex.DecodeString(chunkHash)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid chunk hash format: %v", err))
		return
	}

	ctx := r.Context()

	// 检查本地是否存在
	chunkPath := filepath.Join(s.config.Storage.ChunkPath, chunkHash)
	info := make(map[string]interface{})
	info["hash"] = chunkHash

	if _, err := os.Stat(chunkPath); err == nil {
		info["local"] = true
		if fileInfo, _ := os.Stat(chunkPath); fileInfo != nil {
			info["size"] = fileInfo.Size()
		}
	} else {
		info["local"] = false
	}

	// 查找P2P网络中的提供者
	providers, err := s.p2pService.Lookup(ctx, chunkHash)
	if err != nil {
		info["p2p_providers"] = 0
		info["p2p_error"] = err.Error()
	} else {
		info["p2p_providers"] = len(providers)
		if len(providers) > 0 {
			providerList := make([]string, 0, len(providers))
			for _, p := range providers {
				providerList = append(providerList, p.String())
			}
			info["providers"] = providerList
		}
	}

	s.respondSuccess(w, info)
}

// 辅助函数

// convertToChunkData 转换哈希列表为ChunkData
func convertToChunkData(hashes [][]byte, chunkSize int) []file.ChunkData {
	chunks := make([]file.ChunkData, len(hashes))
	for i, hash := range hashes {
		chunks[i] = file.ChunkData{
			ChunkSize: chunkSize,
			ChunkHash: hash,
		}
	}
	return chunks
}

// saveMetadata 保存元数据
func (s *Server) saveMetadata(cid string, metadata *file.MetaData) error {
	metadataPath := filepath.Join(s.config.HTTP.MetadataStoragePath, cid+".json")
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(metadataPath), 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// loadMetadata 加载元数据
func (s *Server) loadMetadata(cid string) (*file.MetaData, error) {
	metadataPath := filepath.Join(s.config.HTTP.MetadataStoragePath, cid+".json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata file.MetaData
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &metadata, nil
}
