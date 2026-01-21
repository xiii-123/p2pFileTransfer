# libp2p 库的使用经验

## 目录

1. [libp2p 简介](#1-libp2p-简介)
2. [核心概念](#2-核心概念)
3. [架构设计](#3-架构设计)
4. [核心组件详解](#4-核心组件详解)
5. [实际应用案例](#5-实际应用案例)
6. [最佳实践](#6-最佳实践)
7. [常见问题与解决方案](#7-常见问题与解决方案)

---

## 1. libp2p 简介

### 1.1 什么是 libp2p？

**libp2p** 是一个模块化的 P2P 网络库，由 Protocol Labs 开发，是 IPFS（InterPlanetary File System）的底层网络框架。它提供了一整套构建 P2P 应用的工具和协议。

**核心特性**：
- ✅ **模块化设计**: 可以只使用需要的组件
- ✅ **跨平台**: 支持 Go、JavaScript、Rust、Java 等多种语言
- ✅ **加密优先**: 默认使用加密连接
- ✅ **NAT 穿透**: 内置 UPnP 和 NAT-PMP 支持
- ✅ **多路复用**: 支持在单个连接上运行多个协议
- ✅ **自寻址**: 使用 Multiaddr 格式，无需额外的 DNS

### 1.2 为什么选择 libp2p？

**优势**：
- 生产就绪，被 IPFS、Filecoin 等大型项目使用
- 活跃的社区和持续的维护
- 丰富的协议和传输层实现
- 完善的文档和示例

**适用场景**：
- 去中心化文件存储
- P2P 通信系统
- 区块链节点通信
- 分布式数据库
- 实时协作应用

### 1.3 项目中的使用

在本项目中，我们使用以下 libp2p 相关库：

| 包名 | 版本 | 用途 |
|------|------|------|
| `go-libp2p` | v0.41.1 | 核心 P2P 功能 |
| `go-libp2p-kad-dht` | v0.31.0 | Kademlia DHT 实现 |
| `go-libp2p-record` | v0.3.1 | DHT 记录验证 |
| `go-multiaddr` | v0.15.0 | 自寻址格式 |

---

## 2. 核心概念

### 2.1 Peer ID（节点标识）

**定义**：Peer ID 是 libp2p 网络中节点的唯一标识符。

**特点**：
- 从节点的公钥派生
- 全局唯一
- 不可伪造（需要私钥才能证明所有权）

**生成方式**：
```go
import (
    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/peer"
)

// 生成密钥对
privKey, _, err := crypto.GenerateKeyPairWithReader(
    crypto.RSA,  // 算法类型
    2048,        // 密钥长度
    rand.Reader, // 随机源
)

// 从公钥生成 Peer ID
peerID := peer.IDFromPublicKey(privKey.GetPublic())

fmt.Printf("Peer ID: %s\n", peerID)
// 输出示例: QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N
```

**Peer ID 格式**：
- 以 `Qm` 或 `b` 开头
- 使用 base58btc 编码
- 长度通常为 46-48 个字符

### 2.2 Multiaddr（多地址）

**定义**：Multiaddr 是一种自描述的地址格式，包含连接所需的所有信息。

**优势**：
- 自包含：不需要额外的 DNS 查询
- 递归：可以嵌套多个协议层
- 人类可读：使用文本表示
- 跨语言：所有实现都兼容

**格式示例**：
```
/ip4/127.0.0.1/tcp/8001/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N

分解：
/ip4/127.0.0.1  - IPv4 地址
/tcp/8001       - TCP 端口
/p2p/<peerID>   - 节点 ID
```

**使用示例**：
```go
import "github.com/multiformats/go-multiaddr"

// 解析地址
addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/8001")

// 封装节点 ID
peerAddr, _ := multiaddr.NewMultiaddr("/p2p/QmPeerID...")
fullAddr := addr.Encapsulate(peerAddr)

// 提取组件
protocols := addr.Protocols()
for _, p := range protocols {
    fmt.Printf("%s: %s\n", p.Name, p.Value)
}
```

**支持的协议**：
- `/ip4/<IPv4>` - IPv4 地址
- `/ip6/<IPv6>` - IPv6 地址
- `/dns/<domain>` - DNS 域名
- `/tcp/<port>` - TCP 端口
- `/udp/<port>` - UDP 端口
- `/ws` - WebSocket
- `/wss` - 安全 WebSocket
- `/p2p/<peerID>` - libp2p 节点 ID

### 2.3 Protocol（协议）

**定义**：Protocol 定义了节点间通信的协议标识符。

**特点**：
- 字符串格式：通常是 `/应用/版本`
- 用于多路复用：单个连接支持多个协议
- 可协商：节点可以协商使用的协议

**示例**：
```go
const (
    ChatProtocol      = "/chat/1.0.0"
    FileTransferProtocol = "/file-transfer/1.0.0"
    ChunkDataProtocol   = "/chunk-data/1.0.0"
)
```

### 2.4 Stream（流）

**定义**：Stream 是节点间的双向数据通道，类似于 TCP 连接。

**特点**：
- 多路复用：单个连接可以有多个流
- 协议标识：每个流关联一个协议
- 全双工：支持同时读写

**生命周期**：
1. 打开流（NewStream）
2. 协商协议（Protocol Handshake）
3. 传输数据
4. 关闭流（Close）

### 2.5 DHT（分布式哈希表）

**定义**：DHT 是一种分布式存储系统，用于存储和检索键值对。

**本项目的使用**：
- Kademlia DHT 实现
- 内容路由：查找数据的提供者
- 节点发现：发现网络中的其他节点
- Provider 系统：公告和查询数据提供者

---

## 3. 架构设计

### 3.1 libp2p 的分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                     应用层（Application）                     │
│  文件传输 | 聊天 | 区块链 | 自定义应用                        │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                   核心层（Core Host）                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Peer ID  │  │ Network  │  │  Router  │  │  Stream  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                   协议层（Protocols）                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   DHT    │  │  Kademlia│  │  mDNS    │  │  Relay   │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                  传输层（Transports）                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   TCP    │  │ WebSocket│  │   QUIC   │  │   UDP    │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                 安全层（Security）                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │  Noise   │  │   TLS    │  │  SECIO   │                  │
│  └──────────┘  └──────────┘  └──────────┘                  │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 模块化组件

libp2p 采用模块化设计，每个功能都是可选的：

**核心组件**：
- **Host**: 节点的主要抽象
- **Network**: 管理网络连接
- **Router**: 处理传入消息的路由
- **Stream**: 多路复用的数据流

**可选组件**：
- **Transports**: TCP, WebSocket, QUIC 等
- **Security**: Noise, TLS, SECIO 等加密协议
- **Multiplexers**: Yamux, Mplex 等多路复用器
- **Peer Discovery**: mDNS, DHT, Bootstrap
- **Content Routing**: Kademlia DHT
- **Relay**: 中继连接
- **NAT穿透**: UPnP, NAT-PMP
- **监控**: Prometheus metrics, tracing

### 3.3 配置系统

libp2p 使用选项模式（Option Pattern）配置节点：

```go
import "github.com/libp2p/go-libp2p"

// 配置选项
opts := []libp2p.Option{
    libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8000"),
    libp2p.Identity(privKey),
    libp2p.DefaultTransports,
    libp2p.Security(noise.ID, noise.New),
}

// 创建 Host
host, err := libp2p.New(opts...)
```

---

## 4. 核心组件详解

### 4.1 Host（主机）

**定义**：Host 是 libp2p 网络中的节点抽象，是所有操作的入口点。

**主要接口**：
```go
type Host interface {
    // ID 返回节点的 Peer ID
    ID() peer.ID

    // Addrs 返回节点的监听地址
    Addrs() []multiaddr.Multiaddr

    // Network 返回网络实例
    Network() network.Network

    // Mux 返回协议多路复用器
    Mux() protocol.Switch

    // ConnectionManager 返回连接管理器
    ConnectionManager() connmgr.ConnManager

    // NewStream 打开到指定节点的流
    NewStream(ctx context.Context, id peer.ID, protos ...protocol.ID) (network.Stream, error)

    // SetStreamHandler 设置流处理器
    SetStreamHandler(pid protocol.ID, handler network.StreamHandler)

    // Connect 连接到指定节点
    Connect(ctx context.Context, pi peer.AddrInfo) error

    // Close 关闭主机
    Close() error
}
```

**实际使用**（来自本项目）：

```go
// pkg/p2p/utils.go
func newBasicHost(listenPort int, insecure bool, randseed int64) (host.Host, error) {
    // 1. 生成密钥对
    var r io.Reader
    if randseed == 0 {
        r = rand.Reader
    } else {
        r = mrand.New(mrand.NewSource(randseed))
    }

    priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
    if err != nil {
        return nil, err
    }

    // 2. 配置选项
    opts := []libp2p.Option{
        libp2p.ListenAddrStrings(
            fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort),
        ),
        libp2p.Identity(priv),
        // libp2p.DisableRelay(),  // 可选：禁用中继
    }

    // 3. 创建 Host
    return libp2p.New(opts...)
}
```

**最佳实践**：
- ✅ 使用 `defer host.Close()` 确保资源释放
- ✅ 处理 `ListenAddrStrings` 的错误
- ✅ 生产环境使用加密连接
- ✅ 使用随机端口（`listenPort = 0`）避免冲突

### 4.2 Network（网络）

**定义**：Network 管理所有连接和流。

**主要功能**：
- 建立和关闭连接
- 管理活跃的连接
- 监听新连接

**使用示例**：
```go
// 获取 Network
network := host.Network()

// 监听连接事件
network.Notify(&network.NotifyBundle{
    ConnectedF: func(n network.Network, c network.Conn) {
        fmt.Printf("Connected to %s\n", c.RemotePeer())
    },
    DisconnectedF: func(n network.Network, c network.Conn) {
        fmt.Printf("Disconnected from %s\n", c.RemotePeer())
    },
})

// 获取所有连接
peers := network.Peers()
for _, p := range peers {
    fmt.Printf("Connected to: %s\n", p)
}
```

### 4.3 Stream（流）

**定义**：Stream 是节点间的双向数据通道，支持多路复用。

**创建流**：
```go
// 打开到指定节点的流
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

stream, err := host.NewStream(
    ctx,
    peerID,
    "my-protocol/1.0.0",  // 协议 ID
)
if err != nil {
    return fmt.Errorf("failed to open stream: %w", err)
}
defer stream.Close()

// 写入数据
writer := bufio.NewWriter(stream)
writer.WriteString("Hello, peer!\n")
writer.Flush()

// 读取数据
reader := bufio.NewReader(stream)
response, err := reader.ReadString('\n')
if err != nil {
    return fmt.Errorf("failed to read: %w", err)
}

fmt.Printf("Received: %s\n", response)
```

**处理传入流**：
```go
// 设置流处理器
host.SetStreamHandler("my-protocol/1.0.0", func(s network.Stream) {
    defer s.Close()

    // 读取请求
    reader := bufio.NewReader(s)
    request, err := reader.ReadString('\n')
    if err != nil {
        logrus.Errorf("Failed to read: %v", err)
        return
    }

    logrus.Infof("Received: %s", request)

    // 发送响应
    writer := bufio.NewWriter(s)
    writer.WriteString("OK\n")
    writer.Flush()
})
```

**流的特性**：
- **全双工**: 可以同时读写
- **多路复用**: 单个连接支持多个流
- **协议协商**: 自动协商协议版本
- **超时控制**: 支持上下文取消

**本项目中的使用**：
```go
// pkg/p2p/dht.go
host.SetStreamHandler(AnnounceProtocol, func(s network.Stream) {
    defer s.Close()

    // 处理 Announce 请求
    var msg announceMsg
    if err := json.NewDecoder(s).Decode(&msg); err != nil {
        logrus.Errorf("Failed to decode announce: %v", err)
        return
    }

    // 处理消息...
    logrus.Infof("Received announce for chunk: %s from %s",
        msg.ChunkHash, s.Conn().RemotePeer())
})
```

### 4.4 DHT（分布式哈希表）

**定义**：DHT 提供键值存储和内容路由功能。

**本项目使用 go-libp2p-kad-dht**，实现了 Kademlia DHT。

**创建 DHT**：
```go
import (
    dht "github.com/libp2p/go-libp2p-kad-dht"
    "github.com/libp2p/go-libp2p/core/protocol"
)

func newDHT(ctx context.Context, host host.Host, config P2PConfig) (*dht.IpfsDHT, error) {
    // 配置 DHT 选项
    opts := []dht.Option{
        // 设置协议前缀
        dht.ProtocolPrefix(protocol.ID(config.ProtocolPrefix)),

        // 设置验证器
        dht.NamespacedValidator(
            config.NameSpace,
            config.Validator,
        ),

        // 设置工作模式
        dht.Mode(dht.ModeAutoServer),

        // 可选：禁用自动刷新
        // dht.DisableAutoRefresh(),
    }

    // 创建 DHT 实例
    kdht, err := dht.New(ctx, host, opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to create DHT: %w", err)
    }

    // Bootstrap DHT
    if err = kdht.Bootstrap(ctx); err != nil {
        return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
    }

    return kdht, nil
}
```

**DHT 操作**：

**1. 存储值（PutValue）**：
```go
// 在 DHT 中存储键值对
ctx := context.Background()
key := "my-key"
value := []byte("my-value")

err := dht.PutValue(ctx, key, value)
if err != nil {
    return fmt.Errorf("failed to put value: %w", err)
}
```

**2. 检索值（GetValue）**：
```go
// 从 DHT 中检索值
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

value, err := dht.GetValue(ctx, key)
if err != nil {
    return fmt.Errorf("failed to get value: %w", err)
}

fmt.Printf("Retrieved: %s\n", string(value))
```

**3. 公告提供者（Provide）**：
```go
// 公告自己是某个内容的提供者
cid, _ := cid.Decode("Qm...")

err := dht.Provide(ctx, cid, true)
if err != nil {
    return fmt.Errorf("failed to provide: %w", err)
}
```

**4. 查找提供者（FindProviders）**：
```go
// 查找内容的提供者
cid, _ := cid.Decode("Qm...")

providerChan := dht.FindProvidersAsync(ctx, cid, 0)

for provider := range providerChan {
    if provider.ID == host.ID() {
        continue  // 跳过自己
    }

    fmt.Printf("Found provider: %s\n", provider.ID)
    fmt.Printf("Addresses: %v\n", provider.Addrs)
}
```

**DHT 工作模式**：
| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `ModeAutoServer` | 自动切换服务器/客户端模式 | 通用 |
| `ModeClient` | 仅客户端模式 | 仅查询，不提供数据 |
| `ModeServer` | 仅服务器模式 | 固定节点，Bootstrap |

**本项目中的 DHT 使用**：
```go
// pkg/p2p/dht.go
const (
    AnnounceProtocol = "p2pFileTransfer/Announce/1.0.0"
    LookupProtocol   = "p2pFileTransfer/Lookup/1.0.0"
)

// Announce: 公告 Chunk 提供者
func (p *P2PService) Announce(ctx context.Context, key string) error {
    ctx, cancel := context.WithTimeout(
        ctx,
        time.Duration(p.Config.DHTTimeout)*time.Second,
    )
    defer cancel()

    cid, err := cid.Decode(key)
    if err != nil {
        return err
    }

    // 通过 DHT 公告
    if err := p.DHT.Provide(ctx, cid, true); err != nil {
        return xerrors.Errorf("failed to announce: %w", err)
    }

    logrus.Infof("Announced chunk: %s", key)
    return nil
}

// FindProviders: 查找 Chunk 提供者
func (p *P2PService) FindProviders(ctx context.Context, key string) []peer.ID {
    ctx, cancel := context.WithTimeout(
        ctx,
        time.Duration(p.Config.DHTTimeout)*time.Second,
    )
    defer cancel()

    cid, err := cid.Decode(key)
    if err != nil {
        return nil
    }

    // 查找提供者
    providerChan := p.DHT.FindProvidersAsync(ctx, cid, 0)

    var providers []peer.ID
    for provider := range providerChan {
        if provider.ID == p.Host.ID() {
            continue
        }
        providers = append(providers, provider.ID)
    }

    return providers
}
```

### 4.5 Transport（传输层）

**定义**：Transport 定义了节点如何建立底层连接。

**支持的传输**：
- **TCP**: 最常用的传输方式
- **WebSocket**: 适用于浏览器环境
- **QUIC**: 基于 UDP 的快速传输
- **UDP**: 用于自定义协议

**使用 TCP 传输**：
```go
// 默认已启用 TCP
opts := []libp2p.Option{
    libp2p.DefaultTransports,  // 包含 TCP
    libp2p.Transport(quic.NewTransport),  // 添加 QUIC
}

host, err := libp2p.New(opts...)
```

**配置监听地址**：
```go
// 监听所有接口的随机端口
libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0")

// 监听特定端口
libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/8000")

// 监听多个地址
libp2p.ListenAddrStrings(
    "/ip4/0.0.0.0/tcp/8000",
    "/ip6::/tcp/8000",
)
```

### 4.6 Security（安全层）

**定义**：Security 提供连接加密和认证。

**支持的协议**：
- **Noise**: 默认推荐，性能好
- **TLS**: 标准 TLS 1.3
- **SECIO**: 早期协议，已弃用

**使用 Noise 加密**：
```go
import (
    "github.com/libp2p/go-libp2p/p2p/security/noise"
)

opts := []libp2p.Option{
    libp2p.Security(noise.ID, noise.New),
}

host, err := libp2p.New(opts...)
```

**不安全模式（仅测试）**：
```go
// ⚠️ 生产环境不要使用！
opts := []libp2p.Option{
    libp2p.NoSecurity,  // 禁用加密
}

host, err := libp2p.New(opts...)
```

**本项目中的安全配置**：
```go
// pkg/p2p/utils.go
// 生产环境使用加密连接（默认启用）
if !insecure {
    // libp2p 默认使用 Noise 加密
    // 无需额外配置
}

// 开发环境可以禁用加密
if insecure {
    opts = append(opts, libp2p.NoSecurity)
}
```

### 4.7 Multiplexer（多路复用）

**定义**：Multiplexer 在单个连接上多路复用多个流。

**支持的多路复用器**：
- **Yamux**: 默认推荐，性能好
- **Mplex**: 早期协议

**配置多路复用器**：
```go
import (
    "github.com/libp2p/go-yamux"
)

opts := []libp2p.Option{
    libp2p.Multiplexer(yamux.DefaultTransport),
}

host, err := libp2p.New(opts...)
```

**多路复用的优势**：
- 单个连接支持多个并发流
- 减少连接开销
- 提高性能

### 4.8 Peer Discovery（节点发现）

**定义**：发现网络中的其他节点。

**方法**：
1. **Bootstrap**: 使用预配置的引导节点
2. **mDNS**: 本地网络发现
3. **DHT**: 通过 DHT 发现

**配置 Bootstrap**：
```go
opts := []libp2p.Option{
    libp2p.BootstrapPeers(
        // Bootstrap 节点地址
        "/ip4/1.2.3.4/tcp/8001/p2p/QmPeerID1",
        "/ip4/5.6.7.8/tcp/8001/p2p/QmPeerID2",
    ),
}
```

**使用 mDNS 发现**：
```go
import (
    "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// 启用 mDNS
service := mdns.NewMdnsService(
    host,
    time.Second*10,  // 间隔
    "",              // 服务标签（空表示所有）
)

if err := service.Start(); err != nil {
    logrus.Errorf("Failed to start mDNS: %v", err)
}
```

---

## 5. 实际应用案例

### 5.1 创建完整的 P2P 节点

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/host"
    "github.com/libp2p/go-libp2p/core/network"
    "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
    dht "github.com/libp2p/go-libp2p-kad-dht"
    "github.com/multiformats/go-multiaddr"
)

func createHost(listenPort int) (host.Host, error) {
    // 1. 生成密钥对
    priv, _, err := crypto.GenerateKeyPairWithReader(
        crypto.RSA,
        2048,
        rand.Reader,
    )
    if err != nil {
        return nil, err
    }

    // 2. 配置选项
    opts := []libp2p.Option{
        // 监听地址
        libp2p.ListenAddrStrings(
            fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort),
        ),

        // 设置身份
        libp2p.Identity(priv),

        // 使用默认传输（TCP, WebSocket）
        libp2p.DefaultTransports,

        // 使用 Noise 加密
        libp2p.Security(noise.ID, noise.New),

        // 使用 Yamux 多路复用
        libp2p.Multiplexer(yamux.DefaultTransport),

        // 支持中继
        // libp2p.EnableRelay(),
    }

    // 3. 创建 Host
    host, err := libp2p.New(opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to create host: %w", err)
    }

    return host, nil
}

func setupDHT(ctx context.Context, host host.Host) (*dht.IpfsDHT, error) {
    // 1. 配置 DHT
    opts := []dht.Option{
        dht.Mode(dht.ModeAutoServer),
    }

    // 2. 创建 DHT
    kdht, err := dht.New(ctx, host, opts...)
    if err != nil {
        return nil, err
    }

    // 3. Bootstrap
    if err = kdht.Bootstrap(ctx); err != nil {
        return nil, err
    }

    log.Println("DHT initialized")
    return kdht, nil
}

func setupDiscovery(host host.Host) {
    // 启动 mDNS 发现
    service := mdns.NewMdnsService(host, time.Second*10, "")
    if err := service.Start(); err != nil {
        log.Printf("Failed to start mDNS: %v", err)
    }
}

func setupProtocolHandlers(host host.Host) {
    // 设置流处理器
    host.SetStreamHandler("/echo/1.0.0", func(s network.Stream) {
        defer s.Close()

        log.Printf("New stream from %s\n", s.Conn().RemotePeer())

        // 回显数据
        buf := make([]byte, 1024)
        n, err := s.Read(buf)
        if err != nil {
            log.Printf("Read error: %v", err)
            return
        }

        s.Write(buf[:n])
        log.Printf("Echoed: %s\n", string(buf[:n]))
    })
}

func main() {
    ctx := context.Background()

    // 1. 创建 Host
    host, err := createHost(0)
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()

    log.Printf("Host created. ID: %s\n", host.ID())

    // 2. 设置 DHT
    dht, err := setupDHT(ctx, host)
    if err != nil {
        log.Fatal(err)
    }
    _ = dht

    // 3. 设置节点发现
    setupDiscovery(host)

    // 4. 设置协议处理器
    setupProtocolHandlers(host)

    // 5. 打印监听地址
    addrs := host.Addrs()
    for _, addr := range addrs {
        fullAddr := addr.Encapsulate(
            multiaddr.StringCast("/p2p/" + host.ID().String()),
        )
        log.Printf("Listening on: %s\n", fullAddr)
    }

    // 6. 保持运行
    select {}
}
```

### 5.2 连接到其他节点

```go
// 连接到指定节点
func connectToPeer(host host.Host, targetAddr string) error {
    // 1. 解析目标地址
    multiAddr, err := multiaddr.NewMultiaddr(targetAddr)
    if err != nil {
        return fmt.Errorf("invalid multiaddr: %w", err)
    }

    // 2. 提取 Peer Info
    peerInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
    if err != nil {
        return fmt.Errorf("failed to extract peer info: %w", err)
    }

    // 3. 连接
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := host.Connect(ctx, peerInfo); err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }

    log.Printf("Connected to %s\n", peerInfo.ID)
    return nil
}

// 使用示例
targetAddr := "/ip4/127.0.0.1/tcp/8001/p2p/QmPeerID..."
if err := connectToPeer(host, targetAddr); err != nil {
    log.Fatal(err)
}
```

### 5.3 发送和接收消息

**发送消息**：
```go
func sendMessage(host host.Host, peerID peer.ID, protocol string, message []byte) error {
    // 1. 打开流
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    stream, err := host.NewStream(ctx, peerID, protocol.ID(protocol))
    if err != nil {
        return fmt.Errorf("failed to open stream: %w", err)
    }
    defer stream.Close()

    // 2. 写入消息
    _, err = stream.Write(message)
    if err != nil {
        return fmt.Errorf("failed to write: %w", err)
    }

    log.Printf("Sent message to %s\n", peerID)
    return nil
}
```

**接收消息**：
```go
// 在 SetStreamHandler 中接收
host.SetStreamHandler("/chat/1.0.0", func(s network.Stream) {
    defer s.Close()

    remotePeer := s.Conn().RemotePeer()
    log.Printf("New stream from %s\n", remotePeer)

    // 读取消息
    buf := make([]byte, 1024)
    n, err := s.Read(buf)
    if err != nil {
        log.Printf("Read error: %v\n", err)
        return
    }

    log.Printf("Received from %s: %s\n", remotePeer, string(buf[:n]))

    // 可选：回复
    s.Write([]byte("Message received!\n"))
})
```

---

## 6. 最佳实践

### 6.1 资源管理

**1. 始终关闭 Host**：
```go
host, err := libp2p.New(opts...)
if err != nil {
    log.Fatal(err)
}
defer host.Close()  // ✅ 确保资源释放
```

**2. 使用 Context 控制超时**：
```go
// ✅ 设置超时
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

stream, err := host.NewStream(ctx, peerID, protocolID)
```

**3. 关闭 Stream**：
```go
stream, err := host.NewStream(ctx, peerID, protocolID)
if err != nil {
    return err
}
defer stream.Close()  // ✅ 确保关闭
```

### 6.2 错误处理

**1. 检查连接错误**：
```go
err := host.Connect(ctx, peerInfo)
if err != nil {
    // 区分错误类型
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("Connection timeout")
    } else if errors.Is(err, context.Canceled) {
        log.Println("Connection canceled")
    } else {
        log.Printf("Connection error: %v\n", err)
    }
    return err
}
```

**2. 处理流错误**：
```go
n, err := stream.Read(buf)
if err != nil {
    if err == io.EOF {
        log.Println("Peer closed stream")
    } else {
        log.Printf("Read error: %v\n", err)
    }
    return err
}
```

### 6.3 性能优化

**1. 使用连接池**：
```go
type StreamPool struct {
    host    host.Host
    peerID  peer.ID
    proto   protocol.ID
    streams chan network.Stream
    mu      sync.Mutex
}

func NewStreamPool(host host.Host, peerID peer.ID, proto protocol.ID, size int) *StreamPool {
    return &StreamPool{
        host:    host,
        peerID:  peerID,
        proto:   proto,
        streams: make(chan network.Stream, size),
    }
}

func (p *StreamPool) Get(ctx context.Context) (network.Stream, error) {
    select {
    case stream := <-p.streams:
        return stream, nil
    default:
        return p.host.NewStream(ctx, p.peerID, p.proto)
    }
}

func (p *StreamPool) Put(stream network.Stream) {
    select {
    case p.streams <- stream:
        // 放回池中
    default:
        stream.Close()  // 池满，关闭
    }
}
```

**2. 批量操作**：
```go
// 批量查找提供者
func findProvidersBatch(dht *dht.IpfsDHT, cids []cid.Cid) map[peer.ID][]cid.Cid {
    providerMap := make(map[peer.ID][]cid.Cid)
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, c := range cids {
        wg.Add(1)
        go func(cid cid.Cid) {
            defer wg.Done()

            providers := findProviders(dht, cid)

            mu.Lock()
            for _, p := range providers {
                providerMap[p] = append(providerMap[p], cid)
            }
            mu.Unlock()
        }(c)
    }

    wg.Wait()
    return providerMap
}
```

**3. 缓存连接**：
```go
type ConnCache struct {
    conns map[peer.ID]network.Conn
    mu    sync.RWMutex
    ttl   time.Duration
}

func (c *ConnCache) Get(peerID peer.ID) (network.Conn, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    conn, exists := c.conns[peerID]
    return conn, exists
}

func (c *ConnCache) Set(peerID peer.ID, conn network.Conn) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.conns[peerID] = conn

    // 自动过期
    go func() {
        time.Sleep(c.ttl)
        c.mu.Lock()
        delete(c.conns, peerID)
        c.mu.Unlock()
    }()
}
```

### 6.4 安全性

**1. 始终使用加密**：
```go
// ✅ 生产环境
opts := []libp2p.Option{
    libp2p.Security(noise.ID, noise.New),
}

// ❌ 不要禁用加密
opts = append(opts, libp2p.NoSecurity)
```

**2. 验证节点身份**：
```go
// 检查 Peer ID
expectedID, _ := peer.Decode("QmExpectedID...")
if peerID != expectedID {
    return fmt.Errorf("peer ID mismatch")
}
```

**3. 使用 TLS 证书**（高级）：
```go
import "github.com/libp2p/go-libp2p/p2p/security/tls"

// 使用 TLS 证书
cert, _ := tls.NewIdentity(privKey)
opts := []libp2p.Option{
    libp2p.Security(tls.ID, tls.New),
}
```

### 6.5 日志和监控

**1. 结构化日志**：
```go
logrus.WithFields(logrus.Fields{
    "peer":    peerID,
    "protocol": protocolID,
    "action":  "new_stream",
}).Info("Opening new stream")
```

**2. 监控指标**：
```go
import "github.com/prometheus/client_golang/prometheus"

var (
    streamCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "libp2p_streams_total",
            Help: "Total number of streams",
        },
        []string{"protocol", "peer"},
    )
)

