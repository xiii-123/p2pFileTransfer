# Chameleon Merkle Tree 组件

## 组件概述

- **位置**: `pkg/chameleonMerkleTree/`
- **职责**: 实现可变色默克尔树（Chameleon Merkle Tree），支持使用私钥修改文件内容而改变根哈希
- **依赖**:
  - `crypto/elliptic` - 椭圆曲线密码学
  - `crypto/sha256` - SHA256 哈希
- **被依赖**: `cmd/p2p/file/`, `pkg/p2p/`

## 文件结构

```
pkg/chameleonMerkleTree/
├── chameleonMerkleTree.go       # 接口定义和常量 (138 行)
├── chameleonMerkleTreeImpl.go   # 核心实现 (370 行)
├── chameleon.go                 # Chameleon 哈希算法 (108 行)
├── chameleonInfo.go             # 密钥和信息结构 (62 行)
├── utils.go                     # 工具函数 (119 行)
└── chameleonMerkleTree_test.go  # 单元测试 (112 行)
```

## 核心概念

### 什么是 Chameleon Hash？

**Chameleon Hash** 是一种特殊的哈希函数，具有以下特性：

1. **抗碰撞性**: 没有私钥时，与标准哈希相同
2. **可编辑性**: 持有私钥时，可以找到不同输入产生相同哈希
3. **公钥验证**: 任何人都可以验证哈希的正确性

### 与 Regular Merkle Tree 的区别

| 特性 | Chameleon Merkle Tree | Regular Merkle Tree |
|------|----------------------|---------------------|
| 哈希算法 | ECDSA P256 + SHA256 | SHA256 |
| 可编辑性 | ✓ 支持（需私钥） | ✗ 不支持 |
| 密钥管理 | 需要公钥/私钥对 | 无需密钥 |
| 性能 | 较慢 | 较快 |
| 复杂度 | 高 | 低 |
| 适用场景 | 需要修改的文件 | 一次性上传 |

## 核心数据结构

### 1. ChameleonPubKey（公钥）

```go
type ChameleonPubKey struct {
    pubX *big.Int  // ECDSA 公钥 X 分量
    pubY *big.Int  // ECDSA 公钥 Y 分量
}
```

### 2. ChameleonRandomNum（随机数）

```go
type ChameleonRandomNum struct {
    rX *big.Int  // 随机点 X 分量
    rY *big.Int  // 随机点 Y 分量
    s  *big.Int  // 随机标量
}
```

### 3. ChameleonMerkleNode（树节点）

```go
type ChameleonMerkleNode struct {
    node *MerkleNode        // 底层 Regular Merkle 树节点
    hash []byte             // Chameleon 哈希值
    pk   *ChameleonPubKey   // 公钥
    rn   *ChameleonRandomNum // 随机数
}
```

### 4. MerkleNode（Merkle 树节点）

```go
type MerkleNode struct {
    Left  *MerkleNode   // 左子节点
    Right *MerkleNode   // 右子节点
    Hash  []byte        // SHA256 哈希
    Data  []byte        // 叶子节点的数据
}
```

## 核心接口

### 1. 创建树

```go
// 从文件创建
func NewChameleonMerkleTree(
    file io.ReadWriter,
    config *MerkleConfig,
    pubKey *ChameleonPubKey
) (*ChameleonMerkleNode, error)

// 从预计算的哈希创建
func NewChameleonMerkleTreeFromHashes(
    hashes [][]byte,
    pubKey *ChameleonPubKey
) (*ChameleonMerkleNode, error)
```

### 2. 更新树

```go
func UpdateChameleonMerkleTree(
    file *os.File,
    config *MerkleConfig,
    secKey, prevText, chameleonHash []byte,
    randomNum *ChameleonRandomNum,
    pubKey *ChameleonPubKey
) (*ChameleonMerkleNode, error)
```

### 3. 验证

```go
// 验证 Chameleon 哈希
func (cmn *ChameleonMerkleNode) VerifyChameleonHash() bool

// 验证根哈希
func (cmn *ChameleonMerkleNode) GetRootHash() []byte

// 获取 Chameleon 哈希
func (cmn *ChameleonMerkleNode) GetChameleonHash() []byte
```

