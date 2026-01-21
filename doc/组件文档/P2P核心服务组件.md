# P2P 核心服务组件

## 组件概述

- **位置**: `pkg/p2p/`
- **职责**: P2P 网络的核心实现，包括 DHT 路由、文件传输、节点管理等功能
- **依赖**:
  - `github.com/libp2p/go-libp2p` - P2P 网络框架
  - `github.com/libp2p/go-libp2p-kad-dht` - DHT 实现
  - `pkg/file` - 文件元数据处理
- **被依赖**: `cmd/p2p/`, `cmd/server/`

## 文件结构

```
pkg/p2p/
├── p2p.go                 # 核心服务和配置定义 (164 行)
├── dht.go                 # DHT 实现和协议处理 (580 行)
├── chunk.go               # Chunk 传输逻辑 (302 行)
├── getFile.go             # 文件下载实现 (534 行)
├── merkletree.go          # Merkle 树工具 (83 行)
├── antiLeecher.go         # 反吸血虫机制 (47 行)
├── connManager.go         # 连接管理器 (246 行)
├── peerSelector.go        # 节点选择器 (161 行)
└── utils.go               # 工具函数 (95 行)
```

## 核心数据结构

### 1. P2PService

```go
// P2PService 是核心服务结构，整合所有 P2P 功能
type P2PService struct {
    Host         host.Host                    // libp2p 主机实例
    DHT          *dht.IpfsDHT                 // DHT 实例
    Config       *P2PConfig                   // 配置
    PeerSelector PeerSelector                 // 节点选择器
    AntiLeecher  AntiLeecher                  // 反吸血虫
    FSAdapter    file.LocalFileSystemAdapter  // 文件系统适配器
    ConnManager  *ConnManager                 // 连接管理器
    Ctx          context.Context              // 服务上下文
    Cancel       context.CancelFunc           // 取消函数
}
```

**职责**:
- 管理整个 P2P 网络生命周期
- 协调各个子组件
- 提供统一的 API 接口

### 2. P2PConfig

```go
type P2PConfig struct {
    Port              int                        // 监听端口（0=随机）
    Insecure          bool                       // 是否使用不安全连接
    Seed              int64                      // 随机种子
    BootstrapPeers    []multiaddr.Multiaddr      // 引导节点
    ProtocolPrefix    string                     // 协议前缀
    EnableAutoRefresh bool                       // 启用自动刷新
    NameSpace         string                     // DHT 命名空间
    Validator         record.Validator           // 验证器
    ChunkStoragePath  string                     // Chunk 存储路径
    MaxRetries        int                        // 最大重试次数
    MaxConcurrency    int                        // 最大并发数
    RequestTimeout    int                        // 请求超时（秒）
    DataTimeout       int                        // 数据传输超时（秒）
    DHTTimeout        int                        // DHT 操作超时（秒）
}
```

### 3. ConnManager

```go
// ConnManager 管理对等节点的连接
type ConnManager struct {
    maxConns    int                           // 每节点最大连接数
    blacklistTTL time.Duration                // 黑名单过期时间
    peerStats   map[peer.ID]*PeerStats        // 节点统计信息
    blacklist   map[peer.ID]time.Time         // 黑名单
    mutex       sync.RWMutex                  // 读写锁
}

type PeerStats struct {
    TotalRequests   int       // 总请求数
    SuccessCount    int       // 成功数
    FailureCount    int       // 失败数
    FirstSeen       time.Time // 首次连接时间
    LastSuccess     time.Time // 最后成功时间
    LastFailure     time.Time // 最后失败时间
    ActiveConns     int       // 活跃连接数
}
```

## 核心接口

### 输入接口（创建服务）

```go
// NewP2PService 创建并启动 P2P 服务
func NewP2PService(ctx context.Context, config P2PConfig) (*P2PService, error)

// 参数:
//   - ctx: 上下文，用于控制生命周期
//   - config: 服务配置

// 返回值:
//   - *P2PService: 服务实例
//   - error: 错误信息
```

### 输出接口（服务操作）

```go
// 关闭服务
func (p *P2PService) Shutdown() error

// DHT 操作
func (p *P2PService) Announce(ctx context.Context, key string) error
func (p *P2PService) FindProviders(ctx context.Context, key string) []peer.ID
func (p *P2PService) PutValue(ctx context.Context, key string, value []byte) error
func (p *P2PService) GetValue(ctx context.Context, key string) ([]byte, error)

// Chunk 操作
func (p *P2PService) RegisterChunk(ctx context.Context, chunkHash []byte) error
func (p *P2PService) GetChunkData(ctx context.Context, peerID peer.ID, chunkHash []byte) ([]byte, error)

// 文件操作
func (p *P2PService) UploadFile(ctx context.Context, filePath string) (*file.MetaData, error)
func (p *P2PService) DownloadFile(ctx context.Context, metadata *file.MetaData, destPath string) error
```