// 在创建流时记录
streamCounter.WithLabelValues(string(protocolID), string(peerID)).Inc()
```

**3. 事件追踪**：
```go
// 使用 OpenTelemetry
import "go.opentelemetry.io/trace"

tracer := trace.Tracer("libp2p")

ctx, span := tracer.Start(ctx, "NewStream")
defer span.End()

stream, err := host.NewStream(ctx, peerID, protocolID)
// ...
```

---

## 7. 常见问题与解决方案

### 7.1 连接问题

**问题1: "connection refused"**

**原因**：
- 目标节点未监听
- 防火墙阻止
- 端口错误

**解决方案**：
```go
// 1. 检查目标地址是否有效
_, err := multiaddr.NewMultiaddr(targetAddr)
if err != nil {
    return fmt.Errorf("invalid multiaddr: %w", err)
}

// 2. 检查防火墙
// Linux: sudo ufw allow 8000/tcp
// Windows: 控制面板 → 防火墙 → 入站规则

// 3. 使用 ping 测试连接
if err := host.Ping(ctx, peerID); err != nil {
    return fmt.Errorf("ping failed: %w", err)
}
```

**问题2: 连接超时**

**原因**：
- 网络延迟高
- 节点负载高
- NAT 穿透失败

**解决方案**：
```go
// 增加超时时间
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

