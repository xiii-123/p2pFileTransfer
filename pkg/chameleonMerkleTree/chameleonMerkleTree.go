// Package chameleonMerkleTree 提供 Chameleon 哈希和 Merkle 树实现
//
// 核心功能:
//   - Chameleon 哈希: 基于椭圆曲线的可编辑哈希函数
//   - Merkle 树: 构建和验证 Merkle 树
//   - 文件分块: 将大文件分割为固定大小的块
//   - 完整性验证: 使用根哈希验证文件完整性
//
// 主要组件:
//   - ChameleonHash: Chameleon 哈希函数实现
//   - ChameleonMerkleTree: Merkle 树结构
//   - ChameleonPubKey: 公钥结构
//   - ChameleonRandomNum: 随机数结构
//
// 特性:
//   - 可编辑: 在已知私钥的情况下可以修改哈希值
//   - 安全性: 基于椭圆曲线离散对数问题
//   - 高效: 支持并发构建 Merkle 树
//
// 使用场景:
//   - 区块链: 可编辑的区块链状态
//   - 文件验证: P2P 文件传输中的完整性验证
//   - 数据完整性: 任何需要可验证数据完整性的场景
//
// 配置:
//   - BlockSize: 默认 256KB
//   - BufferNumber: 默认 16
//
// 注意事项:
//   - 私钥必须妥善保管，泄露后任何人都可以修改哈希
//   - 修改哈希需要知道原始随机数
package chameleonMerkleTree

import (
	"math/big"
)

// MerkleConfig contains configuration for Merkle tree construction
// MerkleConfig holds configuration for building a Merkle tree
type MerkleConfig struct {
	BlockSize    uint // 每个文件块的大小
	BufferNumber uint // channel缓冲区大小
}

// NewDefaultMerkleConfig returns a default configuration
func NewDefaultMerkleConfig() *MerkleConfig {
	return &MerkleConfig{
		BlockSize:    256 * 1024, // 默认256KB块
		BufferNumber: 16,         // 缓冲channel 16个
	}
}

// ChameleonPubKey represents the public key for Chameleon hash
type ChameleonPubKey struct {
	pubX *big.Int
	pubY *big.Int
}

// ChameleonRandomNum contains random numbers for Chameleon hash
type ChameleonRandomNum struct {
	rX *big.Int
	rY *big.Int
	s  *big.Int
}

// MerkleNode represents a node in the Merkle tree
type MerkleNode struct {
	Hash   []byte
	Left   *MerkleNode
	Right  *MerkleNode
	Parent *MerkleNode
}

// ChameleonMerkleNode extends MerkleNode with Chameleon hash capabilities
type ChameleonMerkleNode struct {
	node *MerkleNode
	hash []byte
	pk   *ChameleonPubKey
	rn   *ChameleonRandomNum
}

// json序列化辅助结构体
type chameleonMerkleNodeLite struct {
	Hash   string                          `json:"hash"`
	PK     *chameleonPubKeySerializable    `json:"pk"`
	RN     *chameleonRandomNumSerializable `json:"rn"`
	Leaves []string                        `json:"leaves"` // 只保存叶子节点哈希
}

// 保持原来的pk和rn辅助结构
type chameleonPubKeySerializable struct {
	PubX string `json:"pubX"`
	PubY string `json:"pubY"`
}

type chameleonRandomNumSerializable struct {
	RX string `json:"rX"`
	RY string `json:"rY"`
	S  string `json:"s"`
}

const (
	DefaultBlockSize = 256 * 1024 // 256KB
)

// MerkleTree defines the basic Merkle tree operations
type MerkleTree interface {
	// GetRootHash returns the root hash of the Merkle tree
	GetRootHash() []byte

	// GenerateProof creates a Merkle proof for a leaf at given index
	GenerateProof(target []byte) ([][][]byte, error)

	// VerifyProof verifies a Merkle proof against the root hash
	VerifyProof(proof [][][]byte, targetHash []byte) bool

	// Serialize converts the tree to a byte representation
	Serialize() ([]byte, error)
}

// ChameleonMerkleTree extends MerkleTree with Chameleon hash capabilities
type ChameleonMerkleTree interface {
	MerkleTree

	// VerifyUpdate verifies an chameleon hash root was performed correctly
	VerifyChameleonHash(randomNumber *ChameleonRandomNum, pubKey *ChameleonPubKey) bool

	// GetPublicKey returns the Chameleon public key
	GetPublicKey() *ChameleonPubKey

	// GetRandomNumber returns the random number used in Chameleon hash
	GetRandomNumber() *ChameleonRandomNum

	GetChameleonHash() []byte

	// GetAllLeavesHashes returns all leaf hashes in the tree
	GetAllLeavesHashes() [][]byte
}
