# P2P File Transfer System

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> 基于 libp2p 的去中心化 P2P 文件传输系统，支持 Chunk 分块传输、DHT 路由、双 Merkle 树（Regular 和 Chameleon）及完整性验证。

## ✨ 特性

### 🚀 CLI 命令行工具

完整的命令行界面，支持文件操作、DHT 管理和节点管理：

- **文件上传** - 支持两种 Merkle 树类型
- **灵活配置** - 命令行参数、环境变量、配置文件
- **进度显示** - 实时上传进度
- **元数据管理** - 自动生成和管理文件元数据

### 🌳 双 Merkle 树支持

#### Regular Merkle Tree（标准）
- SHA256 哈希，不可变
- 适合一次性上传和长期存储
- 性能优异，简单高效

#### Chameleon Merkle Tree（可编辑）
- 基于椭圆曲线 P256 的 Chameleon 哈希
- 支持文件内容修改（需私钥）
- 密钥对自动生成和管理
- 适合需要版本控制的场景

### ⚡ 高性能传输

- **流式下载** - 大文件内存占用仅 32KB
- **并发控制** - 工作池模式，goroutine 数量恒定
- **智能重试** - 指数退避 + 错误分类
- **Chunk 验证** - SHA256 哈希确保完整性

### 🔒 可靠性保障

- **超时保护** - 全方位超时控制
- **连接管理** - 自动黑名单失败节点
- **优雅关闭** - 完整的资源释放
- **并发安全** - 消除所有竞态条件

### 🌐 P2P 网络功能

- **DHT 路由** - Kademlia 分布式哈希表
- **节点发现** - 自动发现网络节点
- **Chunk 公告** - 广播文件块可用性
- **Provider 查询** - 查找文件块提供者

### 🌍 HTTP API 服务

完整的 RESTful API，支持文件上传、下载和分片操作：

- **文件管理** - 上传、下载、查询文件信息
- **分片操作** - 按需下载单个分片，支持断点续传
- **节点管理** - 查看节点信息和对等连接
- **DHT 操作** - 查询提供者、公告内容

**新功能** ⭐:
- `GET /api/v1/chunks/{hash}/download` - 下载单个分片
- `GET /api/v1/chunks/{hash}` - 查询分片信息

详细 API 文档: [API_DOCUMENTATION.md](API_DOCUMENTATION.md)

---

## 📦 安装

### 前置要求

- Go 1.23 或更高版本
- Git（用于克隆仓库）

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/xiii-123/p2pFileTransfer.git
cd p2pFileTransfer

# 下载依赖
go mod download

# 构建 CLI 工具和服务器
go build -o bin/p2p ./cmd/p2p
go build -o bin/p2p-server ./cmd/server
go build -o bin/api.exe ./cmd/api
```

### 快速验证

```bash
# 查看版本
./bin/p2p version

# 查看帮助
./bin/p2p --help
```

---

## 🚀 快速开始

### 1. 使用 CLI 工具上传文件

```bash
# 使用 Regular Merkle Tree 上传（推荐）
./bin/p2p file upload myfile.txt -t regular -d "My important file"

# 使用 Chameleon Merkle Tree 上传（可编辑）
./bin/p2p file upload myfile.txt -t chameleon -d "Editable version"

# 显示上传进度
./bin/p2p file upload largefile.zip -p -t regular

# 自定义分块大小
./bin/p2p file upload largefile.zip --chunk-size 524288 -t regular
```

**上传后生成的文件**：
- `metadata/<CID>.json` - 文件元数据
- `metadata/<CID>.key` - 私钥（仅 Chameleon 模式）
- `files/<chunkHash>` - 文件块数据

### 2. 启动 P2P 服务

```bash
# 使用默认配置
./bin/p2p-server

# 使用自定义配置
./bin/p2p-server --config config/config.yaml

# 指定端口
./bin/p2p-server --port 8000
```

### 3. 从代码中使用

```go
package main

import (
    "context"
    "fmt"
    "p2pFileTransfer/pkg/p2p"
)

func main() {
    ctx := context.Background()

    // 创建配置
    config := p2p.NewP2PConfig()
    config.ChunkStoragePath = "files"
    config.MaxConcurrency = 16

    // 创建服务
    service, err := p2p.NewP2PService(ctx, config)
    if err != nil {
        panic(err)
    }
    defer service.Shutdown()

    // 上传文件
    // ... (使用 API 进行文件操作)
}
```

### 4. 使用 HTTP API

```bash
# 启动 HTTP API 服务（默认端口 8080）
./bin/api.exe

# 上传文件
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@myfile.txt" \
  -F "tree_type=chameleon" \
  -F "description=My file"

