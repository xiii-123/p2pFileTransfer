# HTTP API 使用指南

## 简介

P2P文件传输系统提供了HTTP API接口，可以通过HTTP请求进行文件上传、下载、节点管理和DHT操作。

## 启动服务器

```bash
# 使用默认配置（端口8080）
.\bin\p2p-api.exe

# 指定端口
.\bin\p2p-api.exe -port 9000

# 使用自定义配置文件
.\bin\p2p-api.exe -config config/config.yaml

# 查看帮助
.\bin\p2p-api.exe -help

# 查看版本
.\bin\p2p-api.exe -version
```

## API端点

### 1. 健康检查

**请求**
```http
GET /api/health
```

**响应**
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "service": "p2p-file-transfer-api"
  }
}
```

### 2. 文件上传

**请求**
```http
POST /api/v1/files/upload
Content-Type: multipart/form-data

file: <文件内容>
tree_type: chameleon  # 可选: chameleon(可编辑) 或 regular(标准)
description: <文件描述>  # 可选
```

**示例 (curl)**
```bash
# 使用Chameleon Merkle Tree上传（可编辑）
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@test.txt" \
  -F "tree_type=chameleon" \
  -F "description=测试文件"

# 使用Regular Merkle Tree上传（标准，更快）
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@test.txt" \
  -F "tree_type=regular"
```

**响应**
```json
{
  "success": true,
  "data": {
    "cid": "a1b2c3d4e5f6...",
    "fileName": "test.txt",
    "treeType": "chameleon",
    "chunkCount": 10,
    "message": "File uploaded successfully with Chameleon Merkle Tree"
  }
}
```

### 3. 查询文件信息

**请求**
```http
GET /api/v1/files/{cid}
```

**示例**
```bash
curl http://localhost:8080/api/v1/files/a1b2c3d4e5f6...
```

**响应**
```json
{
  "success": true,
  "data": {
    "rootHash": "a1b2c3d4e5f6...",
    "randomNum": "fedcba987654...",
    "publicKey": "1234567890abcdef...",
    "description": "测试文件",
    "fileSize": 2048576,
    "fileName": "test.txt",
    "encryption": "none",
    "treeType": "chameleon",
    "leaves": [
      {
        "chunkSize": 262144,
        "chunkHash": "aaa111..."
      }
    ]
  }
}
```

### 4. 下载文件

**请求**
```http
GET /api/v1/files/{cid}/download
```

**示例**
```bash
# 下载并保存到本地
curl -O http://localhost:8080/api/v1/files/a1b2c3d4e5f6.../download

# 下载并指定文件名
curl -o downloaded.txt http://localhost:8080/api/v1/files/a1b2c3d4e5f6.../download
```

**响应**
- 文件内容（二进制流）
- Content-Type: application/octet-stream
- Content-Disposition: attachment; filename="{原始文件名}"

### 5. 获取节点信息

**请求**
```http
GET /api/v1/node/info
```

**响应**
```json
{
  "success": true,
  "data": {
    "peerID": "QmXYZ...",
    "addresses": [
      "/ip4/127.0.0.1/tcp/12345/p2p/QmXYZ..."
    ],
    "protocols": ["/p2p-file-transfer/1.0.0"]
  }
}
```

### 6. 获取连接的对等节点列表

**请求**
```http
GET /api/v1/node/peers
```

**响应**
```json
{
  "success": true,
  "data": {
    "count": 2,
    "peers": [
      {
        "peerID": "QmABC...",
        "address": "/ip4/192.168.1.100/tcp/12345/p2p/QmABC...",
        "direction": "outbound"
      }
    ]
  }
}
```

### 7. DHT查找提供者

**请求**
```http
GET /api/v1/dht/providers/{key}
```

**示例**
```bash
curl http://localhost:8080/api/v1/dht/providers/a1b2c3d4e5f6...
```

**响应**
```json
{
  "success": true,
  "data": {
    "key": "a1b2c3d4e5f6...",
    "count": 3,
    "providers": [
      "QmProvider1...",
      "QmProvider2...",
      "QmProvider3..."
    ]
  }
}
```

### 8. DHT公告

**请求**
```http
POST /api/v1/dht/announce
Content-Type: application/json

