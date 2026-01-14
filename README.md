# P2P File Transfer

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> 基于 libp2p 的去中心化 P2P 文件传输系统，支持 Chunk 分块传输、DHT 路由、连接管理和完整性验证。

## 项目背景

**EthosNet** 是一个基于区块链的去中心化存储网络，用户在区块链上存储文件 Hash 等元信息，并在链下进行点对点文件传输。本项目是 EthosNet 的核心子模块，负责实现高效的分布式文件传输。

### 网络角色

- **用户**：上传文件到网络
- **存储节点**：按照规则存储文件块并提交存储证明
- **轻节点**：只存储感兴趣的文件，不提交存储证明

### 核心流程

1. **上传文件**：文件分块 → 生成元数据 → 元数据上链
2. **存储**：存储节点获取文件块 → 定期提交存储证明
3. **下载**：任意节点从链上获取元数据 → 查找存储节点 → 下载文件块

## 核心特性

### 高性能传输

- **流式下载**：大文件内存占用仅 32KB（相比 1GB 降低 99.997%）
- **并发控制**：工作池模式，goroutine 数量恒定为 16（降低 99.84%）
- **智能重试**：指数退避 + 错误分类，避免无效重试
- **Chunk 验证**：SHA256 哈希确保数据完整性

### 可靠性保障

- **超时保护**：全方位超时控制（请求、数据、DHT）
- **连接管理**：自动黑名单失败节点，统计成功率
- **优雅关闭**：完整的资源释放和生命周期管理
- **并发安全**：消除所有竞态条件

### P2P 网络功能

- **DHT 路由**：基于 Kademlia 的分布式哈希表
- **节点发现**：自动发现和连接网络节点
- **Chunk 公告**：向网络广播文件块可用性
- **Provider 查询**：查找文件块的提供节点

### 多种下载模式

- **顺序下载**：按顺序接收和写入
- **随机下载**：支持并发随机位置写入
- **流式下载**：适合大文件，低内存占用
- **进度报告**：实时回调下载进度

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/yourusername/p2pFileTransfer.git
cd p2pFileTransfer

# 下载依赖
go mod download

# 编译
go build -o p2p-server ./cmd/server/main.go
```

### 基本使用

```go
package main

import (
    "context"
    "fmt"
    "os"
    "p2pFileTransfer/pkg/p2p"
)

func main() {
    ctx := context.Background()

    // 创建配置
    config := p2p.NewP2PConfig()
    config.ChunkStoragePath = "files"
    config.MaxConcurrency = 16
    config.MaxRetries = 3

    // 创建 P2P 服务
    service, err := p2p.NewP2PService(ctx, config)
    if err != nil {
        panic(err)
    }
    defer service.Shutdown()

    // 下载文件
    file, _ := os.Create("downloaded_file.bin")
    defer file.Close()

    fileHash := "QmXYZ..." // 从链上获取的文件哈希
    err = service.GetFileOrdered(ctx, fileHash, file)
    if err != nil {
        panic(err)
    }

    fmt.Println("File downloaded successfully!")
}
```

### 使用配置文件

```bash
# 1. 复制配置模板
cp config/config.example.yaml config/config.yaml

# 2. 修改配置（可选）
vim config/config.yaml

# 3. 启动服务
go run cmd/server/main.go

# 或使用环境变量
P2P_PORT=8000 P2P_LOG_LEVEL=debug go run cmd/server/main.go
```

完整配置指南：[CONFIGURATION_GUIDE.md](CONFIGURATION_GUIDE.md)

## API 使用示例

### 基础下载

```go
// 顺序下载（适合小文件)
err := service.GetFileOrdered(ctx, fileHash, writer)
```

### 带进度报告

```go
// 实时进度回调
err = service.GetFileOrderedWithProgress(ctx, fileHash, file,
    func(downloaded, total int64, chunkIndex, totalChunks int) {
        percent := float64(downloaded) / float64(total) * 100
        fmt.Printf("Progress: %.2f%% (%d/%d chunks)\n",
            percent, chunkIndex, totalChunks)
    },
)
```

### 流式下载大文件

```go
// 内存占用仅 32KB，适合 GB 级文件
err = service.DownloadFileStreaming(ctx, fileHash, writer,
    func(downloaded, total int64, chunkIndex, totalChunks int) {
        fmt.Printf("Progress: %.2f%%\n",
            float64(downloaded)/float64(total)*100)
    },
)
```

## 配置管理

### 主要配置项

| 类别 | 配置项 | 默认值 | 说明 |
|-----|--------|--------|------|
| 网络 | port | 0 | 监听端口（0=随机） |
| 网络 | insecure | false | 不安全连接（仅开发） |
| 存储 | chunk_path | "files" | Chunk 存储路径 |
| 性能 | max_concurrency | 16 | 最大并发下载数 |
| 性能 | max_retries | 3 | 最大重试次数 |
| 日志 | level | "info" | 日志级别 |

### 环境变量示例

```bash
# 网络配置
P2P_PORT=8000
P2P_INSECURE=false