# 查询文件信息
curl http://localhost:8080/api/v1/files/{cid}

# 下载完整文件
curl http://localhost:8080/api/v1/files/{cid}/download -o downloaded.txt

# 查询分片信息（新功能）
curl http://localhost:8080/api/v1/chunks/{chunk_hash}

# 下载单个分片（新功能）
curl http://localhost:8080/api/v1/chunks/{chunk_hash}/download -o chunk.bin
```

**分片下载功能**:
- 支持断点续传 - 只下载需要的分片
- 并行下载 - 同时下载多个分片提高速度
- 智能缓存 - P2P 下载的分片自动缓存
- 带宽优化 - 按需下载，节省流量

详细文档: [docs/CHUNK_DOWNLOAD_FEATURE_SUMMARY.md](docs/CHUNK_DOWNLOAD_FEATURE_SUMMARY.md)

---

## 📖 CLI 命令参考

### 文件操作

```bash
# 上传文件
p2p file upload <file> [flags]

Flags:
  -t, --tree-type <type>      Merkle 树类型: chameleon | regular (默认: chameleon)
  -d, --description <text>    文件描述
  -o, --output <path>         元数据输出路径 (默认: ./metadata)
  --chunk-size <size>         分块大小，字节 (默认: 262144)
  -p, --progress              显示进度条

# 查看帮助
p2p file --help
p2p file upload --help
```

### 服务管理

```bash
# 启动服务
p2p server [flags]

Flags:
  --config <path>            配置文件路径
  --port <port>              监听端口 (默认: 0 = 随机)
```

### 其他命令

```bash
p2p version                 # 显示版本信息
p2p help                    # 显示帮助
```

---

## ⚙️ 配置

### 配置文件

创建 `config/config.yaml`：

```yaml
network:
  port: 8000
  insecure: false
  bootstrap_peers:
    - /ip4/127.0.0.1/tcp/8001/p2p/QmXXX

storage:
  chunk_path: "files"

performance:
  max_concurrency: 16
  max_retries: 3
  request_timeout: 5

logging:
  level: "info"
  format: "text"
```

### 环境变量

```bash
# 网络配置
export P2P_PORT=8000
export P2P_INSECURE=false

# 存储配置
export P2P_CHUNK_PATH="files"

# 性能配置
export P2P_MAX_CONCURRENCY=16
export P2P_MAX_RETRIES=3

# 日志配置
export P2P_LOG_LEVEL=info
```

完整配置指南：[CONFIGURATION_GUIDE.md](CONFIGURATION_GUIDE.md)

---

## 🌳 Merkle 树类型选择

### Regular Merkle Tree

**使用场景**：
- ✅ 文件备份和归档
- ✅ 一次性文件发布
- ✅ 数据持久化存储
- ✅ 追求最佳性能

**优点**：
- 性能优异
- 实现简单
- 广泛支持

**限制**：
- ❌ 文件内容不可修改

### Chameleon Merkle Tree

**使用场景**：
- ✅ 需要修改已发布的文件
- ✅ 版本控制和可追溯编辑
- ✅ 需要证明编辑权限

**优点**：
- 支持内容修改
- 密钥对控制编辑权限
- 适合动态内容

**注意**：
- ⚠️ 需要妥善保管私钥文件
- ⚠️ 性能略低于 Regular Tree

---

## 🏗️ 项目结构

```
p2pFileTransfer/
├── cmd/
│   ├── p2p/                        # CLI 工具
│   │   ├── main.go                 # CLI 入口
│   │   ├── root.go                 # 根命令
│   │   ├── version.go              # 版本命令
│   │   ├── server.go               # 服务命令
│   │   └── file/                   # 文件操作命令
│   │       ├── cmd.go
│   │       └── upload.go           # 上传实现
│   ├── api/                        # HTTP API 服务 ⭐
│   │   ├── main.go                 # API 入口
│   │   ├── server.go               # 服务器配置
│   │   └── handlers.go             # API 处理函数
│   ├── server/
│   │   └── main.go                 # 服务入口
│   ├── multinode/                  # 多节点测试工具
│   └── test_chunk_download/        # 分片下载测试程序
├── pkg/
│   ├── p2p/                         # P2P 网络核心
│   ├── file/                        # 文件元数据
│   ├── config/                      # 配置管理
│   └── chameleonMerkleTree/        # Merkle 树实现
│       ├── chameleon.go            # Chameleon 哈希
│       ├── chameleonMerkleTree.go # 树结构
│       └── chameleonMerkleTreeImpl.go # 实现
├── config/
│   ├── config.yaml                 # 配置文件
│   └── config.example.yaml         # 配置模板
├── docs/                           # 文档目录 ⭐
│   ├── CHUNK_DOWNLOAD_FEATURE_SUMMARY.md
│   └── MANUAL_TEST_GUIDE.md
├── tests/                          # 测试脚本 ⭐
│   ├── quick-test.ps1              # 快速验证脚本
│   └── test-chunk-download.ps1     # 完整测试脚本
├── test/                            # 测试套件
├── doc/                             # 开发文档
├── bin/                             # 编译输出
│   ├── api.exe                     # HTTP API 服务器
│   ├── p2p.exe                     # CLI 工具
│   └── test_chunk.exe              # 测试程序
├── go.mod
├── go.sum
├── README.md
├── API_DOCUMENTATION.md            # API 完整文档
├── CONFIGURATION_GUIDE.md
└── build.bat                        # 构建脚本
```

---

## 🧪 测试

### 运行测试

```bash
# 运行所有测试
go test ./test -v

