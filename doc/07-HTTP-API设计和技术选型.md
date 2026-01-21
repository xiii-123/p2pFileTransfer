# P2P 文件传输系统 - HTTP API 设计与技术选型

## 目录

1. [技术选型](#1-技术选型)
2. [API 设计原则](#2-api-设计原则)
3. [核心 API 接口](#3-核心-api-接口)
4. [数据模型](#4-数据模型)
5. [认证与授权](#5-认证与授权)
6. [错误处理](#6-错误处理)
7. [性能优化](#7-性能优化)
8. [部署架构](#8-部署架构)
9. [实施计划](#9-实施计划)

---

## 1. 技术选型

### 1.1 Web 框架选择

#### 推荐：Gin + gRPC

| 方案 | 优势 | 劣势 | 推荐指数 |
|------|------|------|----------|
| **Gin** | 高性能、易用、生态好 | 仅支持 Go | ⭐⭐⭐⭐⭐ |
| **Echo** | 高性能、中间件丰富 | 学习曲线稍陡 | ⭐⭐⭐⭐ |
| **Fiber** | 极高性能、基于 FastIO | 社区相对较小 | ⭐⭐⭐⭐ |
| **gRPC-Gateway** | 支持流式传输、Protobuf | 复杂度高 | ⭐⭐⭐⭐⭐ |
| **go-resty** | 简单轻量 | 性能一般 | ⭐⭐⭐ |

#### 最终选择：**Gin**

**理由**：
- ✅ 高性能（基于 httprouter）
- ✅ API 设计优雅
- ✅ 中间件生态完善
- ✅ JSON 验证、绑定方便
- ✅ 与现有 Go 项目集成容易
- ✅ 社区活跃，文档完善

**依赖**：
```go
import (
    "github.com/gin-gonic/gin"
    "github.com/gin-contrib/cors"
    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/cookie"
)
```



---

## 2. API 设计原则

### 2.1 RESTful 设计

**URL 结构**：
```
/api/v1/
├── /files              # 文件管理
├── /chunks            # Chunk 管理
├── /users             # 用户管理
├── /nodes             # 节点管理
├── /status            # 系统状态
└── /health            # 健康检查
```

**命名规范**：
- 使用名词复数：`/files` 而不是 `/file`
- 使用小写：`/files/{id}` 而不是 `/Files/{ID}`
- 使用连字符：`/chunk-hashes` 而不是 `/chunkHashes`

### 2.2 HTTP 方法映射

| 操作 | HTTP 方法 | URL | 说明 |
|------|-----------|-----|------|
| 列出文件 | GET | /api/v1/files | 获取文件列表 |
| 上传文件 | POST | /api/v1/files | 上传新文件 |
| 获取文件信息 | GET | /api/v1/files/{cid} | 获取文件元数据 |
| 下载文件 | GET | /api/v1/files/{cid}/download | 下载文件内容 |
| 删除文件 | DELETE | /api/v1/files/{cid} | 删除文件记录 |
| 更新文件 | PUT | /api/v1/files/{cid} | 更新文件元数据 |

### 2.3 版本控制

**URL 版本**：`/api/v1/`

**Header 版本**（可选）：
```http
GET /files HTTP/1.1
Accept: application/vnd.p2p.v1+json
```

### 2.4 分页、排序、过滤

**分页**：
```http
GET /api/v1/files?page=1&page_size=20
```

**排序**：
```http
GET /api/v1/files?sort=created_at&order=desc
```

**过滤**：
```http
GET /api/v1/files?tree_type=chameleon&uploader_id=1
```

---

## 3. 核心 API 接口

### 3.1 文件管理 API

#### 3.1.1 上传文件

**请求**：
```http
POST /api/v1/files/upload
Content-Type: multipart/form-data

# 表单字段
- file: 文件数据
- tree_type: Merkle 树类型 (chameleon | regular)
- description: 文件描述
- chunk_size: 分块大小（可选，默认 262144）
```

**代码实现**：
```go
type UploadHandler struct {
    p2pService *p2p.P2PService
    storage    StorageBackend
    db         *gorm.DB
}

// UploadFile 上传文件
// @Summary 上传文件到 P2P 网络
// @Description 上传文件并返回文件的 CID
// @Tags files
// @Accept multipart/form-data
// @Param file formData file true "文件"
// @Param tree_type formData string false "Merkle 树类型" Enums(chameleon, regular)
// @Param description formData string false "文件描述"
// @Success 200 {object} UploadResponse
// @Router /api/v1/files/upload [post]
func (h *UploadHandler) UploadFile(c *gin.Context) {
    // 1. 解析表单
    fileHeader, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
        return
    }

    treeType := c.DefaultPostForm("tree_type", "regular")
    description := c.PostForm("description")
    chunkSize := c.DefaultPostForm("chunk_size", "262144")

    // 2. 打开文件
    file, err := fileHeader.Open()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "文件打开失败"})
        return
    }
    defer file.Close()

    // 3. 计算哈希
    chunks, err := p2p.CalculateChunkHashes(file, chunkSize)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "哈希计算失败"})
        return
    }

    // 4. 构建 Merkle 树
    var cid string
    var metadata *file.MetaData

    ctx := context.Background()

    if treeType == "chameleon" {
        pubKey, privKey, _ := chameleonMerkleTree.GenerateKeyPair()
        cmt, _ := chameleonMerkleTree.NewChameleonMerkleTreeFromHashes(chunks, pubKey)
        cid = fmt.Sprintf("%x", cmt.GetChameleonHash())
        metadata = &file.MetaData{
            RootHash:    cmt.GetChameleonHash(),
            PublicKey:   serializePublicKey(pubKey),
            RandomNum:   serializeRandomNum(cmt.GetRandomNumber()),
            TreeType:     "chameleon",
            // ...
        }
    } else {
        cid = buildRegularMerkleRoot(chunks)
        metadata = &file.MetaData{
            RootHash: []byte(cid),
            TreeType:  "regular",
            // ...
        }
    }

    // 5. 存储 Chunks
    for i, chunk := range chunks {
        path := fmt.Sprintf("files/%x", chunk.Hash)
        if err := os.WriteFile(path, chunk.Data, 0644); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Chunk 存储失败"})
            return
        }
    }

    // 6. 公告到 DHT
    for _, chunk := range chunks {
        hash := fmt.Sprintf("%x", chunk.Hash)
        if err := h.p2pService.Announce(ctx, hash); err != nil {
            logrus.Errorf("Announce chunk %s failed: %v", hash, err)
        }
    }

    // 7. 保存元数据到数据库
    if err := h.saveMetadata(metadata, fileHeader.Filename); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "元数据保存失败"})
        return
    }

    // 8. 返回结果
    c.JSON(http.StatusOK, gin.H{
        "cid":         cid,
        "file_name":   fileHeader.Filename,
        "file_size":   fileHeader.Size,
        "tree_type":   treeType,
        "chunk_count": len(chunks),
        "created_at":  time.Now(),
    })
}
```

**响应**：
```json
{
  "cid": "a1b2c3d4e5f6...",
  "file_name": "document.pdf",
  "file_size": 1048576,
  "tree_type": "regular",
  "chunk_count": 4,
  "created_at": "2026-01-15T10:30:00Z"
}
```

#### 3.1.2 下载文件

**请求**：
```http
GET /api/v1/files/{cid}/download
```

**代码实现**：
```go
func (h *FileHandler) DownloadFile(c *gin.Context) {
    cid := c.Param("cid")

    // 1. 从数据库获取元数据
    metadata, err := h.getMetadata(cid)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
        return
    }

    // 2. 下载文件
    destPath := fmt.Sprintf("/tmp/%s", metadata.FileName)
    ctx := context.Background()

    if err := h.p2pService.DownloadFile(ctx, metadata, destPath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "下载失败"})
        return
    }

    // 3. 返回文件
    c.FileAttachment(destPath, metadata.FileName)
}
```

#### 3.1.3 获取文件信息

**请求**：
```http
GET /api/v1/files/{cid}
```

**响应**：
```json
{
  "cid": "a1b2c3d4e5f6...",
  "file_name": "document.pdf",
  "file_size": 1048576,
  "tree_type": "chameleon",
  "description": "重要文档",
  "chunk_count": 4,
  "uploader": "user1",
  "created_at": "2026-01-15T10:30:00Z",
  "chunks": [
    {
      "index": 0,
      "hash": "abc123...",
      "size": 262144
    },
    {
      "index": 1,
      "hash": "def456...",
      "size": 262144
    }
  ]
}
```

#### 3.1.4 列出文件

**请求**：
```http
GET /api/v1/files?page=1&page_size=20&sort=created_at&order=desc
```

**响应**：
```json
{
  "files": [
    {
      "cid": "a1b2c3...",
      "file_name": "doc1.pdf",
      "file_size": 1048576,
      "tree_type": "regular",
      "created_at": "2026-01-15T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

#### 3.1.5 更新文件（仅 Chameleon）

**请求**：
```http
PUT /api/v1/files/{cid}
Content-Type: application/json

{
  "description": "更新后的描述",
  "private_key_file": "/path/to/private.key"
}
```

### 3.2 Chunk 管理 API

#### 3.2.1 上传 Chunk

**请求**：
```http
POST /api/v1/chunks
Content-Type: application/json

{
  "chunk_hash": "abc123...",
  "chunk_data": "base64encodeddata",
  "chunk_index": 0,
  "chunk_size": 262144
}
```

**代码实现**：
```go
func (h *ChunkHandler) UploadChunk(c *gin.Context) {
    var req ChunkUploadRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 1. 解码 Base64 数据
    data, err := base64.StdEncoding.DecodeString(req.ChunkData)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "数据格式错误"})
        return
    }

    // 2. 验证哈希
    hash := sha256.Sum256(data)
    hashStr := fmt.Sprintf("%x", hash)
    if hashStr != req.ChunkHash {
        c.JSON(http.StatusBadRequest, gin.H{"error": "哈希不匹配"})
        return
    }

    // 3. 存储 Chunk
    storagePath := fmt.Sprintf("files/%s", req.ChunkHash)
    if err := os.WriteFile(storagePath, data, 0644); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "存储失败"})
        return
    }

    // 4. 公告到 DHT
    ctx := context.Background()
    if err := h.p2pService.Announce(ctx, req.ChunkHash); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "DHT 公告失败"})
        return
    }

    // 5. 记录到数据库
    chunk := Chunk{
        Hash:     req.ChunkHash,
        Index:    req.ChunkIndex,
        Size:     req.ChunkSize,
        FilePath: storagePath,
    }
    if err := h.db.Create(&chunk).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库保存失败"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "chunk_hash": req.ChunkHash,
        "status":      "uploaded",
    })
}
```

#### 3.2.2 下载 Chunk

**请求**：
```http
GET /api/v1/chunks/{hash}
```

**响应**：
```json
{
  "chunk_hash": "abc123...",
  "chunk_data": "base64encodeddata",
  "chunk_size": 262144,
  "status": "available"
}
```

#### 3.2.3 验证 Chunk

**请求**：
```http
POST /api/v1/chunks/{hash}/verify
```

**代码实现**：
```go
func (h *ChunkHandler) VerifyChunk(c *gin.Context) {
    chunkHash := c.Param("hash")

    // 1. 从存储读取 Chunk
    storagePath := fmt.Sprintf("files/%s", chunkHash)
    data, err := os.ReadFile(storagePath)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Chunk 不存在"})
        return
    }

    // 2. 计算哈希
    hash := sha256.Sum256(data)
    hashStr := fmt.Sprintf("%x", hash)

    // 3. 验证
    if hashStr != chunkHash {
        c.JSON(http.StatusOK, gin.H{
            "valid": false,
            "expected": hashStr,
            "actual":   chunkHash,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "valid":       true,
        "chunk_hash": chunkHash,
        "chunk_size": len(data),
    })
}
```

### 3.3 节点管理 API

#### 3.3.1 获取节点信息

**请求**：
```http
GET /api/v1/nodes/info
```

**响应**：
```json
{
  "peer_id": "QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
  "addresses": [
    "/ip4/127.0.0.1/tcp/8001/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N"
  ],
  "protocols": [
    "/p2p-file-transfer/1.0.0"
  ],
  "connected_peers": 5,
  "uptime": 3600
}
```

#### 3.3.2 列出连接的节点

**请求**：
```http
GET /api/v1/nodes/peers
```

**响应**：
```json
{
  "peers": [
    {
      "peer_id": "QmPeerID1",
      "address": "/ip4/1.2.3.4/tcp/8001",
      "latency": "50ms",
      "connected_at": "2026-01-15T10:00:00Z"
    }
  ],
  "total": 5
}
```

#### 3.3.3 连接到节点

**请求**：
```http
POST /api/v1/nodes/connect
Content-Type: application/json