// 或使用无限超时（谨慎使用）
ctx := context.Background()
```

**问题3: 无法穿透 NAT**

**原因**：
- 节点在 NAT 后
- 没有启用中继
- UPnP 失败

**解决方案**：
```go
// 启用中继
opts := []libp2p.Option{
    libp2p.EnableRelay(),
    libp2p.EnableRelayService(),
}

// 或使用 AutoRelay
opts = append(opts, libp2p.AutoRelay())
```

### 7.2 DHT 问题

**问题1: DHT Bootstrap 失败**

**原因**：
- 没有配置 Bootstrap 节点
- Bootstrap 节点不可用
- 网络问题

**解决方案**：
```go
// 1. 配置 Bootstrap 节点
opts = []libp2p.Option{
    libp2p.BootstrapPeers(
        "/ip4/1.2.3.4/tcp/8001/p2p/QmPeerID1",
        "/ip4/5.6.7.8/tcp/8001/p2p/QmPeerID2",
    ),
}

// 2. 使用公共 Bootstrap 节点
// libp2p 提供了默认的 Bootstrap 节点
// libp2p.DefaultBootstrapPeers

// 3. 检查网络连接
if err := dht.Bootstrap(ctx); err != nil {
    log.Printf("DHT bootstrap failed: %v\n", err)
}
```

**问题2: FindProviders 找不到提供者**

**原因**：
- 内容未公告
- DHT 未完全初始化
- 网络规模太小

**解决方案**：
```go
// 1. 确保内容已公告
err := dht.Provide(ctx, cid, true)
if err != nil {
    return fmt.Errorf("failed to provide: %w", err)
}