## 核心实现

### 1. 服务初始化流程

```go
func NewP2PService(ctx context.Context, config P2PConfig) (*P2PService, error) {
    // 1. 创建 libp2p Host
    host, err := newBasicHost(config.Port, config.Insecure, config.Seed)
    if err != nil {
        return nil, xerrors.Errorf("failed to create host: %w", err)
    }

    // 2. 创建 DHT
    kdht, err := newDHT(ctx, host, config)
    if err != nil {
        return nil, xerrors.Errorf("failed to create DHT instance: %w", err)
    }

    // 3. 创建可取消的上下文
    serviceCtx, cancel := context.WithCancel(context.Background())

    // 4. 组装服务
    p := &P2PService{
        Host:         host,
        DHT:          kdht,
        Config:       &config,
        PeerSelector: &RandomPeerSelector{},
        AntiLeecher:  &DefaultAntiLeecher{},
        FSAdapter:    file.LocalFileSystemAdapter{},
        ConnManager:  NewConnManager(5, 10*time.Minute),
        Ctx:          serviceCtx,
        Cancel:       cancel,
    }

    // 5. 注册协议处理器
    p.AnnounceHandler(ctx)
    p.LookupHandler(ctx)
    p.RegisterChunkExistHandler(ctx)
    p.RegisterChunkDataHandler(ctx)

    return p, nil
}
```

### 2. DHT 操作

#### 2.1 Announce（公告）

```go
// Announce 向网络公告自己是某个 Chunk 的提供者
func (p *P2PService) Announce(ctx context.Context, key string) error {
    // 1. 通过 DHT Provide 公告
    ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Config.DHTTimeout)*time.Second)
    defer cancel()

    // 将字符串 key 转换为 CID
    cid, err := cid.Decode(key)
    if err != nil {
        return err
    }

    // 公告到 DHT
    if err := p.DHT.Provide(ctx, cid, true); err != nil {
        return xerrors.Errorf("failed to announce: %w", err)
    }

    logrus.Infof("Announced chunk: %s", key)
    return nil
}
```

#### 2.2 FindProviders（查找提供者）

```go
// FindProviders 查找特定 Chunk 的提供者
func (p *P2PService) FindProviders(ctx context.Context, key string) []peer.ID {
    // 1. 通过 DHT 查找
    ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Config.DHTTimeout)*time.Second)
    defer cancel()

    cid, err := cid.Decode(key)
    if err != nil {
        return nil
    }

    // 查找 Provider
    providerChan := p.DHT.FindProvidersAsync(ctx, cid, 0)

    // 2. 收集结果
    var providers []peer.ID
    for provider := range providerChan {
        if provider.ID == host.ID() {
            continue  // 跳过自己
        }
        providers = append(providers, provider.ID)
    }

    return providers
}
```

### 3. Chunk 传输

#### 3.1 注册 Chunk

```go
// RegisterChunk 注册一个新的 Chunk 到本地存储
func (p *P2PService) RegisterChunk(ctx context.Context, chunkHash []byte) error {
    // 1. 存储 Chunk 数据
    chunkPath := filepath.Join(p.Config.ChunkStoragePath, fmt.Sprintf("%x", chunkHash))

    if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
        return xerrors.Errorf("chunk file not found: %s", chunkPath)
    }

    // 2. 公告到 DHT
    key := fmt.Sprintf("%x", chunkHash)
    if err := p.Announce(ctx, key); err != nil {
        return err
    }

    logrus.Infof("Registered chunk: %x", chunkHash)
    return nil
}
```

#### 3.2 获取 Chunk 数据

```go
// GetChunkData 从指定节点获取 Chunk 数据
func (p *P2PService) GetChunkData(ctx context.Context, peerID peer.ID, chunkHash []byte) ([]byte, error) {
    // 1. 检查黑名单
    if p.ConnManager.IsBlacklisted(peerID) {
        return nil, xerrors.Errorf("peer is blacklisted")
    }

    // 2. 创建流
    ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Config.DataTimeout)*time.Second)
    defer cancel()

    stream, err := p.Host.NewStream(ctx, peerID, ChunkDataProtocol)
    if err != nil {
        p.ConnManager.RecordFailure(peerID)
        return nil, xerrors.Errorf("failed to create stream: %w", err)
    }
    defer stream.Close()

    // 3. 发送请求
    req := ChunkDataRequest{Hash: chunkHash}
    if err := json.NewEncoder(stream).Encode(req); err != nil {
        return nil, err
    }

    // 4. 读取响应
    var resp ChunkDataResponse
    if err := json.NewDecoder(stream).Decode(&resp); err != nil {
        p.ConnManager.RecordFailure(peerID)
        return nil, err
    }

    // 5. 验证哈希
    if !bytes.Equal(hashData(resp.Data), chunkHash) {
        p.ConnManager.RecordFailure(peerID)
        return nil, xerrors.Errorf("chunk hash mismatch")
    }

    // 6. 记录成功
    p.ConnManager.RecordSuccess(peerID)

    return resp.Data, nil
}
```