{
  "key": "a1b2c3d4e5f6..."
}
```

**响应**
```json
{
  "success": true,
  "data": {
    "key": "a1b2c3d4e5f6...",
    "message": "Announced successfully"
  }
}
```

### 9. DHT获取值

**请求**
```http
GET /api/v1/dht/value/{key}
```

**响应**
```json
{
  "success": true,
  "data": {
    "key": "mykey",
    "value": "myvalue"
  }
}
```

### 10. DHT设置值

**请求**
```http
POST /api/v1/dht/value
Content-Type: application/json

{
  "key": "mykey",
  "value": "myvalue"
}
```

**响应**
```json
{
  "success": true,
  "data": {
    "key": "mykey",
    "message": "Value stored successfully"
  }
}
```

## 错误响应

所有错误响应的格式：

```json
{
  "success": false,
  "error": "错误信息"
}
```

常见HTTP状态码：
- 200: 成功
- 400: 请求参数错误
- 404: 资源不存在
- 405: 方法不允许
- 500: 服务器内部错误
- 501: 功能未实现

## Merkle树类型选择

### Chameleon Merkle Tree（可编辑）
- **优点**: 文件上传后可以修改内容（需要私钥）
- **缺点**: 上传速度较慢，需要管理私钥
- **适用**: 需要未来修改的文件

### Regular Merkle Tree（标准）
- **优点**: 上传速度快，无需密钥管理
- **缺点**: 不可修改
- **适用**: 一次性上传，不需要修改的文件

## 配置

HTTP服务器支持以下配置：

```yaml
http:
  port: 8080              # HTTP端口
  metadata_path: metadata  # 元数据存储路径
```

可以通过以下方式配置：
1. 配置文件（config.yaml）
2. 环境变量（P2P_HTTP_PORT, P2P_METADATA_PATH）
3. 命令行参数（-port）

## CORS支持

API默认支持CORS，允许跨域请求。响应头：
```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

## 注意事项

1. **文件大小**: 默认支持最大100GB的文件上传
2. **超时设置**: 上传/下载超时时间为30分钟
3. **元数据存储**: 元数据保存在`metadata/`目录，以CID作为文件名
4. **并发限制**: 每个节点最多5个并发流

## 示例工作流程

### 完整的文件上传和下载流程

```bash
# 1. 启动HTTP API服务器
.\bin\p2p-api.exe -port 8080

# 2. 上传文件
UPLOAD_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@myfile.txt" \
  -F "tree_type=chameleon" \
  -F "description=重要文件")

# 提取CID
CID=$(echo $UPLOAD_RESPONSE | jq -r '.data.cid')

echo "文件CID: $CID"

# 3. 查询文件信息
curl -s http://localhost:8080/api/v1/files/$CID | jq '.'

# 4. 下载文件
curl -O http://localhost:8080/api/v1/files/$CID/download

# 5. 查找文件提供者
curl -s http://localhost:8080/api/v1/dht/providers/$CID | jq '.'
```

### DHT键值存储示例

```bash
# 存储值
curl -X POST http://localhost:8080/api/v1/dht/value \
  -H "Content-Type: application/json" \
  -d '{"key": "mykey", "value": "myvalue"}'

# 获取值
curl http://localhost:8080/api/v1/dht/value/mykey
```

## 技术实现

- **Web框架**: Go标准库 net/http
- **路由**: http.ServeMux
- **JSON**: encoding/json
- **P2P后端**: libp2p
- **Merkle树**: Chameleon Merkle Tree / Regular Merkle Tree

## 性能特性

- **流式下载**: 使用内存缓冲区，大文件下载仅需32KB内存
- **并发处理**: 支持多个并发HTTP请求
- **异步上传**: 文件分块异步上传到DHT
- **CORS支持**: 支持浏览器跨域请求
