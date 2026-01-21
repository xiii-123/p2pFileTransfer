# P2P File Transfer HTTP API 文档

## 目录

- [概述](#概述)
- [快速开始](#快速开始)
- [API基础信息](#api基础信息)
- [通用说明](#通用说明)
- [API接口](#api接口)
  - [健康检查](#1-健康检查)
  - [文件操作](#2-文件操作)
  - [分片操作](#3-分片操作) ⭐
  - [节点管理](#4-节点管理)
  - [DHT操作](#5-dht操作)
- [数据模型](#数据模型)
- [错误处理](#错误处理)
- [配置说明](#配置说明)

---

## 概述

P2P File Transfer HTTP API 提供了一套完整的RESTful接口，用于通过HTTP协议与P2P文件传输网络进行交互。该API支持文件上传、下载、查询以及P2P网络管理等功能。

### 主要特性

- 支持大文件上传（最大100GB）
- 支持两种Merkle Tree类型：Chameleon和Regular
- 文件分块存储和传输
- DHT（分布式哈希表）内容路由
- 完整的节点管理功能
- 跨域支持（CORS）

---

## 快速开始

### 启动服务器

```bash
# 使用默认配置
./bin/p2p-api

# 指定端口
./bin/p2p-api -port 9000

# 使用配置文件
./bin/p2p-api -config config/config.yaml

# 查看帮助
./bin/p2p-api -help

# 查看版本
./bin/p2p-api -version
```

### 快速示例

```bash
# 健康检查
curl http://localhost:8080/api/health

# 上传文件（Chameleon Merkle Tree）
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@/path/to/file.txt" \
  -F "tree_type=chameleon" \
  -F "description=My first file"

# 查询文件信息
curl http://localhost:8080/api/v1/files/{cid}

# 下载文件
curl http://localhost:8080/api/v1/files/{cid}/download -o downloaded.txt
```

---

## API基础信息

### Base URL

```
http://localhost:8080
```

默认端口为 `8080`，可通过配置文件或命令行参数修改。

### 内容类型

所有请求和响应使用 `application/json` 格式（文件上传除外，使用 `multipart/form-data`）。

### 跨域支持

API支持CORS，允许来自任何源的跨域请求。

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
```

---

## 通用说明

### 统一响应格式

所有API响应遵循统一格式：

#### 成功响应

```json
{
  "success": true,
  "data": {
    // 响应数据
  }
}
```

#### 错误响应

```json
{
  "success": false,
  "error": "错误描述信息"
}
```

### HTTP状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 405 | 方法不允许 |
| 500 | 服务器内部错误 |
| 501 | 功能未实现 |

---

## API接口

### 1. 健康检查

#### 1.1 健康检查

检查API服务器是否正常运行。

**请求**

```
GET /api/health
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "status": "ok",
    "service": "p2p-file-transfer-api"
  }
}
```

---

### 2. 文件操作

#### 2.1 上传文件

上传文件到P2P网络，支持两种Merkle Tree类型。

**请求**

```
POST /api/v1/files/upload
Content-Type: multipart/form-data
```

**参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | File | 是 | 要上传的文件（最大100GB） |
| tree_type | String | 否 | Merkle Tree类型：`chameleon`（默认）或 `regular` |
| description | String | 否 | 文件描述信息 |

**请求示例（cURL）**

```bash
# 使用Chameleon Merkle Tree（默认）
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@example.txt" \
  -F "description=示例文件"

# 使用Regular Merkle Tree
curl -X POST http://localhost:8080/api/v1/files/upload \
  -F "file=@example.txt" \
  -F "tree_type=regular" \
  -F "description=Regular Merkle Tree文件"
```

**请求示例（Go）**

```go
package main

import (
    "bytes"
    "mime/multipart"
    "os"
    "net/http"
)

func main() {
    file, _ := os.Open("example.txt")
    defer file.Close()

    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)

    // 添加文件
    part, _ := writer.CreateFormFile("file", "example.txt")
    io.Copy(part, file)

    // 添加其他字段
    writer.WriteField("tree_type", "chameleon")
    writer.WriteField("description", "示例文件")
    writer.Close()

    req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/files/upload", body)
    req.Header.Set("Content-Type", writer.FormDataContentType())

    client := &http.Client{}
    client.Do(req)
}
```

**响应示例（Chameleon Merkle Tree）**

```json
{
  "success": true,
  "data": {
    "cid": "a1b2c3d4e5f6...",
    "fileName": "example.txt",
    "treeType": "chameleon",
    "chunkCount": 42,
    "fileSize": 10737418,
    "message": "File uploaded successfully with Chameleon Merkle Tree"
  }
}
```

**响应示例（Regular Merkle Tree）**

```json
{
  "success": true,
  "data": {
    "cid": "f6e5d4c3b2a1...",
    "fileName": "example.txt",
    "treeType": "regular",
    "chunkCount": 42,
    "fileSize": 10737418,
    "message": "File uploaded successfully with Regular Merkle Tree"
  }
}
```

**错误响应**

```json
{
  "success": false,
  "error": "Invalid tree_type 'invalid'. Must be 'chameleon' or 'regular'"
}
```

**处理流程**

1. 接收上传的文件
2. 创建临时文件并流式写入（避免内存溢出）
3. 根据选择的树类型构建Merkle Tree：
   - **Chameleon**: 使用Chameleon哈希，生成密钥对
   - **Regular**: 使用标准SHA256哈希
4. 将文件分块（默认256KB/块）
5. 计算每个分块的哈希值
6. 保存分块到本地存储
7. 将每个分块的哈希公告到DHT
8. 保存文件元数据
9. 返回CID（内容标识符）

#### 2.2 查询文件信息

根据CID查询文件的元数据信息。

**请求**

```
GET /api/v1/files/{cid}
```

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| cid | String | 文件的内容标识符（十六进制编码） |

**请求示例**

```bash
curl http://localhost:8080/api/v1/files/a1b2c3d4e5f6...
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "rootHash": "a1b2c3d4e5f6...",
    "randomNum": "1234567890abcdef...",
    "publicKey": "fedcba0987654321...",
    "description": "示例文件",
    "fileName": "example.txt",
    "fileSize": 10737418,
    "encryption": "none",
    "treeType": "chameleon",
    "leaves": [
      {
        "chunkSize": 262144,
        "chunkHash": "chunk1hash..."
      },
      {
        "chunkSize": 262144,
        "chunkHash": "chunk2hash..."
      }
    ]
  }
}
```

**错误响应**

```json
{
  "success": false,
  "error": "File not found: failed to read metadata file..."
}
```

#### 2.3 下载文件

根据CID下载完整的文件。

**请求**

```
GET /api/v1/files/{cid}/download
```

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| cid | String | 文件的内容标识符（十六进制编码） |

**请求示例**

```bash
# 直接下载到文件
curl http://localhost:8080/api/v1/files/a1b2c3d4e5f6.../download -o downloaded.txt

# 在浏览器中下载
# 访问: http://localhost:8080/api/v1/files/a1b2c3d4e5f6.../download
```

**响应**

- **Content-Type**: `application/octet-stream`
- **Content-Disposition**: `attachment; filename="{fileName}"`

**响应体**

文件的二进制数据。

**错误响应**

```json
{
  "success": false,
  "error": "Download failed: ..."
}
```

**下载逻辑**

1. 首先尝试从本地分块文件重组
2. 如果本地分块不可用，从P2P网络下载
3. 按顺序组装所有分块
4. 返回完整文件

---

### 3. 分片操作 ⭐

分片操作允许你按需下载单个文件分片，支持断点续传、并行下载等高级功能。

#### 3.1 查询分片信息

查询指定hash的分片在本地和P2P网络中的可用性。

**请求**

```
GET /api/v1/chunks/{hash}
```

**路径参数**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| hash | string | 是 | 分片的哈希值（hex编码） |

**请求示例**

```bash
curl http://localhost:8080/api/v1/chunks/d7c2690e86d1075297043b53011358fbf2b7465a9e3ea58109fd5a0ee28c8403
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "hash": "d7c2690e86d1075297043b53011358fbf2b7465a9e3ea58109fd5a0ee28c8403",
    "local": true,
    "size": 262144,
    "p2p_providers": 3,
    "providers": [
      "12D3KooWSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
      "QmXZj9h5V3g8K2bL4c7N6p1R3s8T9w0QmYyQSo1c1Ym7or"
    ]
  }
}
```

**字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| hash | string | 分片的哈希值 |
| local | boolean | 是否在本地存储中存在 |
| size | number | 分片大小（字节），仅当local=true时有值 |
| p2p_providers | number | P2P网络中提供者数量 |
| providers | array[string] | 提供者的Peer ID列表 |

**使用场景**
- 检查分片是否可用
- 查找P2P网络中的提供者
- 决定从哪里下载分片

#### 3.2 下载分片

下载指定hash的分片数据。

**请求**

```
GET /api/v1/chunks/{hash}/download
```

**路径参数**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| hash | string | 是 | 分片的哈希值（hex编码） |

**响应头**

| 头部 | 说明 |
|------|------|
| Content-Type | application/octet-stream |
| Content-Disposition | attachment; filename="{hash}.bin" |
| X-Chunk-Source | 数据来源标识 |
| &nbsp;&nbsp;&nbsp;• `local` | 从本地存储读取 |
| &nbsp;&nbsp;&nbsp;• `p2p-downloaded` | 从P2P网络下载并已缓存 |
| &nbsp;&nbsp;&nbsp;• `p2p` | 从P2P网络下载（缓存失败） |

**请求示例**

```bash
# 下载分片
curl http://localhost:8080/api/v1/chunks/d7c2690e86d1075297043b53011358fbf2b7465a9e3ea58109fd5a0ee28c8403/download -o chunk.bin

# 查看数据来源
curl -I http://localhost:8080/api/v1/chunks/d7c2690e86d1075297043b53011358fbf2b7465a9e3ea58109fd5a0ee28c8403/download
```

**响应体**

分片的二进制数据。

**下载逻辑**

1. 优先从本地存储读取分片
2. 本地不存在时，通过DHT查找P2P网络中的提供者
3. 逐个尝试从提供者下载
4. 下载成功后自动缓存到本地存储
5. 返回分片数据

**错误响应**

| HTTP状态码 | 说明 |
|-----------|------|
| 400 | 无效的hash格式 |
| 404 | 分片不存在（本地和P2P网络都找不到） |
| 500 | P2P下载失败 |

**使用场景**

1. **断点续传** - 根据需要下载特定分片
   ```bash
   # 获取文件信息，找出缺失的分片
   curl http://localhost:8080/api/v1/files/{cid}

   # 只下载缺失的分片
   curl http://localhost:8080/api/v1/chunks/{missing_chunk_hash}/download -o chunk.bin
   ```

2. **并行下载** - 同时下载多个分片提高速度
   ```bash
   for hash in chunk1_hash chunk2_hash chunk3_hash; do
     curl http://localhost:8080/api/v1/chunks/$hash/download -o chunk_$hash.bin &
   done
   wait
   ```

3. **部分访问** - 只需要文件的部分内容
   ```bash
   # 只下载第N个分片（如视频的第5个分片）
   curl http://localhost:8080/api/v1/chunks/{chunk5_hash}/download
   ```

4. **带宽优化** - 只下载需要的分片，节省流量
   ```bash
   # 只下载需要的分片，而非整个文件
   curl http://localhost:8080/api/v1/chunks/{only_needed_chunk}/download
   ```

**注意事项**

- 分片大小默认为 256KB，可通过配置文件修改
- P2P下载的分片会自动缓存到本地存储
- 后续下载相同分片将优先从本地缓存读取
- 响应头 `X-Chunk-Source` 可以用于调试和监控

---

### 4. 节点管理

#### 4.1 获取节点信息

获取当前节点的信息，包括Peer ID和监听地址。

**请求**

```
GET /api/v1/node/info
```

**请求示例**

```bash
curl http://localhost:8080/api/v1/node/info
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "peerID": "QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
    "addresses": [
      "/ip4/127.0.0.1/tcp/0/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
      "/ip4/192.168.1.100/tcp/0/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N"
    ],
    "protocols": [
      "/p2p-file-transfer/1.0.0"
    ]
  }
}
```

**字段说明**

| 字段 | 说明 |
|------|------|
| peerID | 节点的唯一标识符 |
| addresses | 节点的监听地址列表（multiaddr格式） |
| protocols | 节点支持的协议列表 |

#### 4.2 获取对等节点列表

获取当前已连接的所有对等节点信息。

**请求**

```
GET /api/v1/node/peers
```

**请求示例**

```bash
curl http://localhost:8080/api/v1/node/peers
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "count": 2,
    "peers": [
      {
        "peerID": "QmXZj9h5V3g8K2bL4c7N6p1R3s8T9w0QmYyQSo1c1Ym7or",
        "address": "/ip4/192.168.1.101/tcp/12345/p2p/QmXZj9h5V3g8K2bL4c7N6p1R3s8T9w0QmYyQSo1c1Ym7or",
        "direction": "outbound"
      },
      {
        "peerID": "QmN2p4R5s6T7u8V9w0X1y2Z3a4B5c6D7e8F9g0H1i2J3",
        "address": "/ip4/192.168.1.102/tcp/23456/p2p/QmN2p4R5s6T7u8V9w0X1y2Z3a4B5c6D7e8F9g0H1i2J3",
        "direction": "inbound"
      }
    ]
  }
}
```

**字段说明**

| 字段 | 说明 |
|------|------|
| count | 已连接的对等节点数量 |
| peers | 对等节点列表 |
| peerID | 对等节点的ID |
| address | 连接地址 |
| direction | 连接方向：`inbound`（入站）或 `outbound`（出站） |

#### 4.3 连接到对等节点

连接到指定的对等节点。

**请求**

```
POST /api/v1/node/connect
Content-Type: application/json
```

**请求体**

```json
{
  "address": "/ip4/192.168.1.100/tcp/12345/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N"
}
```

**请求示例**

```bash
curl -X POST http://localhost:8080/api/v1/node/connect \
  -H "Content-Type: application/json" \
  -d '{"address": "/ip4/192.168.1.100/tcp/12345/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N"}'
```

**响应**

```json
{
  "success": false,
  "error": "Connection feature not implemented"
}
```

**注意**

此功能当前版本未实现，返回501状态码。

---

### 4. DHT操作

#### 4.1 查找提供者

在DHT中查找指定内容的提供者节点。

**请求**

```
GET /api/v1/dht/providers/{key}
```

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| key | String | 要查找的内容键（通常是chunk哈希的十六进制编码） |

**请求示例**

```bash
curl http://localhost:8080/api/v1/dht/providers/a1b2c3d4e5f6...
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "key": "a1b2c3d4e5f6...",
    "count": 3,
    "providers": [
      "QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
      "QmXZj9h5V3g8K2bL4c7N6p1R3s8T9w0QmYyQSo1c1Ym7or",
      "QmN2p4R5s6T7u8V9w0X1y2Z3a4B5c6D7e8F9g0H1i2J3"
    ]
  }
}
```

**字段说明**

| 字段 | 说明 |
|------|------|
| key | 查询的键 |
| count | 找到的提供者数量 |
| providers | 提供者Peer ID列表 |

#### 4.2 DHT公告

向DHT网络公告自己提供指定内容。

**请求**

```
POST /api/v1/dht/announce
Content-Type: application/json
```

**请求体**

```json
{
  "key": "a1b2c3d4e5f6..."
}
```

**请求示例**

```bash
curl -X POST http://localhost:8080/api/v1/dht/announce \
  -H "Content-Type: application/json" \
  -d '{"key": "a1b2c3d4e5f6..."}'
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "key": "a1b2c3d4e5f6...",
    "message": "Announced successfully"
  }
}
```

**说明**

此操作告诉DHT网络，当前节点可以提供指定key对应的内容。通常在上传文件后自动调用。

#### 4.3 获取DHT值

从DHT中获取指定key的值。

**请求**

```
GET /api/v1/dht/value/{key}
```

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| key | String | 要查询的键 |

**请求示例**

```bash
curl http://localhost:8080/api/v1/dht/value/mykey
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "key": "mykey",
    "value": "SGVsbG8gV29ybGQ="
  }
}
```

**说明**

value字段返回的是Base64编码的字节数组。

#### 4.4 设置DHT值

向DHT中存入键值对。

**请求**

```
POST /api/v1/dht/value
Content-Type: application/json
```

**请求体**

```json
{
  "key": "mykey",
  "value": "Hello World"
}
```

**请求示例**

```bash
curl -X POST http://localhost:8080/api/v1/dht/value \
  -H "Content-Type: application/json" \
  -d '{"key": "mykey", "value": "Hello World"}'
