# P2P 文件传输系统 - 配置管理指南

## 概述

项目支持三种配置方式，推荐使用**配置文件**方式进行配置：

1. **配置文件**（推荐）- YAML 格式
2. **环境变量** - 覆盖配置文件中的值
3. **代码配置** - 直接在代码中设置（仅用于开发）

## 配置优先级

配置的优先级从高到低为：
1. **环境变量**（最高优先级）
2. **配置文件**
3. **代码默认值**（最低优先级）

---

## 快速开始

### 1. 使用配置文件启动

```bash
# 复制示例配置文件
cp config/config.example.yaml config/config.yaml

# 根据需要修改配置
vim config/config.yaml

# 启动服务
go run cmd/server/main.go

# 或编译后启动
go build -o p2p-server ./cmd/server/main.go
./p2p-server
```

### 2. 使用环境变量覆盖

```bash
# 使用环境变量覆盖配置
P2P_PORT=8000 \
P2P_LOG_LEVEL=debug \
P2P_MAX_CONCURRENCY=32 \
go run cmd/server/main.go
```

### 3. 指定配置文件路径

```bash
# 使用自定义配置文件
go run cmd/server/main.go -config /path/to/custom-config.yaml
```

---

## 配置文件详解

### 配置文件位置

配置文件搜索顺序（按优先级）：
1. 命令行 `-config` 参数指定的路径
2. `./config.yaml`
3. `./config/config.yaml`
4. `/etc/p2p-file-transfer/config.yaml`

### 配置文件结构

```yaml
# 网络配置 / Network Configuration
network:
  # 监听端口 (0 = 自动分配随机端口)
  port: 0

  # 是否使用不安全的连接（仅用于开发测试）
  insecure: false

  # 随机种子（用于确定性密钥生成）
  seed: 0

  # Bootstrap 节点地址（用于初始网络连接）
  # 格式: /ip4/IP/tcp/PORT/p2p/PEER_ID
  bootstrap_peers:
    - /ip4/127.0.0.1/tcp/8001/p2p/QmPeerID123

  # 协议前缀
  protocol_prefix: "/p2p-file-transfer"

  # 启用自动刷新
  auto_refresh: true

  # DHT 命名空间
  namespace: "p2p-file-transfer"

# 存储配置 / Storage Configuration
storage:
  # Chunk 存储路径
  chunk_path: "files"

  # Merkle 树块大小（字节）
  block_size: 262144  # 256KB

  # 缓冲区数量
  buffer_number: 16

# 性能配置 / Performance Configuration
performance:
  # 最大重试次数
  max_retries: 3

  # 最大并发下载数
  max_concurrency: 16

  # 请求超时（秒）
  request_timeout: 5

  # 数据传输超时（秒）
  data_timeout: 30

  # DHT 操作超时（秒）
  dht_timeout: 10

# 日志配置 / Logging Configuration
logging:
  # 日志级别 (debug, info, warn, error)
  level: "info"

  # 日志格式 (json, text)
  format: "text"

# 反吸血虫配置 / Anti-Leecher Configuration
anti_leecher:
  # 是否启用反吸血虫机制
  enabled: true

  # 最小成功率阈值（0.0-1.0）
  min_success_rate: 0.5

  # 黑名单前的最小请求数
  min_requests: 10
```

---

## 配置项说明

### 网络配置 (network)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `port` | int | 0 | 监听端口，0 表示自动分配随机端口 |
| `insecure` | bool | false | 是否使用不安全连接（仅开发环境使用） |
| `seed` | int64 | 0 | 随机种子，用于生成确定性密钥 |
| `bootstrap_peers` | []string | [] | Bootstrap 节点地址列表 |
| `protocol_prefix` | string | "/p2p-file-transfer" | 协议前缀 |
| `auto_refresh` | bool | true | 是否启用 DHT 自动刷新 |
| `namespace` | string | "p2p-file-transfer" | DHT 命名空间 |