// 2. 等待 DHT 初始化
time.Sleep(10 * time.Second)

// 3. 增加查找超时
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

providers := dht.FindProvidersAsync(ctx, cid, 0)
```

### 7.3 内存问题

**问题1: 内存泄漏**

**原因**：
- 未关闭 Stream
- 未关闭 Host
- Goroutine 泄漏

**解决方案**：
```go
// 1. 确保 Stream 关闭
stream, err := host.NewStream(ctx, peerID, protocolID)
if err != nil {
    return err
}
defer stream.Close()  // ✅ 确保关闭

// 2. 使用 pprof 监控内存
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// 3. 定期检查内存
var m runtime.MemStats
runtime.ReadMemStats(&m)
log.Printf("Memory usage: %d MB\n", m.Alloc/1024/1024)
```

### 7.4 性能问题

**问题1: 延迟高**

**原因**：
- 网络延迟
- 协议开销
- 未优化配置

**解决方案**：
```go
// 1. 使用 QUIC（更快）
import "github.com/libp2p/go-libp2p/p2p/transport/quic"

opts := []libp2p.Option{
    libp2p.Transport(quic.NewTransport),
}

// 2. 调整多路复用器配置
import "github.com/libp2p/go-yamux"

yamuxCfg := yamux.Config{
    AcceptBacklog:          512,
    EnableKeepAlive:        true,
    KeepAliveInterval:      30 * time.Second,
    ConnectionWriteTimeout: 10 * time.Second,
    MaxStreamWindowSize:    256 * 1024,
}

