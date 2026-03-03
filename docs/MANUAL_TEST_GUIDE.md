# 分片下载功能测试指南

本指南提供完整的测试步骤，验证新增的根据 hash 下载分片功能。

## 测试环境准备

### 1. 清理环境
```bash
# 清理测试数据
rm -rf test_chunk_data
# 确保端口未被占用
netstat -ano | grep ":8080"
```

### 2. 启动节点 1 (Bootstrap 节点)
```bash
# 创建配置
mkdir -p node1/files node1/metadata
cat > node1/config.yaml << 'EOF'
network:
  port: 0
  insecure: true
  seed: 1
  bootstrap_peers: []

storage:
  chunk_path: "node1/files"

http:
  port: 8080
  metadata_storage_path: "node1/metadata"

performance:
  max_retries: 3
  max_concurrency: 16
  request_timeout: 5
  data_timeout: 30
  dht_timeout: 10

logging:
  level: "info"
  format: "text"

anti_leecher:
  enabled: false
EOF

# 启动节点
./bin/api.exe -config node1/config.yaml -port 8080
```

### 3. 启动节点 2 (连接到节点 1)
```bash
# 在另一个终端窗口
mkdir -p node2/files node2/metadata

# 先获取节点1的peer地址（从节点1日志中复制）
# 格式类似: /ip4/127.0.0.1/tcp/60046/p2p/QmYKkEy7DvM52sp6...

cat > node2/config.yaml << 'EOF'
network:
  port: 0
  insecure: true
  seed: 2
  bootstrap_peers: ["/ip4/127.0.0.1/tcp/60046/p2p/QmYKkEy7DvM52sp6EcXN3CtLYYjckCFLFtGaJaEtxQudmz"]

storage:
  chunk_path: "node2/files"

http:
  port: 8081
  metadata_storage_path: "node2/metadata"

performance:
  max_retries: 3
  max_concurrency: 16
  request_timeout: 5
  data_timeout: 30
  dht_timeout: 10

logging:
  level: "info"
  format: "text"

anti_leecher:
  enabled: false
EOF

# 启动节点2
./bin/api.exe -config node2/config.yaml -port 8081
```

### 4. 启动节点 3 (连接到节点 1)
```bash
# 在第三个终端窗口
mkdir -p node3/files node3/metadata

cat > node3/config.yaml << 'EOF'
network:
  port: 0
  insecure: true
  seed: 3
  bootstrap_peers: ["/ip4/127.0.0.1/tcp/60046/p2p/QmYKkEy7DvM52sp6EcXN3CtLYYjckCFLFtGaJaEtxQudmz"]

storage:
  chunk_path: "node3/files"

http:
  port: 8082
  metadata_storage_path: "node3/metadata"

performance:
  max_retries: 3
  max_concurrency: 16
  request_timeout: 5
  data_timeout: 30
  dht_timeout: 10

logging:
  level: "info"
  format: "text"

anti_leecher:
  enabled: false
EOF

# 启动节点3
./bin/api.exe -config node3/config.yaml -port 8082
```

---

## 阶段 1: 单节点基本功能测试

### 测试 1.1: 健康检查
```bash
curl http://localhost:8080/api/health
```
**预期输出:**
```json
{"success":true,"data":{"status":"ok","service":"p2p-file-transfer-api"}}
```

### 测试 1.2: 获取节点信息
```bash
curl http://localhost:8080/api/v1/node/info
```
**预期输出:** 包含 peerID 和 addresses

### 测试 1.3: 上传测试文件
```bash
# 创建测试文件
echo "Hello P2P World! This is a test file for chunk download." > test.txt

# 上传文件
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@test.txt" \
  -F "tree_type=chameleon" \
  -F "description=Test file"
```
**预期输出:**
```json
{
  "success": true,
  "data": {
    "cid": "...",
    "fileName": "test.txt",
    "treeType": "chameleon",
    "chunkCount": 1,
    "fileSize": 58,
    "message": "..."
  }
}
```