```

**响应示例**

```json
{
  "success": true,
  "data": {
    "key": "mykey",
    "message": "Value stored successfully"
  }
}
```

**说明**

- value会被转换为字节数组存储
- DHT中的值会根据协议进行复制和传播
- 值可能不会永久保存，取决于DHT的实现

---

## 数据模型

### MetaData（文件元数据）

```json
{
  "rootHash": "[]byte",        // Merkle树根哈希
  "randomNum": "[]byte",       // 随机数（Chameleon树专用）
  "publicKey": "[]byte",       // 公钥（Chameleon树专用）
  "description": "string",     // 文件描述
  "fileName": "string",        // 文件名
  "fileSize": "uint64",        // 文件大小（字节）
  "encryption": "string",      // 加密方式（当前仅支持"none"）
  "treeType": "string",        // Merkle树类型："chameleon" | "regular"
  "leaves": [                  // 分块信息列表
    {
      "chunkSize": "int",      // 分块大小
      "chunkHash": "[]byte"    // 分块哈希
    }
  ]
}
```

### ChunkData（分块数据）

```json
{
  "chunkSize": "int",          // 分块大小（字节）
  "chunkHash": "[]byte"        // 分块内容的哈希值
}
```

### APIResponse（统一响应）

```json
{
  "success": "boolean",        // 请求是否成功
  "data": "object",           // 响应数据（成功时）
  "error": "string"           // 错误信息（失败时）
}
```

---

## 错误处理

### 错误响应格式

所有错误都遵循统一格式：

```json
{
  "success": false,
  "error": "详细的错误信息"
}
```

### 常见错误

| HTTP状态码 | 错误示例 | 说明 |
|------------|----------|------|
| 400 | `Method not allowed` | HTTP方法不正确 |
| 400 | `Failed to parse form: ...` | 表单解析失败 |
| 400 | `Failed to get file: ...` | 文件获取失败 |
| 400 | `Invalid tree_type 'xxx'` | 树类型参数无效 |
| 400 | `CID is required` | 缺少必需的CID参数 |
| 400 | `Invalid request body: ...` | 请求体格式错误 |
| 400 | `Address is required` | 缺少必需的地址参数 |
| 400 | `Key and value are required` | 缺少必需的键值参数 |
| 404 | `File not found: ...` | 文件元数据不存在 |
| 500 | `Upload failed: ...` | 文件上传处理失败 |
| 500 | `Download failed: ...` | 文件下载失败 |
| 500 | `Failed to find providers: ...` | DHT提供者查找失败 |
| 500 | `Announce failed: ...` | DHT公告失败 |
| 500 | `Failed to get value: ...` | DHT值获取失败 |
| 500 | `Failed to put value: ...` | DHT值设置失败 |
| 501 | `Connection feature not implemented` | 功能未实现 |

### 错误处理建议

1. **客户端应检查`success`字段**
   ```javascript
   const response = await fetch(url);
   const data = await response.json();
   if (!data.success) {
     console.error(data.error);
     // 处理错误
   }
   ```

2. **根据HTTP状态码进行不同处理**
   ```javascript
   if (response.status === 404) {
     // 资源不存在
   } else if (response.status === 500) {
     // 服务器错误，可能需要重试
   }
   ```

3. **大文件上传建议使用进度监控**
   ```javascript
   const xhr = new XMLHttpRequest();
   xhr.upload.addEventListener('progress', (e) => {
     const percent = (e.loaded / e.total) * 100;
     console.log(`Upload progress: ${percent}%`);
   });
   ```

---

## 配置说明

### 配置文件（config.yaml）

```yaml
network:
  port: 0                          # P2P监听端口（0表示随机）
  insecure: false                  # 是否使用不安全连接
  seed: 0                          # 随机数种子
  bootstrap_peers:                 # 启动节点列表
    - "/ip4/1.2.3.4/tcp/12345/p2p/..."
  protocol_prefix: "/p2p-file-transfer"
  auto_refresh: true               # 自动刷新
  namespace: "p2p-file-transfer"