opts = append(opts, libp2p.Multiplexer(
    yamux.DefaultTransport.WithConfig(yamuxCfg),
))
```

**问题2: 吞吐量低**

**原因**：
- 单流传输
- 缓冲区太小
- 未使用并发

**解决方案**：
```go
// 1. 使用多个并发流
func parallelTransfer(host host.Host, peerID peer.ID, data [][]byte) error {
    var wg sync.WaitGroup
    sem := make(chan struct{}, 10)  // 限制并发数

    for _, chunk := range data {
        wg.Add(1)
        sem <- struct{}{}

        go func(d []byte) {
            defer wg.Done()
            defer func() { <-sem }()

            stream, _ := host.NewStream(ctx, peerID, protocolID)
            defer stream.Close()

            stream.Write(d)
        }(chunk)
    }

    wg.Wait()
    return nil
}

// 2. 增大缓冲区
buf := make([]byte, 128*1024)  // 128KB 缓冲区
```

### 7.5 兼容性问题

**问题1: 协议版本不匹配**

**原因**：
- 节点使用不同的协议版本
- 协议 ID 不一致

**解决方案**：
```go
// 1. 定义清晰的协议版本
const (
    ChatProtocolV1 = "/chat/1.0.0"
    ChatProtocolV2 = "/chat/2.0.0"
)