### 存储配置 (storage)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `chunk_path` | string | "files" | Chunk 文件存储路径 |
| `block_size` | uint | 262144 | Merkle 树块大小（256KB） |
| `buffer_number` | uint | 16 | 缓冲区数量 |

### 性能配置 (performance)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `max_retries` | int | 3 | 最大重试次数 |
| `max_concurrency` | int | 16 | 最大并发下载数 |
| `request_timeout` | int | 5 | 请求超时时间（秒） |
| `data_timeout` | int | 30 | 数据传输超时时间（秒） |
| `dht_timeout` | int | 10 | DHT 操作超时时间（秒） |

### 日志配置 (logging)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `level` | string | "info" | 日志级别（debug, info, warn, error） |
| `format` | string | "text" | 日志格式（json, text） |

### 反吸血虫配置 (anti_leecher)

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `enabled` | bool | true | 是否启用反吸血虫机制 |
| `min_success_rate` | float64 | 0.5 | 最小成功率阈值（0.0-1.0） |
| `min_requests` | int | 10 | 黑名单前的最小请求数 |

---

## 环境变量列表

所有配置项都可以通过环境变量覆盖，使用 `P2P_` 前缀：

| 环境变量 | 对应配置项 | 示例值 |
|---------|-----------|--------|
| `P2P_PORT` | network.port | `8000` |
| `P2P_INSECURE` | network.insecure | `true` |
| `P2P_SEED` | network.seed | `12345` |
| `P2P_BOOTSTRAP_PEERS` | network.bootstrap_peers | `/ip4/127.0.0.1/tcp/8001/p2p/QmXXX,...` |
| `P2P_PROTOCOL_PREFIX` | network.protocol_prefix | `/p2p-file-transfer` |
| `P2P_NAMESPACE` | network.namespace | `p2p-file-transfer` |
| `P2P_CHUNK_PATH` | storage.chunk_path | `/var/lib/p2p/chunks` |
| `P2P_BLOCK_SIZE` | storage.block_size | `262144` |
| `P2P_BUFFER_NUMBER` | storage.buffer_number | `16` |
| `P2P_MAX_RETRIES` | performance.max_retries | `3` |
| `P2P_MAX_CONCURRENCY` | performance.max_concurrency | `16` |
| `P2P_REQUEST_TIMEOUT` | performance.request_timeout | `5` |
| `P2P_DATA_TIMEOUT` | performance.data_timeout | `30` |
| `P2P_DHT_TIMEOUT` | performance.dht_timeout | `10` |
| `P2P_LOG_LEVEL` | logging.level | `debug` |
| `P2P_LOG_FORMAT` | logging.format | `json` |
| `P2P_ANTI_LEECHER_ENABLED` | anti_leecher.enabled | `true` |
| `P2P_MIN_SUCCESS_RATE` | anti_leecher.min_success_rate | `0.5` |
| `P2P_MIN_REQUESTS` | anti_leecher.min_requests | `10` |

---

## 在代码中使用配置

### 基本用法

```go
package main

import (
    "context"
    "github.com/sirupsen/logrus"
    "p2pFileTransfer/pkg/config"
    "p2pFileTransfer/pkg/p2p"
)

func main() {
    // 加载配置
    cfg, err := config.Load("config/config.yaml")
    if err != nil {
        logrus.Fatalf("Failed to load config: %v", err)
    }

    // 确保目录存在
    if err := cfg.EnsureDirectories(); err != nil {
        logrus.Fatalf("Failed to create directories: %v", err)
    }

    // 转换为 P2PConfig
    p2pConfig := cfg.ToP2PConfig()

    // 创建服务
    ctx := context.Background()
    service, err := p2p.NewP2PService(ctx, *p2pConfig)
    if err != nil {
        logrus.Fatalf("Failed to create service: %v", err)
    }
    defer service.Shutdown()

    // 使用服务...
}
```

### 查找配置文件

```go
// 让系统自动查找配置文件
configPath := config.GetConfigPath("")
cfg, err := config.Load(configPath)
```

---