# 运行单元测试（快速）
go test ./test -v -short

# 运行多节点测试
go test ./test -v -run TestMultiNode -timeout 10m

# 使用脚本运行
./run_multinode_tests.sh      # Linux/macOS
run_multinode_tests.bat       # Windows
```

### 分片下载功能测试

```powershell
# 快速验证（5分钟）
.\tests\quick-test.ps1

# 完整测试（15分钟）
.\tests\test-chunk-download.ps1

# 手动测试
# 参考 docs/MANUAL_TEST_GUIDE.md
```

### 测试覆盖

- ✅ 40+ 测试用例
- ✅ 7 个多节点集成测试
- ✅ 100% 通过率
- ✅ 并发安全验证 (`-race`)

---

## 📊 技术栈

| 组件 | 技术 | 用途 |
|------|------|------|
| P2P 网络 | libp2p | 去中心化网络 |
| DHT | go-libp2p-kad-dht | Kademlia 路由 |
| 哈希 | SHA256 + Chameleon Hash | 完整性验证 |
| 加密 | elliptic.P256 | 椭圆曲线密码学 |
| CLI | Cobra | 命令行框架 |
| 日志 | logrus | 结构化日志 |
| 配置 | viper | 配置管理 |
| 测试 | testify | 测试框架 |

---

## 📚 文档

### 核心文档
- [API_DOCUMENTATION.md](API_DOCUMENTATION.md) - 完整 API 参考
- [CONFIGURATION_GUIDE.md](CONFIGURATION_GUIDE.md) - 配置指南
- [docs/CHUNK_DOWNLOAD_FEATURE_SUMMARY.md](docs/CHUNK_DOWNLOAD_FEATURE_SUMMARY.md) - 分片下载功能说明
- [docs/MANUAL_TEST_GUIDE.md](docs/MANUAL_TEST_GUIDE.md) - 手动测试指南

### 测试文档
- [test/README.md](test/README.md) - 测试文档
- [test/MULTINODE_TESTS.md](test/MULTINODE_TESTS.md) - 多节点测试

---

## 🤝 贡献指南

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 开发要求

- Go 1.23+
- 遵循 Go 代码规范
- 添加测试用例
- 更新相关文档

---

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

---

## 🔗 相关链接

- [libp2p 文档](https://docs.libp2p.io/)
- [Kademlia DHT 论文](https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf)
- [Go 最佳实践](https://golang.org/doc/effective_go.html)

---

## 🙏 致谢

感谢以下开源项目：

- [libp2p](https://github.com/libp2p/go-libp2p)
- [Cobra](https://github.com/spf13/cobra)
- [logrus](https://github.com/sirupsen/logrus)
- [viper](https://github.com/spf13/viper)
- [testify](https://github.com/stretchr/testify)

---

**当前版本**: v1.1.0 | **最后更新**: 2026-01-21 | **状态**: 生产就绪 ✅

### 更新日志

#### v1.1.0 (2026-01-21)
- ⭐ **新增**: HTTP API 服务
- ⭐ **新增**: 分片下载功能 - 支持按需下载单个分片
- ⭐ **新增**: 分片信息查询 API
- 🔧 **改进**: 自动缓存机制 - P2P 下载的分片自动缓存
- 📝 **新增**: 完整的 API 文档和测试指南
- ✅ **测试**: 新增自动化测试脚本

#### v1.0.0 (2026-01-15)
- 🎉 初始版本发布
- ✅ CLI 命令行工具
- ✅ 双 Merkle 树支持（Regular 和 Chameleon）
- ✅ P2P 网络功能
- ✅ 完整的测试覆盖