{
  "peer_id": "QmPeerID",
  "address": "/ip4/1.2.3.4/tcp/8001/p2p/QmPeerID"
}
```

### 3.4 DHT 操作 API

#### 3.4.1 查找提供者

**请求**：
```http
GET /api/v1/dht/providers/{key}
```

**响应**：
```json
{
  "key": "abc123...",
  "providers": [
    {
      "peer_id": "QmPeerID1",
      "addresses": ["/ip4/1.2.3.4/tcp/8001"]
    },
    {
      "peer_id": "QmPeerID2",
      "addresses": ["/ip4/5.6.7.8/tcp/8001"]
    }
  ]
}
```

#### 3.4.2 公告提供者

**请求**：
```http
POST /api/v1/dht/announce
Content-Type: application/json

{
  "key": "abc123..."
}
```

### 3.5 用户管理 API

#### 3.5.1 用户注册

**请求**：
```http
POST /api/v1/users/register
Content-Type: application/json

{
  "username": "user1",
  "email": "user1@example.com",
  "password": "securepassword"
}
```

**代码实现**：
```go
func (h *UserHandler) Register(c *gin.Context) {
    var req RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 1. 验证输入
    if err := h.validateRegister(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 2. 检查用户名是否存在
    var count int64
    h.db.Model(&User{}).Where("username = ?", req.Username).Count(&count)
    if count > 0 {
        c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
        return
    }

    // 3. 哈希密码
    passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
        return
    }

    // 4. 生成 API Key
    apiKey := generateAPIKey()

    // 5. 创建用户
    user := User{
        Username:     req.Username,
        Email:        req.Email,
        PasswordHash: string(passwordHash),
        APIKey:       apiKey,
    }

    if err := h.db.Create(&user).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "用户创建失败"})
        return
    }

    // 6. 生成 JWT Token
    token, err := h.generateJWT(user.ID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Token 生成失败"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "user_id":  user.ID,
        "username": user.Username,
        "api_key": apiKey,
        "token":   token,
    })
}
```

#### 3.5.2 用户登录

**请求**：
```http
POST /api/v1/users/login
Content-Type: application/json