storage:
  chunk_path: "files"              # 分块存储路径
  block_size: 262144               # 分块大小（256KB）
  buffer_number: 16                # 缓冲区数量

performance:
  max_retries: 3                   # 最大重试次数
  max_concurrency: 16              # 最大并发数
  request_timeout: 5               # 请求超时（秒）
  data_timeout: 30                 # 数据传输超时（秒）
  dht_timeout: 10                  # DHT操作超时（秒）

logging:
  level: "info"                    # 日志级别
  format: "text"                   # 日志格式

anti_leecher:
  enabled: true                    # 是否启用反吸血虫
  min_success_rate: 0.5            # 最低成功率
  min_requests: 10                 # 最小请求数
  blacklist_timeout: 3600          # 黑名单超时（秒）

http:
  port: 8080                       # HTTP API端口
  metadata_path: "metadata"        # 元数据存储路径
```

### 环境变量

所有配置项都可以通过环境变量覆盖，使用 `P2P_` 前缀：

```bash
export P2P_HTTP_PORT=9000
export P2P_METADATA_PATH=/var/lib/p2p/metadata
export P2P_LOG_LEVEL=debug
```

### 命令行参数

```bash
./bin/p2p-api [options]

选项:
  -config string     # 配置文件路径
  -port int          # HTTP服务器端口（默认8080）
  -version           # 显示版本信息
  -help              # 显示帮助信息