**保存 CID**，例如: `abc123...`

### 测试 1.4: 获取文件信息
```bash
# 替换 {CID} 为上一步获取的 CID
curl http://localhost:8080/api/v1/files/{CID}
```
**预期输出:** 包含 Leaves 数组，每个元素包含 ChunkHash

### 测试 1.5: 查询分片信息
```bash
# 从文件信息中提取第一个 ChunkHash，然后查询
curl http://localhost:8080/api/v1/chunks/{CHUNK_HASH}
```
**预期输出:**
```json
{
  "success": true,
  "data": {
    "hash": "...",
    "local": true,
    "size": 58,
    "p2p_providers": 1,
    "providers": ["..."]
  }
}
```

### 测试 1.6: 下载分片（本地）
```bash
# 下载分片并查看响应头
curl -I http://localhost:8080/api/v1/chunks/{CHUNK_HASH}/download

# 下载分片数据
curl http://localhost:8080/api/v1/chunks/{CHUNK_HASH}/download -o downloaded_chunk.bin

# 验证文件
cat downloaded_chunk.bin
```
**验证点:**
- 响应头包含 `X-Chunk-Source: local`
- 下载的文件内容与原始文件一致

### 测试 1.7: 下载完整文件
```bash
curl http://localhost:8080/api/v1/files/{CID}/download -o full_file.bin

# 验证
diff test.txt full_file.bin
```

---

## 阶段 2: 多节点 P2P 测试

### 测试 2.1: 检查节点连接
```bash
# 查看节点0连接的对等节点
curl http://localhost:8080/api/v1/node/peers
```
**预期输出:** 应该看到至少 2 个对等节点（节点1和节点2）

### 测试 2.2: 从节点2查询分片信息
```bash
# 使用节点2的端口
curl http://localhost:8081/api/v1/chunks/{CHUNK_HASH}
```
**预期输出:**
```json
{
  "success": true,
  "data": {
    "hash": "...",
    "local": false,  // 节点2本地没有此分片
    "p2p_providers": 1,  // 但在P2P网络中找到提供者
    "providers": ["..."]
  }
}
```

### 测试 2.3: 从节点2下载分片（P2P）
```bash
# 从节点2下载分片（节点2会从节点0获取）
curl -I http://localhost:8081/api/v1/chunks/{CHUNK_HASH}/download

curl http://localhost:8081/api/v1/chunks/{CHUNK_HASH}/download -o p2p_chunk.bin
```
**验证点:**
- 响应头包含 `X-Chunk-Source: p2p-downloaded` 或 `p2p`
- 下载的分片与本地下载的分片一致: `md5sum downloaded_chunk.bin p2p_chunk.bin`

### 测试 2.4: 从节点3下载完整文件
```bash
curl http://localhost:8082/api/v1/files/{CID}/download -o p2p_file.bin

# 验证完整性
diff test.txt p2p_file.bin
```

---

## 阶段 3: 缓存机制测试

### 测试 3.1: 验证缓存
```bash
# 检查节点2的本地存储
ls -la node2/files/

# 应该能看到下载的分片已经缓存
```

### 测试 3.2: 再次从节点2下载（应从缓存）
```bash
curl -I http://localhost:8081/api/v1/chunks/{CHUNK_HASH}/download
```
**预期输出:** `X-Chunk-Source: local`（因为已经缓存）

### 测试 3.3: 删除缓存后重新下载
```bash
# 删除节点2的缓存
rm node2/files/{CHUNK_HASH}

# 再次下载，应从P2P获取
curl -I http://localhost:8081/api/v1/chunks/{CHUNK_HASH}/download
```
**预期输出:** `X-Chunk-Source: p2p-downloaded`

---

## 阶段 4: 错误处理测试

### 测试 4.1: 不存在的分片
```bash
curl -i http://localhost:8080/api/v1/chunks/deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef/download
```
**预期输出:** HTTP 404 Not Found