{
  "username_or_email": "user1",
  "password": "securepassword"
}
```

**响应**：
```json
{
  "user_id": 1,
  "username": "user1",
  "api_key": "sk-abc123...",
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

#### 3.5.3 生成 API Key

**请求**：
```http
POST /api/v1/users/api-key
Authorization: Bearer <jwt_token>
```

**响应**：
```json
{
  "api_key": "sk-abc123xyz789",
  "created_at": "2026-01-15T10:30:00Z"
}
```

### 3.6 系统状态 API

#### 3.6.1 健康检查

**请求**：
```http
GET /api/v1/health
```

**响应**：
```json
{
  "status": "healthy",
  "timestamp": "2026-01-15T10:30:00Z",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "p2p": "ok",
    "storage": "ok"
  }
}
```

**代码实现**：
```go
func (h *HealthHandler) Check(c *gin.Context) {
    checks := make(map[string]string)

    // 检查数据库
    if err := h.db.DB().Ping(); err != nil {
        checks["database"] = "unhealthy"
    } else {
        checks["database"] = "ok"
    }

    // 检查 Redis
    if _, err := h.redis.Ping().Result(); err != nil {
        checks["redis"] = "unhealthy"
    } else {
        checks["redis"] = "ok"
    }

    // 检查 P2P 服务
    if h.p2pService == nil || h.p2pService.Host == nil {
        checks["p2p"] = "unhealthy"
    } else {
        checks["p2p"] = "ok"
    }

    // 检查存储
    if _, err := os.Stat("files"); os.IsNotExist(err) {
        checks["storage"] = "unhealthy"
    } else {
        checks["storage"] = "ok"
    }

    // 判断整体状态
    allHealthy := true
    for _, status := range checks {
        if status != "ok" {
            allHealthy = false
            break
        }
    }

    status := "healthy"
    httpStatus := http.StatusOK
    if !allHealthy {
        status = "unhealthy"
        httpStatus = http.StatusServiceUnavailable
    }

    c.JSON(httpStatus, gin.H{
        "status":    status,
        "timestamp": time.Now(),
        "checks":    checks,
    })
}
```

#### 3.6.2 统计信息

**请求**：
```http
GET /api/v1/stats
```

**响应**：
```json
{
  "files": {
    "total": 1000,
    "regular": 700,
    "chameleon": 300
  },
  "chunks": {
    "total": 10000,
    "size_total": 2621440000
  },
  "users": {
    "total": 50
  },
  "nodes": {
    "connected": 5,
    "total": 10
  }
}
```

---

## 4. 数据模型

### 4.1 核心数据结构

#### File（文件）

```go
type File struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    CID         string    `gorm:"unique;not null" json:"cid"`
    FileName    string    `gorm:"not null" json:"file_name"`
    FileSize    int64     `gorm:"not null" json:"file_size"`
    TreeType    string    `gorm:"not null" json:"tree_type"` // 'chameleon' or 'regular'
    Description string    `json:"description"`
    PublicKey   string    `gorm:"type:text" json:"public_key,omitempty"`
    RandomNum   string    `gorm:"type:text" json:"random_num,omitempty"`
    ChunkCount  int       `gorm:"not null" json:"chunk_count"`
    UploaderID  uint      `gorm:"not null" json:"uploader_id"`
    Uploader    User      `gorm:"foreignKey:UploaderID" json:"uploader,omitempty"`
    Chunks      []Chunk   `gorm:"foreignKey:FileID" json:"chunks"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

func (File) TableName() string {
    return "files"
}
```

#### Chunk（分块）

```go
type Chunk struct {
    ID         uint      `gorm:"primaryKey" json:"id"`
    FileID     uint      `gorm:"not null" json:"file_id"`
    Hash       string    `gorm:"size:64;not null" json:"hash"`
    Index      int       `gorm:"not null" json:"index"`
    Size       int       `gorm:"not null" json:"size"`
    StoragePath string    `gorm:"size:512" json:"storage_path"`
    File       File      `gorm:"foreignKey:FileID" json:"-"`
}

func (Chunk) TableName() string {
    return "chunks"
}
```

#### User（用户）

```go
type User struct {
    ID           uint      `gorm:"primaryKey" json:"id"`
    Username     string    `gorm:"unique;not null;size:50" json:"username"`
    Email        string    `gorm:"unique;not null;size:100" json:"email"`
    PasswordHash string    `gorm:"not null;size:255" json:"-"`
    APIKey       string    `gorm:"unique;size:64" json:"api_key"`
    CreatedAt    time.Time `json:"created_at"`
}

func (User) TableName() string {
    return "users"
}
```

### 4.2 请求/响应模型

#### UploadRequest

```go
type UploadRequest struct {
    TreeType    string `json:"tree_type" binding:"required,oneof=chameleon regular"`
    Description string `json:"description"`
    ChunkSize   uint   `json:"chunk_size"`
}
```

#### UploadResponse

```go
type UploadResponse struct {
    CID        string    `json:"cid"`
    FileName   string    `json:"file_name"`
    FileSize   int64     `json:"file_size"`
    TreeType   string    `json:"tree_type"`
    ChunkCount int       `json:"chunk_count"`
    CreatedAt  time.Time `json:"created_at"`
    DownloadURL string    `json:"download_url"`
}
```

#### FileInfoResponse

```go
type FileInfoResponse struct {
    CID        string           `json:"cid"`
    FileName   string           `json:"file_name"`
    FileSize   int64            `json:"file_size"`
    TreeType   string           `json:"tree_type"`
    Description string           `json:"description"`
    ChunkCount int              `json:"chunk_count"`
    Uploader   string           `json:"uploader"`
    CreatedAt  time.Time        `json:"created_at"`
    Chunks     []ChunkInfo      `json:"chunks"`
}

type ChunkInfo struct {
    Index int    `json:"index"`
    Hash  string `json:"hash"`
    Size  int    `json:"size"`
}
```

#### PaginationResponse

```go
type PaginationResponse struct {
    Page       int    `json:"page"`
    PageSize   int    `json:"page_size"`
    Total      int64  `json:"total"`
    TotalPages int    `json:"total_pages"`
}

type ListFilesResponse struct {
    Files      []FileInfoResponse `json:"files"`
    Pagination PaginationResponse  `json:"pagination"`
}
```

---

## 5. 认证与授权

### 5.1 API Key 认证

#### 中间件实现

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "p2pFileTransfer/internal/database"
)

const APIKeyHeader = "X-API-Key"

func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        apiKey := c.GetHeader(APIKeyHeader)
        if apiKey == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "缺少 API Key",
            })
            return
        }

        // 验证 API Key
        var user database.User
        if err := db.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "无效的 API Key",
            })
            return
        }

        // 将用户信息存入上下文
        c.Set("user", user)
        c.Next()
    }
}
```

#### 使用

```go
router := gin.Default()

// 需要认证的路由
api := router.Group("/api/v1")
api.Use(middleware.APIKeyAuth(db))
{
    api.POST("/files/upload", fileHandler.UploadFile)
    api.DELETE("/files/:cid", fileHandler.DeleteFile)
}

// 公开路由（不需要认证）
api.GET("/files/:cid", fileHandler.GetFileInfo)
api.GET("/files/:cid/download", fileHandler.DownloadFile)
```

### 5.2 JWT 认证

#### JWT 中间件

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v4"
)