# 性能配置
P2P_MAX_CONCURRENCY=32
P2P_MAX_RETRIES=5

# 日志配置
P2P_LOG_LEVEL=debug
P2P_LOG_FORMAT=json
```

## 测试

### 运行测试

```bash
# 运行所有测试
go test ./test -v

# 运行单元测试（快速）
go test ./test -v -short

# 运行多节点测试
go test ./test -v -run TestMultiNode -timeout 10m

# 使用脚本运行多节点测试
./run_multinode_tests.sh  # Linux/macOS
run_multinode_tests.bat    # Windows
```

### 测试覆盖

- **40+ 测试用例**：集成测试、单元测试、性能测试
- **7 个多节点测试**：验证真实 P2P 网络环境
- **100% 通过率**：所有测试均通过
- **并发安全**：使用 `-race` 标志验证

## 项目结构

```
p2pFileTransfer/
├── cmd/
│   └── server/
│       └── main.go              # 主程序入口
├── config/
│   ├── config.yaml              # 配置文件
│   └── config.example.yaml      # 配置模板
├── pkg/
│   ├── p2p/                     # P2P 网络核心
│   ├── file/                    # 文件处理
│   ├── config/                  # 配置管理
│   └── chameleonMerkleTree/    # Merkle 树
├── test/                        # 测试套件
├── run_multinode_tests.bat     # 测试脚本（Windows）
├── run_multinode_tests.sh      # 测试脚本（Linux/macOS）
├── README.md                    # 本文档
├── CONFIGURATION_GUIDE.md      # 配置指南
└── go.mod
```

## 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| 网络层 | libp2p | 去中心化 P2P 网络 |
| DHT | go-libp2p-kad-dht | Kademlia 分布式哈希表 |
| 哈希 | SHA256 + Chameleon Hash | 数据完整性验证 |
| 加密 | elliptic.P256 | 椭圆曲线加密 |
| 日志 | logrus | 结构化日志 |
| 配置 | viper | 配置管理 |
| 测试 | testify + testing | 单元和集成测试 |

## 性能指标

### 优化效果

| 指标 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| 内存占用 (1GB文件) | ~1 GB | 32 KB | 99.997% ↓ |
| Goroutine 数量 (10K chunks) | 10,000 | 16 | 99.84% ↓ |
| 网络开销 | 重复 DHT 查询 | 一次查询复用 | 50% ↓ |
| Chunk 下载延迟 | - | < 100 ms | - |

### 资源使用

- **内存**：~100MB（5节点多节点测试）
- **Goroutine**：恒定 16 个（工作池模式）
- **临时文件**：自动清理
- **连接数**：每个节点最多 5 个并发流

## 文档

- [CONFIGURATION_GUIDE.md](CONFIGURATION_GUIDE.md) - 完整配置指南
- [test/README.md](test/README.md) - 测试文档
- [test/MULTINODE_TESTS.md](test/MULTINODE_TESTS.md) - 多节点测试文档

## 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 开发环境

- Go 1.23+
- 启用 Go Modules
- 运行测试确保无回归

## 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 相关链接

- [libp2p 官方文档](https://docs.libp2p.io/)
- [Kademlia DHT 论文](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf)
- [Go 最佳实践](https://golang.org/doc/effective_go.html)

## 致谢

感谢以下开源项目：

- [libp2p](https://github.com/libp2p/go-libp2p) - P2P 网络库
- [logrus](https://github.com/sirupsen/logrus) - 日志库
- [viper](https://github.com/spf13/viper) - 配置管理
- [testify](https://github.com/stretchr/testify) - 测试框架

---

**当前版本**: v1.0.0 | **最后更新**: 2026-01-14
