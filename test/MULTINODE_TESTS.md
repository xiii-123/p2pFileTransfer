# 多节点集成测试指南

## 概述

`multinode_test.go` 包含需要多个P2P节点互联才能完成的集成测试。这些测试验证了：
- DHT 数据存储和检索
- Chunk 公告和查询
- 跨节点文件下载
- 并发文件传输
- 节点发现机制
- DHT 数据持久化

---

## 测试列表

### 1. TestMultiNodeDHTPutAndGet
测试在多个节点组成的DHT网络中存储和检索数据。

**测试内容**:
- 在节点0上存储键值对
- 从节点4检索数据
- 验证数据正确性

**预期时间**: ~10秒

### 2. TestMultiNodeChunkAnnounceAndLookup
测试Chunk公告和提供者查询功能。

**测试内容**:
- 节点0创建并公告chunk
- 节点4查询chunk的提供者
- 验证节点0在提供者列表中

**预期时间**: ~15秒

### 3. TestMultiNodeChunkDownload
测试跨节点Chunk下载。

**测试内容**:
- 节点0存储chunk并公告
- 节点4检查chunk存在性
- 节点4下载chunk数据
- 验证数据完整性

**预期时间**: ~15秒

### 4. TestMultiNodeFileDownload
测试完整文件的多节点下载。

**测试内容**:
- 节点0创建包含多个chunk的文件
- 节点0公告所有chunk
- 节点4下载完整文件
- 验证进度回调

**预期时间**: ~20秒

### 5. TestMultiNodeConcurrentDownloads
测试多个文件并发下载。

**测试内容**:
- 节点0创建3个测试文件
- 不同节点并发下载不同文件
- 验证所有文件下载成功

**预期时间**: ~30秒

### 6. TestMultiNodePeerDiscovery
测试节点发现机制。

**测试内容**:
- 创建7个节点的网络
- 检查每个节点的连接状态
- 验证全网状连接

**预期时间**: ~20秒

### 7. TestMultiNodeDHTPersistence
测试DHT数据在多个节点间的持久化。

**测试内容**:
- 节点0存储数据
- 等待数据传播
- 从多个节点检索数据
- 统计检索成功率

**预期时间**: ~15秒

---

## 运行测试

### 方法1: 使用提供的脚本 (推荐)

**Windows**:
```batch
run_multinode_tests.bat
```

**Linux/macOS**:
```bash
chmod +x run_multinode_tests.sh
./run_multinode_tests.sh
```

脚本会自动:
1. 运行所有多节点测试
2. 生成覆盖率报告
3. 显示测试总结

### 方法2: 手动运行

**运行所有多节点测试**:
```bash
cd test
go test -v -run TestMultiNode -timeout 10m
```

**运行特定测试**:
```bash
# 只运行Chunk下载测试
go test -v -run TestMultiNodeChunkDownload -timeout 2m

# 只运行文件下载测试
go test -v -run TestMultiNodeFileDownload -timeout 2m
```

**运行带覆盖率的测试**:
```bash
cd test
go test -v -coverprofile=coverage_multinode.out -run TestMultiNode -timeout 10m
go tool cover -html=coverage_multinode.out
```

### 方法3: 短模式测试 (快速验证)

在短模式下，多节点测试会被跳过:
```bash
go test -v -short -run TestMultiNode
```

输出:
```
=== RUN   TestMultiNodeDHTPutAndGet
    multinode_test.go:XX: Skipping multi-node test in short mode
--- SKIP: TestMultiNodeDHTPutAndGet (0.00s)
```

---

## 测试要求

### 系统要求
- **Go**: 1.20 或更高版本
- **内存**: 至少 2GB 可用内存
- **CPU**: 多核处理器推荐
- **网络**: 本地回环接口正常工作

### 时间要求
- **快速测试** (short模式): < 1秒 (跳过多节点测试)
- **完整测试**: ~2分钟 (包含所有多节点测试)

### 端口要求
测试使用随机端口 (Port = 0)，无需手动配置端口。

---

## 测试架构

### 网络拓扑

```
节点0 ─── 节点1
 │ \    / │
 │  \  /  │
 │   \/   │
 │   /\   │
 │  /  \  │
 │ /    \ │
节点4 ─── 节点3 ─── 节点2
```

所有测试都创建**全网状网络**，确保:
- 每个节点都直接连接到所有其他节点
- DHT 路由表充分填充
- 数据可以在任意节点间传播