const (
    issuer = "p2p-file-transfer"
    secretKey = "your-secret-key" // 应从环境变量读取
)

type Claims struct {
    UserID uint   `json:"user_id"`
    Username string `json:"username"`
    jwt.RegisteredClaims
}

func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := c.GetHeader("Authorization")
        if tokenString == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "缺少 Token",
            })
            return
        }

        // 去除 "Bearer " 前缀
        tokenString = strings.TrimPrefix(tokenString, "Bearer ")

        token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
            return []byte(secretKey), nil
        })

        if err != nil || !token.Valid {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "无效的 Token",
            })
            return
        }

        claims, ok := token.Claims.(*Claims)
        if !ok {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "Token 解析失败",
            })
            return
        }

        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Next()
    }
}
```

#### 生成 JWT

```go
func generateJWT(userID uint) (string, error) {
    claims := &Claims{
        UserID: userID,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    issuer,
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secretKey))
}
```

### 5.3 权限控制

#### RBAC 实现

```go
type Role string

const (
    RoleAdmin Role = "admin"
    RoleUser  Role = "user"
)

type User struct {
    // ...
    Role Role `gorm:"not null;default:'user'"`
}

func RequireRole(role Role) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, exists := c.Get("user")
        if !exists {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "未认证",
            })
            return
        }

        u := user.(User)
        if u.Role != role && u.Role != RoleAdmin {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "权限不足",
            })
            return
        }

        c.Next()
    }
}
```

#### 使用

```go
// 需要 Admin 权限
api.DELETE("/admin/files/:cid",
    middleware.JWTAuth(),
    middleware.RequireRole("admin"),
    fileHandler.DeleteFile,
)