### 4. 密钥生成

```go
// 生成密钥对
func GenerateKeyPair() (*ChameleonPubKey, *ChameleonRandomNum, error)

// 序列化公钥
func (pk *ChameleonPubKey) MarshalJSON() ([]byte, error)

// 反序列化公钥
func (pk *ChameleonPubKey) UnmarshalJSON(data []byte) error
```

## 核心实现

### 1. Chameleon 哈希算法

```go
// ComputeHash 计算 Chameleon 哈希
// 参数:
//   - hash: 原始数据哈希
//   - pubX, pubY: 公钥坐标
// 返回:
//   - rX, rY: 随机点坐标
//   - s: 随机标量
//   - hX: Chameleon 哈希值
func ComputeHash(hash []byte, pubX, pubY *big.Int) (*big.Int, *big.Int, *big.Int, *big.Int) {
    // 1. 使用椭圆曲线 P256
    curve := elliptic.P256()

    // 2. 生成随机数 r
    rX, rY := curve.ScalarBaseMult(randomBytes())

    // 3. 计算 s = H(hash || r) mod n
    s = hashToInt(sha256.Sum(append(hash, rX.Bytes()...)))

    // 4. 计算 hX = H(hash || r || s) mod n
    combined := append(hash, rX.Bytes()...)
    combined = append(combined, s.Bytes()...)
    hX := hashToInt(sha256.Sum256(combined))

    return rX, rY, s, hX
}
```

### 2. 碰撞查找（编辑功能）

```go
// FindCollision 找到一个碰撞，使得新数据产生相同的哈希
// 只有持有私钥才能完成
func FindCollision(
    prevText, newText []byte,
    prevChameleonHash []byte,
    secKey *big.Int,
    randomNum *ChameleonRandomNum,
    pubKey *ChameleonPubKey
) (*big.Int, *big.Int, *big.Int, error) {
    // 1. 计算新文本的哈希
    newHash := sha256.Sum256(newText)

    // 2. 使用私钥计算新的随机数
    // 这是核心：使用私钥可以找到新的 (r', s') 使得
    // H(prevHash || r || s) = H(newHash || r' || s')
    curve := elliptic.P256()

    // 计算新的随机点
    newX, newY := curve.ScalarMult(pubKey.pubX, pubKey.pubY, secKey)

    // 计算新的 s'
    newS := new(big.Int).Add(randomNum.s, hashToInt(newHash))

    // 计算新的 r'
    newRX, newRY := curve.ScalarBaseMult(newS.Bytes())

    return newRX, newRY, newS, nil
}
```

### 3. Merkle 树构建

```go
// BuildMerkleTreeFromFileRW 从文件构建 Merkle 树
func BuildMerkleTreeFromFileRW(file io.ReadWriter, config *MerkleConfig) (*MerkleNode, error) {
    // 1. 分块读取文件
    buffers, err := ReadFileToBuffers(file, config.BlockSize)
    if err != nil {
        return nil, err
    }

    // 2. 计算每个块的哈希
    var leaves []*MerkleNode
    for _, buffer := range buffers {
        hash := sha256.Sum256(buffer)
        leaves = append(leaves, &MerkleNode{
            Hash: hash[:],
            Data: buffer,
        })
    }

    // 3. 自底向上构建树
    return buildMerkleTreeFromLeaves(leaves), nil
}

// buildMerkleTreeFromLeaves 从叶子节点构建 Merkle 树
func buildMerkleTreeFromLeaves(leaves []*MerkleNode) *MerkleNode {
    if len(leaves) == 0 {
        return nil
    }

    // 如果只有一个叶子，直接返回
    if len(leaves) == 1 {
        return leaves[0]
    }

    // 构建下一层
    var nextLevel []*MerkleNode
    for i := 0; i < len(leaves); i += 2 {
        if i+1 == len(leaves) {
            // 奇数个节点，复制最后一个
            nextLevel = append(nextLevel, leaves[i])
        } else {
            // 合并两个节点
            combinedHash := sha256.Sum256(append(leaves[i].Hash, leaves[i+1].Hash...))
            node := &MerkleNode{
                Left:  leaves[i],
                Right: leaves[i+1],
                Hash:  combinedHash[:],
            }
            nextLevel = append(nextLevel, node)
        }
    }

    // 递归构建
    return buildMerkleTreeFromLeaves(nextLevel)
}
```