```

---

## 附录

### Merkle Tree类型说明

#### Chameleon Merkle Tree

- **特点**：支持哈希碰撞，可以修改内容而不改变根哈希
- **适用场景**：需要动态修改文件内容的场景
- **安全性**：需要私钥才能修改，公钥用于验证

#### Regular Merkle Tree

- **特点**：标准Merkle树，内容不可变
- **适用场景**：不需要修改的静态文件
- **安全性**：一旦生成，任何修改都会改变根哈希

### 分块大小说明

- **默认大小**：256KB（262,144字节）
- **可配置范围**：1KB - 4MB
- **影响**：
  - 较小的分块：更好的并行传输，但管理开销大
  - 较大的分块：管理开销小，但可能降低并行效率

### CID（内容标识符）

- **格式**：十六进制编码的根哈希
- **用途**：唯一标识文件内容
- **特点**：内容相同，CID必然相同

### 性能建议

1. **大文件上传**
   - 使用稳定的网络连接
   - 考虑增加超时时间
   - 使用合适的分块大小

2. **批量操作**
   - 控制并发数量（建议不超过16）
   - 实现重试机制
   - 监控网络状态

3. **DHT操作**
   - DHT查找可能需要时间（默认超时10秒）
   - 值不会永久保存
   - 提供者信息会动态变化

---

## 版本历史

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0.0 | 2024-01-15 | 初始版本 |

---

## 联系方式

如有问题或建议，请通过以下方式联系：

- 项目地址：[GitHub]
- 问题反馈：[Issues]