// 需要 User 权限
api.POST("/files/upload",
    middleware.JWTAuth(),
    middleware.RequireRole("user"),
    fileHandler.UploadFile,
)
```

---

## 6. 错误处理

### 6.1 错误响应格式

```json
{
  "error": "错误信息",
  "code": "ERROR_CODE",
  "details": {},
  "timestamp": "2026-01-15T10:30:00Z"
}
```

### 6.2 错误码定义

```go
package errors

const (
    // 通用错误码 1000-1999
    ErrCodeBadRequest     = 1000
    ErrCodeUnauthorized   = 1001
    ErrCodeForbidden      = 1002
    ErrCodeNotFound       = 1003
    ErrCodeConflict       = 1004
    ErrCodeInternalError  = 1005

    // 文件错误码 2000-2999
    ErrCodeFileNotFound      = 2000
    ErrCodeFileUploadFailed  = 2001
    ErrCodeFileDownloadFailed = 2002
    ErrCodeInvalidFileType    = 2003

    // Chunk 错误码 3000-3999
    ErrCodeChunkNotFound     = 3000
    ErrCodeChunkVerifyFailed = 3001

    // P2P 错误码 4000-4999
    ErrCodeP2PConnectionFailed = 4000
    ErrCodeDHTAnnounceFailed   = 4001
)
```

### 6.3 错误处理中间件

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "logrus"
)

func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()

        // 检查是否有错误
        if len(c.Errors) > 0 {
            err := c.Errors.Last()

            // 记录错误日志
            logrus.WithFields(logrus.Fields{
                "path":   c.Request.URL.Path,
                "method": c.Request.Method,
                "ip":     c.ClientIP(),
                "error":  err.Error(),
            }).Error("Request error")

            // 返回统一格式
            c.JSON(c.Writer.Status(), gin.H{
                "error":     err.Error(),
                "code":      "INTERNAL_ERROR",
                "timestamp": time.Now(),
            })
        }
    }
}

// 自定义错误
type AppError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

func (e *AppError) Error() string {
    return e.Message
}

func NewAppError(code int, message string, details interface{}) *AppError {
    return &AppError{
        Code:    code,
        Message: message,
        Details: details,
    }
}
```