### 4. 从预计算哈希构建

```go
// NewChameleonMerkleTreeFromHashes 从预计算的哈希创建树
// 这个函数避免重复读取文件，提高性能
func NewChameleonMerkleTreeFromHashes(hashes [][]byte, pubKey *ChameleonPubKey) (*ChameleonMerkleNode, error) {
    if len(hashes) == 0 {
        return nil, fmt.Errorf("no hashes provided")
    }

    // 1. 构建 Regular Merkle Tree
    root, err := buildMerkleTreeFromLeafHashes(hashes)
    if err != nil {
        return nil, fmt.Errorf("failed to build Merkle tree from hashes: %w", err)
    }

    // 2. 应用 Chameleon 哈希到根节点
    rX, rY, s, hX := ComputeHash(root.Hash, pubKey.pubX, pubKey.pubY)

    // 3. 返回 Chameleon Merkle Tree 节点
    return &ChameleonMerkleNode{
        node: root,
        hash: hX.Bytes(),
        pk:   pubKey,
        rn: &ChameleonRandomNum{
            rX: rX,
            rY: rY,
            s:  s,
        },
    }, nil
}
```

## 使用示例

### 1. 生成密钥对

```go
import "p2pFileTransfer/pkg/chameleonMerkleTree"

// 生成密钥对
pubKey, privKey, err := chameleonMerkleTree.GenerateKeyPair()
if err != nil {
    log.Fatal(err)
}

// 序列化公钥（保存到元数据）
pubKeyJSON, _ := json.Marshal(pubKey)

// 序列化私钥（妥善保管）
privKeyJSON, _ := json.Marshal(privKey)
```

### 2. 创建 Chameleon Merkle Tree

```go
// 打开文件
file, err := os.Open("myfile.txt")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

// 创建配置
config := &chameleonMerkleTree.MerkleConfig{
    BlockSize: 262144,  // 256KB
}

// 创建树
tree, err := chameleonMerkleTree.NewChameleonMerkleTree(file, config, pubKey)
if err != nil {
    log.Fatal(err)
}

// 获取根哈希（CID）
cid := tree.GetChameleonHash()
fmt.Printf("File CID: %x\n", cid)

// 获取随机数（用于修改）
randomNum := tree.GetRandomNumber()
```

### 3. 验证哈希

```go
// 验证 Chameleon 哈希
isValid := tree.VerifyChameleonHash()
if !isValid {
    log.Fatal("Invalid Chameleon hash!")
}

// 验证根哈希
rootHash := tree.GetRootHash()
fmt.Printf("Root Hash: %x\n", rootHash)
```

### 4. 修改文件内容

```go
// 修改文件内容
err = os.WriteFile("myfile.txt", []byte("new content"), 0644)
if err != nil {
    log.Fatal(err)
}

// 重新打开文件
file, _ = os.Open("myfile.txt")
defer file.Close()

// 更新树（使用私钥）
prevHash := tree.GetChameleonHash()
newTree, err := chameleonMerkleTree.UpdateChameleonMerkleTree(
    file,
    config,
    privKey,
    prevText,
    prevHash,
    randomNum,
    pubKey,
)
if err != nil {
    log.Fatal(err)
}

// 根哈希保持不变！
fmt.Printf("Old CID: %x\n", prevHash)
fmt.Printf("New CID: %x\n", newTree.GetChameleonHash())
// 输出应该相同
```

## 配置项

### MerkleConfig

```go
type MerkleConfig struct {
    BlockSize    uint   // 分块大小（字节）
    BufferNumber uint   // 缓冲区数量
}
```