## 不同环境的配置示例

### 开发环境 (config.dev.yaml)

```yaml
network:
  insecure: true        # 使用不安全连接便于调试
  port: 8000            # 固定端口

logging:
  level: "debug"        # 详细日志
  format: "text"        # 文本格式便于阅读

performance:
  max_concurrency: 8    # 降低并发
```

### 测试环境 (config.test.yaml)

```yaml
network:
  insecure: false
  port: 0               # 随机端口

logging:
  level: "info"
  format: "json"        # JSON 格式便于解析

performance:
  max_concurrency: 16
  max_retries: 5
```

### 生产环境 (config.prod.yaml)

```yaml
network:
  insecure: false       # 必须使用安全连接
  port: 8000

logging:
  level: "info"         # 生产环境使用 info 级别
  format: "json"        # JSON 格式便于日志收集

performance:
  max_concurrency: 32   # 高并发
  max_retries: 3
  request_timeout: 10
  data_timeout: 60

anti_leecher:
  enabled: true
  min_success_rate: 0.6  # 更严格的要求
  min_requests: 20
```

### 低带宽环境 (config.low-bandwidth.yaml)

```yaml
performance:
  max_concurrency: 8    # 降低并发
  max_retries: 5        # 增加重试
  request_timeout: 10
  data_timeout: 60       # 更长的超时
```

### 高性能环境 (config.high-performance.yaml)

```yaml
performance:
  max_concurrency: 64   # 高并发
  max_retries: 2        # 快速失败
  request_timeout: 3
  data_timeout: 15

storage:
  buffer_number: 32     # 增加缓冲
```

---

## 配置验证

系统会自动验证配置，包括：

- **端口范围**: 0-65535
- **块大小**: 1KB - 4MB
- **并发数**: 1 - 1024
- **超时时间**: 1 - 7200 秒
- **成功率阈值**: 0.0 - 1.0
- **日志级别**: debug, info, warn, error
- **日志格式**: json, text

如果配置无效，服务会在启动时报错并退出。

---

## 故障排查

### 配置文件未找到

```
Config file not found, using defaults
```

**解决方案**：
- 确认配置文件路径正确
- 使用 `-config` 参数指定完整路径
- 或复制 `config.example.yaml` 创建配置文件

### 配置验证失败

```
config validation failed: invalid port: 70000 (must be 0-65535)
```

**解决方案**：
- 检查配置文件中的值是否在有效范围内
- 参考上方的配置项说明

### 权限错误

```
failed to create chunk directory: permission denied
```

**解决方案**：
- 检查 `chunk_path` 目录的写权限
- 使用绝对路径或确保相对路径目录存在

---

## 命令行参数

```bash
# 显示帮助
go run cmd/server/main.go -help

# 显示版本
go run cmd/server/main.go -version

# 使用自定义配置文件
go run cmd/server/main.go -config /path/to/config.yaml
```

---

## 最佳实践

1. **使用配置文件** - 推荐使用 YAML 配置文件而非环境变量
2. **环境区分** - 为不同环境创建不同的配置文件
3. **敏感信息** - 不要在配置文件中存储敏感信息，使用环境变量
4. **默认值** - 保留配置文件中的默认值注释，便于查阅
5. **版本控制** - 将 `config.example.yaml` 加入版本控制，`config.yaml` 加入 `.gitignore`
6. **验证配置** - 启动前先验证配置是否正确
7. **日志级别** - 生产环境使用 `info`，开发环境使用 `debug`

---

## 相关文件

- `config/config.yaml` - 主配置文件
- `config/config.example.yaml` - 配置文件示例
- `.env.example` - 环境变量示例
- `pkg/config/loader.go` - 配置加载代码
- `cmd/server/main.go` - 主程序入口

---

## 更多帮助

如有问题，请查看：
- [README.md](README.md) - 项目总体介绍
- [test/README.md](test/README.md) - 测试文档
- [第二次改进总结.md](doc/第二次改进总结.md) - 代码改进记录