### 6.4 使用示例

```go
func (h *FileHandler) GetFile(c *gin.Context) {
    cid := c.Param("cid")

    // 查询数据库
    var file File
    if err := h.db.Where("cid = ?", cid).First(&file).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // 文件不存在
            err := middleware.NewAppError(
                middleware.ErrCodeFileNotFound,
                "文件不存在",
                map[string]string{"cid": cid},
            )
            c.AbortWithStatusJSON(http.StatusNotFound, err)
            return
        }

        // 其他错误
        c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
            "error": "数据库查询失败",
        })
        return
    }

    c.JSON(http.StatusOK, file)
}
```

---

## 7. 性能优化

### 7.1 连接池

```go
package database

import (
    "gorm.io/gorm"
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

var DB *gorm.DB
var SQLDB *sqlx.DB

// 初始化连接池
func InitDB(dsn string) error {
    var err error

    // GORM
    DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
        MaxIdleConns: 10,
        MaxOpenConns: 100,
        ConnMaxLifetime: time.Hour,
    })
    if err != nil {
        return err
    }

    // sqlx
    SQLDB, err = sqlx.Connect("postgres", dsn)
    if err != nil {
        return err
    }

    SQLDB.SetMaxOpenConns(100)
    SQLDB.SetMaxIdleConns(10)
    SQLDB.SetConnMaxLifetime(time.Hour)

    return nil
}
```

### 7.2 Redis 缓存