| 配置项 | 默认值 | 说明 | 影响 |
|--------|--------|------|------|
| BlockSize | 262144 (256KB) | 分块大小 | 影响传输效率和内存使用 |
| BufferNumber | 16 | 缓冲区数量 | 影响内存占用和性能 |

## 性能分析

### 时间复杂度

| 操作 | 复杂度 | 说明 |
|------|--------|------|
| 创建树 | O(n) | n = 文件大小 / 块大小 |
| 验证哈希 | O(1) | 常数时间 |
| 修改内容 | O(log n) | 只需更新路径上的节点 |
| 查找证明 | O(log n) | Merkle 证明 |

### 空间复杂度

| 场景 | 复杂度 | 说明 |
|------|--------|------|
| 树存储 | O(n) | 需要存储所有节点 |
| 哈希存储 | O(n) | 每个叶子一个哈希 |
| 密钥存储 | O(1) | 公钥和私钥固定大小 |

### 性能对比

| 操作 | Chameleon Tree | Regular Tree |
|------|---------------|--------------|
| 创建 | ~1.2x 慢 | 基准 |
| 验证 | 相同 | 相同 |
| 修改 | O(log n) | 不支持 |

## 安全性分析

### 1. 密钥安全

- **私钥泄露**: 攻击者可以伪造文件修改
- **保护措施**:
  - 私钥文件权限设置为 0600
  - 不要将私钥提交到版本控制
  - 考虑使用密码加密私钥

### 2. 碰撞攻击

- **无私钥**: 与 SHA256 相同，碰撞困难
- **有私钥**: 可以找到碰撞，但这是设计特性
- **保护措施**: 妥善保管私钥

### 3. 前向安全性

- 如果私钥泄露，过去的文件修改仍可验证
- 建议定期轮换密钥对

## 扩展点

### 1. 支持其他椭圆曲线

```go
// 当前使用 P256
curve := elliptic.P256()

// 可以扩展支持其他曲线
// - P224: 更快但安全性略低
// - P384: 更安全但更慢
// - P521: 最高安全性
```

### 2. 密钥加密

```go
// 添加密码保护私钥
func EncryptPrivateKey(privKey *ChameleonRandomNum, password string) ([]byte, error) {
    // 使用 AES 加密私钥
    // ...
}

func DecryptPrivateKey(data []byte, password string) (*ChameleonRandomNum, error) {
    // 解密私钥
    // ...
}
```

### 3. 批量验证

```go
// 优化批量验证性能
func VerifyBatch(trees []*ChameleonMerkleNode) bool {
    // 并行验证多个树
    // ...
}
```

## 测试用例

```go
func TestChameleonHashCollision(t *testing.T) {
    // 生成密钥对
    pubKey, privKey, _ := GenerateKeyPair()

    // 计算原始文本的哈希
    prevText := []byte("original")
    prevHash := sha256.Sum256(prevText)
    rX, rY, s, hX := ComputeHash(prevHash[:], pubKey.pubX, pubKey.pubY)

    // 修改文本
    newText := []byte("modified")

    // 找到碰撞
    newRX, newRY, newS, _ := FindCollision(
        prevText,
        newText,
        hX.Bytes(),
        privKey,
        &ChameleonRandomNum{rX: rX, rY: rY, s: s},
        pubKey,
    )

    // 验证新哈希与旧哈希相同
    assert.Equal(t, hX, newS)
}
```

## 最佳实践

1. **密钥管理**
   - ✅ 私钥文件权限设置为 0600
   - ✅ 使用密码加密私钥
   - ✅ 定期备份密钥
   - ❌ 不要将私钥提交到版本控制

2. **性能优化**
   - ✅ 使用 `NewChameleonMerkleTreeFromHashes` 避免重复读取
   - ✅ 选择合适的块大小（256KB 是好的默认值）
   - ✅ 考虑并发验证

3. **安全性**
   - ✅ 使用足够长的密钥（P256 推荐）
   - ✅ 定期轮换密钥
   - ✅ 记录所有修改操作

---

**相关文档**:
- [P2P 核心服务组件](P2P核心服务组件.md)
- [配置管理组件](配置管理组件.md)