### 4. 文件下载

#### 4.1 核心流程

```go
// DownloadFile 下载文件（并发下载）
func (p *P2PService) DownloadFile(ctx context.Context, metadata *file.MetaData, destPath string) error {
    // 1. 创建目标文件
    destFile, err := os.Create(destPath)
    if err != nil {
        return err
    }
    defer destFile.Close()

    // 2. 创建工作池
    semaphore := make(chan struct{}, p.Config.MaxConcurrency)
    results := make(chan *ChunkResult, len(metadata.Leaves))

    // 3. 启动下载任务
    for i, leaf := range metadata.Leaves {
        semaphore <- struct{}{}  // 获取令牌

        go func(index int, chunk file.ChunkData) {
            defer func() { <-semaphore }()  // 释放令牌

            result := &ChunkResult{Index: index}
            result.Data, result.Error = p.downloadChunk(ctx, chunk)

            results <- result
        }(i, leaf)
    }

    // 4. 收集结果
    chunks := make([][]byte, len(metadata.Leaves))
    for range metadata.Leaves {
        result := <-results
        if result.Error != nil {
            return result.Error
        }
        chunks[result.Index] = result.Data
    }

    // 5. 写入文件
    for _, data := range chunks {
        if _, err := destFile.Write(data); err != nil {
            return err
        }
    }

    return nil
}
```

#### 4.2 下载单个 Chunk

```go
func (p *P2PService) downloadChunk(ctx context.Context, chunk file.ChunkData) ([]byte, error) {
    // 1. 查找提供者
    key := fmt.Sprintf("%x", chunk.ChunkHash)
    providers := p.FindProviders(ctx, key)

    if len(providers) == 0 {
        return nil, xerrors.Errorf("no providers found for chunk")
    }

    // 2. 选择节点（考虑反吸血虫）
    providers = p.AntiLeecher.FilterProviders(providers)
    peerID := p.PeerSelector.SelectPeer(providers)

    // 3. 重试逻辑
    for retry := 0; retry < p.Config.MaxRetries; retry++ {
        data, err := p.GetChunkData(ctx, peerID, chunk.ChunkHash)
        if err == nil {
            return data, nil
        }

        logrus.Warnf("Retry %d: %v", retry+1, err)
        time.Sleep(time.Duration(retry+1) * time.Second)  // 指数退避
    }

    return nil, xerrors.Errorf("max retries exceeded")
}
```

### 5. 连接管理

#### 5.1 记录成功/失败

```go
// RecordSuccess 记录成功请求
func (cm *ConnManager) RecordSuccess(peerID peer.ID) {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()

    stats, exists := cm.peerStats[peerID]
    if !exists {
        stats = &PeerStats{FirstSeen: time.Now()}
        cm.peerStats[peerID] = stats
    }

    stats.TotalRequests++
    stats.SuccessCount++
    stats.LastSuccess = time.Now()
}

// RecordFailure 记录失败请求
func (cm *ConnManager) RecordFailure(peerID peer.ID) {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()

    stats, exists := cm.peerStats[peerID]
    if !exists {
        stats = &PeerStats{FirstSeen: time.Now()}
        cm.peerStats[peerID] = stats
    }

    stats.TotalRequests++
    stats.FailureCount++
    stats.LastFailure = time.Now()

    // 检查是否需要加入黑名单
    if stats.TotalRequests >= 10 {
        successRate := float64(stats.SuccessCount) / float64(stats.TotalRequests)
        if successRate < 0.5 {
            cm.blacklist[peerID] = time.Now()
            logrus.Warnf("Peer %s blacklisted (success rate: %.2f%%)", peerID, successRate*100)
        }
    }
}
```

#### 5.2 黑名单管理

```go
// IsBlacklisted 检查节点是否在黑名单中
func (cm *ConnManager) IsBlacklisted(peerID peer.ID) bool {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()

    expiryTime, exists := cm.blacklist[peerID]
    if !exists {
        return false
    }

    // 检查是否过期
    if time.Since(expiryTime) > cm.blacklistTTL {
        delete(cm.blacklist, peerID)
        return false
    }

    return true
}

// CleanBlacklist 清理过期的黑名单条目
func (cm *ConnManager) CleanBlacklist() {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()

    now := time.Now()
    for peerID, expiryTime := range cm.blacklist {
        if now.Sub(expiryTime) > cm.blacklistTTL {
            delete(cm.blacklist, peerID)
        }
    }
}
```

## 协议定义

### 1. Announce 协议