```go
package cache

import (
    "context"
    "encoding/json"
    "github.com/go-redis/redis/v8"
    "time"
)

type Cache struct {
    client *redis.Client
}

func NewCache(addr string) *Cache {
    return &Cache{
        client: redis.NewClient(&redis.Options{
            Addr:     addr,
            Password: "",
            DB:       0,
            PoolSize: 100,
        }),
    }
}

// 缓存文件信息
func (c *Cache) GetFile(ctx context.Context, cid string) (*FileInfoResponse, error) {
    key := fmt.Sprintf("file:%s", cid)

    val, err := c.client.Get(ctx, key).Result()
    if err != nil {
        return nil, err
    }

    var info FileInfoResponse
    if err := json.Unmarshal([]byte(val), &info); err != nil {
        return nil, err
    }

    return &info, nil
}

func (c *Cache) SetFile(ctx context.Context, cid string, info *FileInfoResponse, expiration time.Duration) error {
    key := fmt.Sprintf("file:%s", cid)

    data, err := json.Marshal(info)
    if err != nil {
        return err
    }

    return c.client.Set(ctx, key, data, expiration).Err()
}
```

### 7.3 限流

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/ulule/limiter"
    "golang.org/x/time/rate"
)

// 全局限流
func RateLimitMiddleware() gin.HandlerFunc {
    limiter := limiter.New(rate.Every(1*time.Second, 100))

    return func(c *gin.Context) {
        if limiter.Allow() {
            c.Next()
        } else {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "请求过于频繁",
            })
        }
    }
}

// IP 限流
func IPRateLimit() gin.HandlerFunc {
    limiter := limiter.New(rate.Every(1*time.Second, 10))

    return func(c *gin.Context) {
        ip := c.ClientIP()
        key := limiter.Key{
            IP: ip,
        }

        if limiter.Allow(key) {
            c.Next()
        } else {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "IP 请求过于频繁",
            })
        }
    }
}
```

### 7.4 流式上传

```go
func (h *UploadHandler) UploadStream(c *gin.Context) {
    // 流式读取大文件
    reader := c.Request.Body
    defer reader.Close()

    // 创建临时文件
    tmpFile, err := os.CreateTemp("", "upload-")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败"})
        return
    }
    defer tmpFile.Close()
    defer os.Remove(tmpFile.Name())

    // 流式复制
    written, err := io.Copy(tmpFile, reader)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "文件写入失败"})
        return
    }

    // 重置文件指针
    tmpFile.Seek(0, 0)

    // 继续处理上传...
}
```

---

## 8. 部署架构

### 8.1 单体部署

```yaml
架构:
  Web + P2P 服务运行在同一进程

优点:
  - 部署简单
  - 资源占用少

缺点:
  - 扩展性差
  - 单点故障

适用:
  - 小规模部署
  - 开发/测试环境
```

**Docker Compose**：
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: p2p_transfer
      POSTGRES_USER: p2p
      POSTGRES_PASSWORD: password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  p2p-api:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - P2P_LOG_LEVEL=info
    volumes:
      - ./files:/app/files
      - ./metadata:/app/metadata

volumes:
  postgres_data:
  redis_data:
```

### 8.2 微服务部署

```yaml
架构:
  - API 服务: 提供 HTTP API
  - Worker 服务: 处理后台任务（上传、DHT 公告）
  - P2P 服务: 管理 P2P 连接

优点:
  - 独立扩展
  - 故障隔离

缺点:
  - 部署复杂
  - 网络开销
```

**Docker Compose**：
```yaml
services:
  api:
    build: ./cmd/api
    ports:
      - "8080:8080"
    environment:
      - WORKER_QUEUE_URL=amqp://guest:guest@rabbitmq:5672
    depends_on:
      - postgres
      - redis

  worker:
    build: ./cmd/worker
    environment:
      - DB_HOST=postgres
      - AMQP_URL=amqp://guest:guest@rabbitmq:5672
    depends_on:
      - postgres
      - rabbitmq

  p2p:
    build: ./cmd/p2p-server
    environment:
      - P2P_PORT=8000
    ports:
      - "8000:8000"

  rabbitmq:
    image: rabbitmq:3-management
    ports:
      - "5672:5672"
      - "15672:15672"
```

### 8.3 Kubernetes 部署

**部署清单**：
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: p2p-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: p2p-api
  template:
    metadata:
      labels:
        app: p2p-api
    spec:
      containers:
      - name: p2p-api
        image: p2p-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: db-host
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
        volumeMounts:
        - name: storage
          mountPath: /app/files
      volumes:
      - name: storage
        persistentVolumeClaim:
          claimName: p2p-storage
---
apiVersion: v1
kind: Service
metadata:
  name: p2p-api
spec:
  selector:
    app: p2p-api
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  db-host: "postgres.default.svc.cluster.local"
  redis-host: "redis.default.svc.cluster.local"
