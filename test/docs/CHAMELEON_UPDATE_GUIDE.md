# 变色龙 Merkle Tree 文件更新功能测试指南

## 功能概述

变色龙 Merkle Tree 允许在保持 CID（文件标识符）不变的情况下修改文件内容。这是通过使用私钥找到哈希碰撞来实现的。

## 测试前准备

### 1. 启动 API 服务器

```bash
# 编译
go build -o bin/api.exe ./cmd/api

# 启动服务器
./bin/api.exe
```

服务器将在 `http://localhost:8080` 上运行。

### 2. 准备测试文件

```bash
# 创建测试文件
echo "Original content for chameleon hash test" > test/files/original.txt
echo "Modified content for chameleon hash test" > test/files/modified.txt
```

---

## 测试场景

### 场景 1：API 完整流程测试

#### 步骤 1：上传原始文件（变色龙模式）

```bash
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@test/files/original.txt" \
  -F "tree_type=chameleon" \
  -F "description=Test file for update"
```

**预期响应**：
```json
{
  "success": true,
  "data": {
    "cid": "abc123...",
    "fileName": "original.txt",
    "treeType": "chameleon",
    "regularRootHash": "def456...",
    "randomNum": "789012...",
    "publicKey": "345678...",
    "chunkCount": 1,
    "fileSize": 44,
    "message": "File uploaded successfully with Chameleon Merkle Tree"
  }
}
```

**重要**：保存返回的以下参数，用于后续更新：
- `cid`
- `regularRootHash`
- `randomNum`
- `publicKey`

#### 步骤 2：保存私钥

上传时会自动保存私钥到 `metadata/<cid>.key`，记录下来：
```bash
cat metadata/<cid>.key
```

#### 步骤 3：更新文件

使用以下命令更新文件（保持 CID 不变）：

```bash
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/modified.txt" \
  -F "cid=<从步骤1获取的CID>" \
  -F "regular_root_hash=<从步骤1获取的regularRootHash>" \
  -F "random_num=<从步骤1获取的randomNum>" \
  -F "public_key=<从步骤1获取的publicKey>" \
  -F "private_key=<从私钥文件读取的私钥>"
```

**预期响应**：
```json
{
  "success": true,
  "data": {
    "cid": "abc123...",  // ⭐ 与原始文件相同
    "fileName": "modified.txt",
    "treeType": "chameleon",
    "regularRootHash": "newhash...",  // ⭐ 已更新
    "randomNum": "newrandom...",       // ⭐ 已更新
    "publicKey": "345678...",          // ⭐ 保持不变
    "chunkCount": 1,
    "fileSize": 43,
    "message": "File updated successfully"
  }
}
```

#### 步骤 4：验证更新

```bash
# 查询文件信息
curl http://localhost:8080/api/v1/files/<CID>

# 下载文件
curl http://localhost:8080/api/v1/files/<CID>/download -o downloaded.txt

# 验证内容
cat downloaded.txt
# 应该输出: Modified content for chameleon hash test
```

---

### 场景 2：使用配置文件中的私钥

#### 配置私钥

编辑 `config/config.yaml`：

```yaml
chameleon:
  private_key: "<从metadata/<cid>.key读取的私钥>"
```

#### 重启服务器并更新

```bash
# 重启服务器
./bin/api.exe

# 更新文件（不需要提供 private_key 参数）
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/modified.txt" \
  -F "cid=<CID>" \
  -F "regular_root_hash=<regularRootHash>" \
  -F "random_num=<randomNum>" \
  -F "public_key=<publicKey>"
```

---

### 场景 3：错误处理测试

#### 测试 3.1：缺少必需参数

```bash
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/modified.txt" \
  -F "cid=<CID>"
  # 缺少 regular_root_hash
```

**预期响应**：
```json
{
  "success": false,
  "error": "regular_root_hash is required"
}
```

#### 测试 3.2：私钥不匹配

```bash
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/modified.txt" \
  -F "cid=<CID>" \
  -F "regular_root_hash=<regularRootHash>" \
  -F "random_num=<randomNum>" \
  -F "public_key=<publicKey>" \
  -F "private_key=0000000000000000000000000000000000000000000000000000000000000000"
```

**预期响应**：
```json
{
  "success": false,
  "error": "Update failed: failed to update merkle tree: ..."
}
```

#### 测试 3.3：错误的 CID 格式

```bash
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/modified.txt" \
  -F "cid=not-a-valid-hex" \
  -F "regular_root_hash=<regularRootHash>" \
  -F "random_num=<randomNum>" \
  -F "public_key=<publicKey>" \
  -F "private_key=<privateKey>"
```

**预期响应**：
```json
{
  "success": false,
  "error": "Update failed: invalid CID format: ..."
}
```

---

### 场景 4：CID 一致性验证

更新文件后，CID 必须保持不变。这是变色龙哈希的核心特性。

```bash
# 上传文件
UPLOAD_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@test/files/original.txt" \
  -F "tree_type=chameleon")

ORIGINAL_CID=$(echo $UPLOAD_RESPONSE | jq -r '.data.cid')

# 更新文件
UPDATE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/modified.txt" \
  -F "cid=$ORIGINAL_CID" \
  -F "regular_root_hash=<...>" \
  -F "random_num=<...>" \
  -F "public_key=<...>" \
  -F "private_key=<...>")

UPDATED_CID=$(echo $UPDATE_RESPONSE | jq -r '.data.cid')

# 验证
if [ "$ORIGINAL_CID" == "$UPDATED_CID" ]; then
  echo "✓ CID consistency test PASSED"
else
  echo "✗ CID consistency test FAILED"
fi
```