### 测试流程

1. **创建节点** (5个节点)
   - 使用随机端口
   - 每个节点有独立的存储目录
   - 等待2秒让节点完全启动

2. **建立连接**
   - 使用 `libp2p` Host.Connect()
   - 连接所有节点对 (C(5,2) = 10个连接)
   - 验证连接状态

3. **DHT稳定**
   - 等待3秒让DHT路由表稳定
   - 确保provider公告传播

4. **执行测试**
   - 运行具体的测试逻辑
   - 验证结果正确性

5. **清理资源**
   - 关闭所有节点连接
   - 取消context
   - 清理临时文件

---

## 故障排查

### 问题1: 测试超时

**症状**: `context deadline exceeded`

**解决方案**:
```bash
# 增加超时时间
go test -v -run TestMultiNode -timeout 20m
```

### 问题2: 节点无法连接

**症状**: `failed to connect to peer`

**原因**:
- 防火墙阻止本地连接
- 系统资源不足

**解决方案**:
- 检查防火墙设置
- 关闭其他占用资源的程序
- 减少节点数量 (修改 `setupMultiNodeNetwork(t, 5)` 中的数字)

### 问题3: DHT查询失败

**症状**: `failed to find any peer in table`

**原因**: DHT未充分初始化

**解决方案**:
- 增加稳定等待时间 (修改 `time.Sleep(3 * time.Second)`)
- 确保节点完全启动后再连接

### 问题4: 竞态条件

**症状**: 测试有时通过，有时失败

**解决方案**:
```bash
# 运行带竞态检测的测试
go test -race -v -run TestMultiNode -timeout 10m
```

---

## 性能基准

### 预期性能指标

| 测试 | 节点数 | 预期时间 | 内存占用 |
|------|--------|----------|----------|
| DHT Put/Get | 5 | ~10s | ~50MB |
| Chunk Announce | 5 | ~15s | ~60MB |
| Chunk Download | 5 | ~15s | ~70MB |
| File Download | 5 | ~20s | ~80MB |
| Concurrent Downloads | 5 | ~30s | ~100MB |
| Peer Discovery | 7 | ~20s | ~80MB |
| DHT Persistence | 5 | ~15s | ~60MB |

### 优化建议

**减少测试时间**:
```go
// 减少节点数量
nodes, cleanup := setupMultiNodeNetwork(t, 3)  // 从5改为3

// 减少等待时间
time.Sleep(1 * time.Second)  // 从3秒改为1秒
```

**减少内存占用**:
```go
// 减少并发下载数量
numFiles := 1  // 从3改为1
```

---

## 添加新的多节点测试

### 测试模板

```go
func TestMultiNodeYourFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping multi-node test in short mode")
    }

    // 创建多节点网络
    nodes, cleanup := setupMultiNodeNetwork(t, 5)
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 你的测试逻辑
    // ...

    // 验证结果
    assert.NoError(t, err)
    assert.Equal(t, expected, actual)
}
```

### 最佳实践

1. **添加跳过检查**: 始终包含 `if testing.Short()` 检查
2. **使用 defer cleanup**: 确保资源正确释放
3. **设置合理超时**: 根据测试复杂度设置超时时间
4. **记录日志**: 使用 `t.Logf()` 记录重要步骤
5. **验证资源清理**: 确保没有goroutine泄漏

---

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: Multi-Node Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  schedule:
    # 每天UTC 0:00运行
    - cron: '0 0 * * *'

jobs:
  multinode-tests:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install dependencies
        run: go mod download

      - name: Run multi-node tests
        run: |
          cd test
          go test -v -run TestMultiNode -timeout 15m

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./test/coverage_multinode.out
        if: always()
```

---

## 相关文档

- **test/README.md** - 通用测试文档
- **第二次改进总结.md** - 代码改进总结
- **测试套件完成报告.md** - 测试套件概览

---

## 更新日志

### 2025-01-14
- 创建多节点测试文件
- 添加7个集成测试
- 创建运行脚本 (Windows/Linux)
- 编写测试文档

---

## 贡献

欢迎提交新的多节点测试或改进现有测试！

1. Fork 项目
2. 创建特性分支 (`git checkout -b test/multinode-feature`)
3. 提交更改 (`git commit -am 'Add new multinode test'`)
4. 推送到分支 (`git push origin test/multinode-feature`)
5. 创建 Pull Request
