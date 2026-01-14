# P2P File Transfer 测试文档

## 测试概述

本项目包含完整的测试套件，涵盖单元测试、集成测试、并发测试和性能测试。

---

## 测试文件结构

```
test/
├── integration_test.go  # 集成测试（主要测试文件）
├── utils_test.go        # 单元测试（工具函数和组件测试）
└── README.md           # 本文档
```

---

## 测试类别

### 1. 集成测试 (integration_test.go)

完整的端到端测试，验证整个系统的功能：

#### P2P 服务测试
- `TestP2PServiceCreation` - 测试服务创建和配置
- `TestServiceShutdown` - 测试优雅关闭

#### DHT 功能测试
- `TestDHTPutAndGet` - 测试 DHT 存储和检索
- `TestAnnounce` - 测试 Chunk 公告
- `TestLookUp` - 测试提供者查询

#### Chunk 传输测试
- `TestChunkExistenceCheck` - 测试 Chunk 存在性检查
- `TestChunkDownload` - 测试 Chunk 下载

#### 文件下载测试
- `TestFileDownloadOrdered` - 测试顺序文件下载
- `TestFileDownloadWithProgress` - 测试带进度报告的下载
- `TestFileDownloadStreaming` - 测试流式下载（大文件）

#### 并发测试
- `TestConcurrentDownloads` - 测试多文件并发下载

#### 组件测试
- `TestPeerSelector` - 测试节点选择器
- `TestConnManager` - 测试连接管理器
- `TestErrorHandling` - 测试错误分类机制

### 2. 单元测试 (utils_test.go)

针对特定函数和组件的详细测试：

#### 配置和工具函数测试
- `TestRetryConfig` - 测试重试配置
- `TestBytesEqual` - 测试字节比较
- `TestRemovePeer` - 测试节点移除
- `TestConfigurationDefaults` - 测试默认配置

#### 连接管理器测试
- `TestConnManagerEdgeCases` - 边界情况测试
- `TestConnManagerConcurrency` - 并发安全测试
- `TestConnManagerCleanup` - 清理功能测试

#### 节点选择器测试
- `TestRandomPeerSelector` - 随机选择器测试
- `TestRoundRobinPeerSelector` - 轮询选择器测试

#### 其他测试
- `TestProgressCallback` - 进度回调测试
- `TestContextCancellation` - 上下文取消测试
- `TestLargeFileHandling` - 大文件处理测试

### 3. 性能测试 (Benchmark)

- `BenchmarkChunkDownload` - Chunk 下载性能
- `BenchmarkFileDownload` - 文件下载性能

---

## 运行测试

### 运行所有测试

```bash
# 在项目根目录
go test ./test/...

# 或在 test 目录
go test -v
```

### 运行特定测试

```bash
# 运行集成测试
go test -v ./test -run TestP2P

# 运行文件下载测试
go test -v ./test -run TestFileDownload

# 运行并发测试
go test -v ./test -run TestConcurrent

# 运行连接管理器测试
go test -v ./test -run TestConnManager
```

### 运行性能测试

```bash
# 运行所有 benchmark
go test -bench=. -benchmem ./test

# 运行特定 benchmark
go test -bench=BenchmarkChunkDownload -benchmem ./test
go test -bench=BenchmarkFileDownload -benchmem ./test
```

### 测试覆盖率

```bash
# 生成覆盖率报告
go test -cover ./test

# 生成详细的覆盖率报告（HTML）
go test -coverprofile=coverage.out ./test
go tool cover -html=coverage.out
```

---

## 测试依赖

测试使用以下库：

- **github.com/stretchr/testify** - 断言和测试辅助工具
- **github.com/libp2p/go-libp2p** - P2P 网络

安装依赖：
```bash
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/require
```

---

## 测试最佳实践

### 1. 测试隔离

每个测试使用独立的临时目录：

```go
tmpDir := t.TempDir()  // 自动清理
```

### 2. 资源清理

使用 `defer` 和 `t.Cleanup` 确保资源释放：

```go
nodes, cleanup := setupTestNodes(t, 2)
defer cleanup()  // 确保节点被关闭
```

### 3. 并发安全

测试并发场景时使用通道收集结果：

```go
errChan := make(chan error, numWorkers)
// 启动 goroutines
for i := 0; i < numWorkers; i++ {
    go func() {
        errChan <- doWork()
    }()
}
// 收集结果
for i := 0; i < numWorkers; i++ {
    assert.NoError(t, <-errChan)
}
```

### 4. 超时控制

为测试设置合理的超时：

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

---

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install dependencies
        run: go mod download

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./test

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

---

## 性能基准

### 预期性能指标

| 操作 | 预期性能 | 备注 |
|------|----------|------|
| Chunk 下载 (100KB) | < 100ms | 局域网环境 |
| 文件下载 (1MB) | < 1s | 16 并发 |
| 流式下载 (100MB) | < 30s | 内存占用 < 100MB |
| 并发下载 (5个文件) | < 5s | 每个 100KB |

### 运行性能分析

```bash
# CPU 性能分析
go test -cpuprofile=cpu.prof -bench=. ./test
go tool pprof cpu.prof

# 内存性能分析
go test -memprofile=mem.prof -bench=. ./test
go tool pprof mem.prof
```

---

## 故障排查

### 测试超时

如果测试超时：
1. 检查网络连接
2. 增加超时时间：
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
   ```

### 端口冲突

如果遇到端口占用：
- 测试使用 `Port = 0` 让系统自动分配端口
- 如果仍然冲突，检查是否有其他进程在运行

### DHT 连接失败

如果 DHT 节点无法连接：
1. 确保防火墙允许 P2P 连接
2. 检查 Bootstrap 节点配置
3. 增加节点启动等待时间

---

## 添加新测试

### 测试模板

```go
func TestNewFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:    "test case 1",
            input:   testInput,
            want:    expectedOutput,
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 测试逻辑
            result, err := FunctionToTest(tt.input)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.want, result)
            }
        })
    }
}
```

---

## 测试检查清单

在提交代码前，确保：

- [ ] 所有测试通过 (`go test ./...`)
- [ ] 竞态检测通过 (`go test -race ./...`)
- [ ] 代码覆盖率 > 80%
- [ ] 性能测试无明显退化
- [ ] 集成测试在干净环境中通过

---

## 参考资源

- [Go Testing 官方文档](https://golang.org/pkg/testing/)
- [Testify 断言库](https://github.com/stretchr/testify)
- [libp2p 文档](https://docs.libp2p.io/)
- [Go Performance 最佳实践](https://go.dev/doc/diagnostics)

---

## 更新日志

### 2025-01-14
- 创建完整的测试套件
- 添加集成测试（8 个测试）
- 添加单元测试（15 个测试）
- 添加性能测试（2 个 benchmark）
- 创建测试文档

---

## 贡献

欢迎提交测试改进和 Bug 修复！

1. Fork 项目
2. 创建测试分支 (`git checkout -b test/new-feature`)
3. 提交更改 (`git commit -am 'Add new test'`)
4. 推送到分支 (`git push origin test/new-feature`)
5. 创建 Pull Request