### 测试 4.2: 无效的 hash 格式
```bash
curl -i http://localhost:8080/api/v1/chunks/invalid-hash-format/download
```
**预期输出:** HTTP 400 Bad Request

### 测试 4.3: 查询不存在的分片信息
```bash
curl http://localhost:8080/api/v1/chunks/deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef
```
**预期输出:**
```json
{
  "success": true,
  "data": {
    "hash": "deadbeef...",
    "local": false,
    "p2p_providers": 0
  }
}
```

---

## 测试结果检查清单

### 单节点测试
- [ ] 健康检查通过
- [ ] 文件上传成功
- [ ] 文件信息查询正确
- [ ] 分片信息查询显示 `local: true`
- [ ] 本地分片下载返回 `X-Chunk-Source: local`
- [ ] 完整文件下载成功

### 多节点测试
- [ ] 节点间成功连接
- [ ] 跨节点查询分片信息显示 `p2p_providers > 0`
- [ ] P2P 分片下载返回 `X-Chunk-Source: p2p-*`
- [ ] 本地下载和 P2P 下载的分片内容一致
- [ ] 跨节点完整文件下载成功

### 缓存测试
- [ ] P2P 下载后分片被缓存到本地
- [ ] 后续下载从缓存获取 (`X-Chunk-Source: local`)
- [ ] 删除缓存后重新从 P2P 获取

### 错误处理测试
- [ ] 不存在的分片返回 404
- [ ] 无效 hash 格式返回 400
- [ ] 错误消息清晰准确

---

## 性能测试（可选）

### 上传大文件
```bash
# 创建 10MB 文件
dd if=/dev/urandom of=large_file.bin bs=1M count=10

# 上传
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@large_file.bin" \
  -F "tree_type=chameleon"
```

### 测试分片并发下载
```bash
# 提取所有分片 hash，然后并发下载
for hash in {CHUNK_HASH_LIST}; do
  curl http://localhost:8081/api/v1/chunks/$hash/download -o chunk_$hash.bin &
done
wait
```

---

## 故障排查

### 问题: 节点启动失败
**解决方案:**
1. 检查端口是否被占用: `netstat -ano | grep ":8080"`
2. 检查配置文件路径是否正确（使用双反斜杠或正斜杠）
3. 查看节点日志文件

### 问题: 节点间无法连接
**解决方案:**
1. 确认 bootstrap 地址格式正确
2. 检查防火墙设置
3. 等待 DHT 初始化完成（约 10-30 秒）

### 问题: P2P 下载失败
**解决方案:**
1. 确认节点已连接: `/api/v1/node/peers`
2. 确认分片已 announce: 检查上传日志
3. 等待 DHT 传播完成
4. 检查 chunk 存储路径

---

## 清理测试环境

```bash
# 停止所有节点（Ctrl+C 或 kill 进程）

# 清理测试数据
rm -rf node1 node2 node3 test.txt *.bin test_chunk_data
```

---

## API 接口总结

### 1. 下载分片
```
GET /api/v1/chunks/{hash}/download
```
**响应头:**
- `X-Chunk-Source`: local | p2p-downloaded | p2p

### 2. 查询分片信息
```
GET /api/v1/chunks/{hash}
```
**响应数据:**
```json
{
  "success": true,
  "data": {
    "hash": "分片哈希",
    "local": true/false,
    "size": 分片大小（如果本地存在）,
    "p2p_providers": 提供者数量,
    "providers": ["peerID列表"]
  }
}
```

### 3. 与完整文件下载的对比
- 完整文件: `GET /api/v1/files/{cid}/download` - 下载所有分片并重组
- 单个分片: `GET /api/v1/chunks/{hash}/download` - 仅下载指定分片

---

## 使用场景

1. **断点续传**: 可以根据需要下载特定分片
2. **部分访问**: 只需要文件的部分内容时
3. **并行下载**: 多个分片可以并行下载以提高速度
4. **选择性加载**: 大型数据集按需加载
5. **带宽优化**: 只下载需要的分片，节省带宽