---

### 场景 5：多次更新测试

验证文件可以被多次更新，每次都保持相同的 CID。

```bash
# 第一次更新
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/version2.txt" \
  -F "cid=$CID" \
  -F "regular_root_hash=<从上一次响应获取>" \
  -F "random_num=<从上一次响应获取>" \
  -F "public_key=<公钥>" \
  -F "private_key=<私钥>"

# 第二次更新（使用第一次更新返回的新参数）
curl -X POST http://localhost:8080/api/v1/files/update \
  -F "file=@test/files/version3.txt" \
  -F "cid=$CID" \
  -F "regular_root_hash=<从第一次更新获取>" \
  -F "random_num=<从第一次更新获取>" \
  -F "public_key=<公钥>" \
  -F "private_key=<私钥>"

# 验证 CID 始终不变
```

---

## 测试检查清单

### 功能测试

- [ ] 上传变色龙文件成功，返回所有必需参数
- [ ] 更新文件后 CID 保持不变
- [ ] 更新后 regularRootHash 发生变化
- [ ] 更新后 randomNum 发生变化
- [ ] 更新后 publicKey 保持不变
- [ ] 下载的文件内容与更新后的内容一致
- [ ] 元数据正确更新

### 错误处理测试

- [ ] 缺少必需参数时返回明确错误
- [ ] 私钥格式错误时返回错误
- [ ] 私钥不匹配时更新失败
- [ ] CID 不存在时返回错误
- [ ] 文件不存在时返回错误

### 安全性测试

- [ ] 只有持有正确私钥才能更新文件
- [ ] 使用错误私钥无法更新文件
- [ ] 无法将 Regular 模式文件更新为 Chameleon 模式

---

## PowerShell 测试脚本

```powershell
# test/scripts/chameleon_update_test.ps1

# 1. 上传原始文件
$uploadResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/files/upload" `
  -Method POST `
  -ContentType "multipart/form-data" `
  -Form @{
    file = Get-Item -Path "test\files\original.txt"
    tree_type = "chameleon"
    description = "Test file"
  }

Write-Host "Upload Response:"
Write-Host "  CID: $($uploadResponse.data.cid)"
Write-Host "  RegularRootHash: $($uploadResponse.data.regularRootHash)"
Write-Host "  RandomNum: $($uploadResponse.data.randomNum)"
Write-Host "  PublicKey: $($uploadResponse.data.publicKey)"

$cid = $uploadResponse.data.cid
$regularRootHash = $uploadResponse.data.regularRootHash
$randomNum = $uploadResponse.data.randomNum
$publicKey = $uploadResponse.data.publicKey

# 2. 读取私钥
$keyData = Get-Content "metadata\$cid.key" | ConvertFrom-Json
$privateKey = $keyData.privateKey

# 3. 更新文件
$updateResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/files/update" `
  -Method POST `
  -ContentType "multipart/form-data" `
  -Form @{
    file = Get-Item -Path "test\files\modified.txt"
    cid = $cid
    regular_root_hash = $regularRootHash
    random_num = $randomNum
    public_key = $publicKey
    private_key = $privateKey
  }

Write-Host "`nUpdate Response:"
Write-Host "  CID: $($updateResponse.data.cid)"
Write-Host "  Original CID: $cid"

# 4. 验证 CID 一致性
if ($updateResponse.data.cid -eq $cid) {
  Write-Host "`n✓ CID consistency test PASSED"
} else {
  Write-Host "`n✗ CID consistency test FAILED"
  Write-Host "  Expected: $cid"
  Write-Host "  Got: $($updateResponse.data.cid)"
}
```

---

## 故障排查

### 问题 1：编译错误

```bash
# 确保所有依赖都已安装
go mod tidy
go mod verify
```

### 问题 2：API 启动失败

检查端口是否被占用：
```bash
netstat -ano | findstr :8080
```

### 问题 3：更新失败

1. 确认所有参数都是正确的 hex 编码
2. 确认私钥与公钥匹配
3. 检查元数据文件是否存在

---

## 测试结果记录模板

| 测试场景 | 测试日期 | 测试结果 | 备注 |
|---------|---------|---------|------|
| API 完整流程 | | ☐ 通过 ☐ 失败 | |
| 配置文件私钥 | | ☐ 通过 ☐ 失败 | |
| 错误处理 | | ☐ 通过 ☐ 失败 | |
| CID 一致性 | | ☐ 通过 ☐ 失败 | |
| 多次更新 | | ☐ 通过 ☐ 失败 | |

---

## 附录：API 规范

### POST /api/v1/files/update

**请求**：
- Method: POST
- Content-Type: multipart/form-data
- Body:
  - file: 文件（必需）
  - cid: 原文件的 CID（必需）
  - regular_root_hash: 原文件的常规 Merkle 根哈希（必需，hex编码）
  - random_num: 原文件的随机数（必需，hex编码）
  - public_key: 原文件的公钥（必需，hex编码）
  - private_key: 私钥（可选，hex编码，如果不提供则从配置文件读取）

**成功响应**（200 OK）：
```json
{
  "success": true,
  "data": {
    "cid": "string",
    "fileName": "string",
    "treeType": "chameleon",
    "regularRootHash": "string",
    "randomNum": "string",
    "publicKey": "string",
    "chunkCount": 0,
    "fileSize": 0,
    "message": "File updated successfully"
  }
}
```

**错误响应**：
- 400 Bad Request: 缺少必需参数或参数格式错误
- 404 Not Found: 文件元数据不存在
- 500 Internal Server Error: 更新失败（私钥不匹配、哈希碰撞失败等）