```

---

## 9. 实施计划

### 9.1 第一阶段：基础 API（1-2周）

**目标**：实现基本的文件上传、下载功能

**任务**：
1. 搭建 Gin 框架
2. 实现文件上传 API
3. 实现文件下载 API
4. 实现文件信息查询 API
5. 集成现有 P2P 服务
6. 单元测试

**验收**：
- ✅ 可以通过 API 上传文件
- ✅ 可以通过 API 下载文件
- ✅ 所有测试通过

### 9.2 第二阶段：认证和用户管理（1周）

**目标**：添加用户认证和权限控制

**任务**：
1. 实现用户注册/登录 API
2. 实现 JWT 认证
3. 实现 API Key 认证
4. 添加权限控制
5. 数据库集成

**验收**：
- ✅ 用户可以注册/登录
- ✅ API 受保护
- ✅ 权限控制正常工作

### 9.3 第三阶段：高级功能（1-2周）

**目标**：添加 Chunk 管理和节点管理

**任务**：
1. 实现 Chunk 上传/下载 API
2. 实现 Chunk 验证 API
3. 实现节点信息查询
4. 实现 DHT 操作 API
5. 添加监控和日志

**验收**：
- ✅ 所有高级功能正常
- ✅ 监控指标正常收集
- ✅ 日志正确记录

### 9.4 第四阶段：优化和部署（1周）

**目标**：性能优化和生产部署

**任务**：
1. 添加缓存层
2. 实现限流
3. 编写 Docker 部署文件
4. 编写 Kubernetes 部署文件
5. 性能测试和调优

**验收**：
- ✅ QPS 达到 1000+
- ✅ 内存占用合理
- ✅ 可以成功部署到生产

---

## 附录

### A. 完整 API 列表

```
# 文件管理
GET    /api/v1/files                    # 列出文件
POST   /api/v1/files/upload            # 上传文件
GET    /api/v1/files/{cid}               # 获取文件信息
GET    /api/v1/files/{cid}/download     # 下载文件
PUT    /api/v1/files/{cid}               # 更新文件
DELETE /api/v1/files/{cid}               # 删除文件

# Chunk 管理
GET    /api/v1/chunks/{hash}             # 获取 Chunk
POST   /api/v1/chunks                  # 上传 Chunk
DELETE /api/v1/chunks/{hash}           # 删除 Chunk
POST   /api/v1/chunks/{hash}/verify     # 验证 Chunk

# 节点管理
GET    /api/v1/nodes/info               # 获取节点信息
GET    /api/v1/nodes/peers              # 列出连接的节点
POST   /api/v1/nodes/connect           # 连接到节点
DELETE /api/v1/nodes/{peerId}        # 断开连接

# DHT 操作
GET    /api/v1/dht/providers/{key}      # 查找提供者
POST   /api/v1/dht/announce            # 公告提供者

# 用户管理
POST   /api/v1/users/register          # 用户注册
POST   /api/v1/users/login             # 用户登录
POST   /api/v1/users/api-key           # 生成 API Key
GET    /api/v1/users/me                # 获取当前用户信息

# 系统状态
GET    /api/v1/health                  # 健康检查
GET    /api/v1/stats                   # 统计信息
```

### B. OpenAPI 规范示例

```yaml
openapi: 3.0.0
info:
  title: P2P File Transfer API
  version: 1.0.0
  description: P2P 文件传输系统 HTTP API
servers:
  - url: http://localhost:8080/api/v1
    description: 本地开发服务器

paths:
  /files/upload:
    post:
      summary: 上传文件
      tags:
        - files
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
                tree_type:
                  type: string
                  enum: [chameleon, regular]
                  default: regular
                description:
                  type: string
      responses:
        '200':
          description: 上传成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UploadResponse'

components:
  schemas:
    UploadResponse:
      type: object
      properties:
        cid:
          type: string
        file_name:
          type: string
        file_size:
          type: integer
        tree_type:
          type: string
        chunk_count:
          type: integer
        created_at:
          type: string
          format: date-time
```

### C. 依赖安装

```bash
# 安装依赖
go get -u github.com/gin-gonic/gin
go get -u github.com/gin-contrib/cors
go get -u github.com/swaggo/gin-swagger
go get -u github.com/swaggo/files
go get -u gorm.io/gorm
go get -u github.com/lib/pq
go get -u github.com/redis/go-redis/v9
go get -u github.com/golang-jwt/jwt/v4
go get -u github.com/ulule/limiter

# 安装 Swagger CLI
go install github.com/swaggo/swag/cmd/swag@latest
```

### D. 快速启动

```bash
# 1. 启动数据库（使用 Docker）
docker-compose up -d postgres redis

# 2. 运行数据库迁移
go run cmd/migrate/main.go

# 3. 启动 API 服务
go run cmd/api/main.go

# 4. 生成 Swagger 文档
swag init
swag generate

# 5. 访问 API
curl http://localhost:8080/api/v1/health
```

---

**文档版本**: v1.0.0
**最后更新**: 2026-01-15
**状态**: 设计阶段

**下一步**: 开始实施第一阶段（基础 API）