// 2. 支持多个版本
host.SetStreamHandler(ChatProtocolV1, handleV1)
host.SetStreamHandler(ChatProtocolV2, handleV2)

// 3. 在 NewStream 时尝试多个版本
for _, proto := range []protocol.ID{ChatProtocolV2, ChatProtocolV1} {
    stream, err := host.NewStream(ctx, peerID, proto)
    if err == nil {
        return stream, nil
    }
}
```

**问题2: Multiaddr 解析失败**

**原因**：
- 格式错误
- 不支持的协议
- 缺少组件

**解决方案**：
```go
// 1. 验证 Multiaddr
func validateMultiaddr(addr string) error {
    ma, err := multiaddr.NewMultiaddr(addr)
    if err != nil {
        return fmt.Errorf("invalid multiaddr: %w", err)
    }

    // 检查必需组件
    required := []string{"/ip4", "/tcp", "/p2p"}
    for _, req := range required {
        found := false
        for _, p := range ma.Protocols() {
            if p.Name == req {
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("missing component: %s", req)
        }
    }

    return nil
}
```

---

## 8. 进阶主题

### 8.1 自定义协议

**定义协议消息格式**：
```go
type Message struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload"`
    ID      string      `json:"id"`
}
```

**实现协议处理器**：
```go
func handleProtocol(s network.Stream) {
    defer s.Close()

    decoder := json.NewDecoder(s)
    encoder := json.NewEncoder(s)

    for {
        var msg Message
        if err := decoder.Decode(&msg); err != nil {
            if err == io.EOF {
                break
            }
            log.Printf("Decode error: %v\n", err)
            return
        }

        // 处理消息
        response := processMessage(msg)

        // 发送响应
        if err := encoder.Encode(response); err != nil {
            log.Printf("Encode error: %v\n", err)
            return
        }
    }
}
```

### 8.2 Relay（中继）

**启用中继**：
```go
import (
    "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

opts := []libp2p.Option{
    // 作为中继节点
    libp2p.EnableRelayService(),

    // 使用中继
    libp2p.EnableRelay(),
}
```

### 8.3 NAT 穿透

**配置 UPnP**：
```go
import "github.com/libp2p/go-nat"

// 自动配置 NAT 穿透
natManager, err := nat.NewAutoNAT(ctx, host)
if err != nil {
    log.Printf("Failed to create NAT manager: %v\n", err)
} else {
    go natManager.Serve()
}
```

### 8.4 监控和追踪

**Prometheus 指标**：
```go
import (
    "github.com/libp2p/go-libp2p/p2p/metricsh"
    "github.com/prometheus/client_golang/prometheus"
)

// 启用指标
opts := []libp2p.Option{
    libp2p.PrometheusRegisterer(prometheus.DefaultRegisterer),
    libp2p.BandwidthReporter(metricsh.NewBandwidthCounter()),
}
```

**OpenTelemetry 追踪**：
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

// 创建 tracer
tracer := otel.Tracer("libp2p")

// 追踪操作
ctx, span := tracer.Start(ctx, "Connect")
defer span.End()

err := host.Connect(ctx, peerInfo)
if err != nil {
    span.RecordError(err)
}
```

---

## 9. 总结

### 9.1 libp2p 的优势

1. **模块化设计**: 只使用需要的组件
2. **加密优先**: 默认安全
3. **跨平台**: 多语言支持
4. **生产就绪**: 被 IPFS 等大型项目验证
5. **活跃社区**: 持续更新和支持

### 9.2 学习建议

1. **从简单开始**: 先实现基本的连接和通信
2. **阅读示例**: 参考 libp2p 的示例代码
3. **实验**: 尝试不同的配置和选项
4. **测试**: 使用多节点测试网络行为
5. **监控**: 使用日志和指标理解运行情况

### 9.3 参考资源

**官方资源**：
- [libp2p 官网](https://libp2p.io/)
- [Go-libp2p GitHub](https://github.com/libp2p/go-libp2p)
- [libp2p 文档](https://docs.libp2p.io/)
- [IPFS 工程](https://github.com/ipfs/ipfs)

**示例项目**：
- [IPFS](https://github.com/ipfs/ipfs)
- [Filecoin](https://github.com/filecoin-project/lotus)
- [本项目的 P2P 实现](https://github.com/yourusername/p2pFileTransfer)

**教程**：
- [libp2p 教程](https://docs.libp2p.io/concepts/)
- [Go-libp2p 示例](https://github.com/libp2p/go-libp2p/tree/master/examples)
- [P2P 技术博客](https://blog.libp2p.io/)

---

**文档版本**: v1.0.0
**最后更新**: 2026-01-15
**作者**: P2P File Transfer Team