**协议 ID**: `p2pFileTransfer/Announce/1.0.0`

**消息格式**:
```json
{
    "chunk_hash": "abc123...",
    "peer_info": {
        "id": "QmPeerID...",
        "addrs": ["/ip4/127.0.0.1/tcp/8001"]
    }
}
```

### 2. Lookup 协议

**协议 ID**: `p2pFileTransfer/Lookup/1.0.0`

**请求**:
```json
{
    "key": "abc123..."
}
```

**响应**:
```json
{
    "providers": [
        {
            "id": "QmPeerID1...",
            "addrs": ["/ip4/1.2.3.4/tcp/8001"]
        },
        {
            "id": "QmPeerID2...",
            "addrs": ["/ip4/5.6.7.8/tcp/8001"]
        }
    ]
}
```

### 3. Chunk Data 协议

**协议 ID**: `p2pFileTransfer/ChunkData/1.0.0`

**请求**:
```json
{
    "hash": "abc123..."
}
```

**响应**:
```json
{
    "data": "base64encodeddata..."
}
```

## 配置项

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| Port | int | 0 | 监听端口（0=随机） |
| Insecure | bool | false | 是否使用不安全连接 |
| ChunkStoragePath | string | "files" | Chunk 存储路径 |
| MaxRetries | int | 3 | 最大重试次数 |
| MaxConcurrency | int | 16 | 最大并发下载数 |
| RequestTimeout | int | 5 | 请求超时（秒） |
| DataTimeout | int | 30 | 数据传输超时（秒） |
| DHTTimeout | int | 10 | DHT 操作超时（秒） |

## 测试用例

### 单元测试示例

```go
func TestNewP2PService(t *testing.T) {
    ctx := context.Background()
    config := p2p.NewP2PConfig()

    service, err := p2p.NewP2PService(ctx, config)
    assert.NoError(t, err)
    assert.NotNil(t, service)

    defer service.Shutdown()

    // 验证服务已启动
    assert.NotNil(t, service.Host)
    assert.NotNil(t, service.DHT)
}
```

### 集成测试示例

```go
func TestMultiNodeFileTransfer(t *testing.T) {
    // 创建两个节点
    node1 := createTestNode(t)
    node2 := createTestNode(t)

    defer node1.Shutdown()
    defer node2.Shutdown()

    // 节点1 上传文件
    metadata := uploadTestFile(t, node1, "test.txt")

    // 节点2 下载文件
    err := node2.DownloadFile(context.Background(), metadata, "downloaded.txt")
    assert.NoError(t, err)

    // 验证文件内容
    assert.FileEquals(t, "test.txt", "downloaded.txt")
}
```

## 扩展点

### 1. 自定义节点选择器

```go
// 实现 PeerSelector 接口
type CustomPeerSelector struct {
    // 自定义字段
}

func (s *CustomPeerSelector) SelectPeer(providers []peer.ID) peer.ID {
    // 自定义选择逻辑
    // 例如：基于延迟、带宽、地理位置等
    return selectedPeer
}

// 使用自定义选择器
service.PeerSelector = &CustomPeerSelector{}
```

### 2. 自定义反吸血虫策略

```go
// 实现 AntiLeecher 接口
type CustomAntiLeecher struct{}

func (al *CustomAntiLeecher) FilterProviders(providers []peer.ID) []peer.ID {
    // 自定义过滤逻辑
    return filteredProviders
}

service.AntiLeecher = &CustomAntiLeecher{}
```

### 3. 添加新协议

```go
// 定义新协议
const CustomProtocol = "p2pFileTransfer/Custom/1.0.0"

// 注册处理器
func (p *P2PService) CustomHandler(ctx context.Context) {
    p.Host.SetStreamHandler(CustomProtocol, func(s network.Stream) {
        defer s.Close()

        // 处理自定义协议逻辑
        // ...
    })
}
```

### 4. 集成其他存储后端

```go
// 实现 FileSystemAdapter 接口
type S3Adapter struct {
    client *s3.Client
    bucket string
}

func (a *S3Adapter) ReadFile(path string) ([]byte, error) {
    // 从 S3 读取
}

func (a *S3Adapter) WriteFile(path string, data []byte) error {
    // 写入 S3
}

// 使用自定义适配器
service.FSAdapter = &S3Adapter{...}
```

## 性能优化建议

1. **调整并发数**: 根据网络带宽调整 `MaxConcurrency`
2. **优化超时**: 根据网络延迟调整各项超时配置
3. **连接复用**: 使用连接池避免频繁建立连接
4. **缓存策略**: 缓存频繁访问的 Chunk
5. **黑名单优化**: 定期清理黑名单，释放内存

---

**相关文档**:
- [Chameleon Merkle Tree 组件](Chameleon-Merkle-Tree组件.md)
- [配置管理组件](配置管理组件.md)
