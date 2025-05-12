package chameleonMerkleTree

import (
	"bytes"
	"container/list"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"math/big"
	"os"
)

var log = logrus.New()

// NewMerkleConfig 创建一个新的Merkle树配置
func NewMerkleConfig() *MerkleConfig {
	return &MerkleConfig{
		BlockSize: DefaultBlockSize,
	}
}

// VerifyChameleonHash 验证Merkle树的根节点是否正确
func (cmn *ChameleonMerkleNode) VerifyChameleonHash() bool {
	randomNum := cmn.rn
	pubKey := cmn.pk
	return VerifyHash(cmn.node.Hash, randomNum.rX, randomNum.rY, randomNum.s, pubKey.pubX, pubKey.pubY, new(big.Int).SetBytes(cmn.hash))
}

func (cmn *ChameleonMerkleNode) GetChameleonHash() []byte {
	return cmn.hash
}

func (cmn *ChameleonMerkleNode) GetRootHash() []byte {
	return cmn.node.Hash
}

func NewChameleonMerkleTree(file io.ReadWriter, config *MerkleConfig, pubKey *ChameleonPubKey) (*ChameleonMerkleNode, error) {
	if file == nil {
		return nil, fmt.Errorf("file is nil")
	}

	node, err := BuildMerkleTreeFromFileRW(file, config)
	if err != nil {
		return nil, err
	}

	// 使用变色龙哈希生成根节点

	rX, rY, s, hX := ComputeHash(node.Hash, pubKey.pubX, pubKey.pubY)
	return &ChameleonMerkleNode{
		node: node,
		hash: hX.Bytes(),
		pk:   pubKey,
		rn: &ChameleonRandomNum{
			rX: rX,
			rY: rY,
			s:  s,
		},
	}, nil
}

// UpdateChameleonMerkleTree 更新Merkle树
func UpdateChameleonMerkleTree(file *os.File, config *MerkleConfig, secKey, prevText, chameleonHash []byte, randomNum *ChameleonRandomNum, pubKey *ChameleonPubKey) (*ChameleonMerkleNode, error) {
	if file == nil {
		return nil, fmt.Errorf("file is nil")
	}
	if !CheckBytes(secKey) {
		return nil, fmt.Errorf("secKey is nil")
	}
	if !CheckBytes(prevText) {
		return nil, fmt.Errorf("prevText is nil")
	}
	if !CheckBytes(chameleonHash) {
		return nil, fmt.Errorf("chameleonHash is nil")
	}

	node, err := BuildMerkleTreeFromFileRW(file, config)
	if err != nil {
		return nil, err
	}
	newText := node.Hash

	newRX, newRY, newS := FindCollision(prevText, randomNum.rX, randomNum.rY, randomNum.s, new(big.Int).SetBytes(chameleonHash), newText, secKey)

	return &ChameleonMerkleNode{
		node: node,
		hash: chameleonHash,
		pk:   pubKey,
		rn: &ChameleonRandomNum{
			rX: newRX,
			rY: newRY,
			s:  newS,
		},
	}, nil
}

// LevelOrderTraversal 层序遍历Merkle树并打印结构
func LevelOrderTraversal(root *MerkleNode) {
	if root == nil {
		return
	}

	queue := list.New()
	queue.PushBack(root)

	for queue.Len() > 0 {
		levelSize := queue.Len()
		for i := 0; i < levelSize; i++ {
			node := queue.Remove(queue.Front()).(*MerkleNode)
			fmt.Printf("%x ", node.Hash[:8])
			if node.Left != nil {
				queue.PushBack(node.Left)
			}
			if node.Right != nil {
				queue.PushBack(node.Right)
			}
		}
		fmt.Println() // 打印完一层后换行
	}
}

// getAllLeaves 从Merkle树的根节点获取所有叶子节点的哈希值
func (cmn *ChameleonMerkleNode) GetAllLeavesHashes() [][]byte {
	var leafHashes [][]byte
	root := cmn.node
	if root == nil {
		return leafHashes
	}

	// 使用队列进行层序遍历
	queue := list.New()
	queue.PushBack(root)

	for queue.Len() > 0 {
		node := queue.Remove(queue.Front()).(*MerkleNode)
		// 如果是叶子节点，则添加到leafHashes列表中
		if node.Left == nil && node.Right == nil {
			leafHashes = append(leafHashes, node.Hash)
		}
		// 将子节点加入队列
		if node.Left != nil {
			queue.PushBack(node.Left)
		}
		if node.Right != nil {
			queue.PushBack(node.Right)
		}
	}

	return leafHashes
}

// GenerateProof 函数生成给定目标节点的默克尔证明路径。
// 它使用深度优先搜索（DFS）算法从根节点开始遍历默克尔树，
// 并收集从根节点到目标节点路径上的所有兄弟节点的哈希值。
//
// 参数:
// - root: 指向默克尔树根节点的指针。
// - target: 目标节点的哈希值。
//
// 返回值:
// 返回一个二维字节数组，其中包含从根节点到目标节点路径上所有兄弟节点的哈希值。
//
// 示例:
// 假设有一个默克尔树，其结构如下:
//
//	     root
//	    /    \
//	  A        B
//	 / \      / \
//	C   D    E   F
//
// 如果目标节点是 D，则返回的兄弟节点哈希值数组为 [[C][][][B]]。其中，空格表示路径上此高度节点所在的位置。
func (cmn *ChameleonMerkleNode) GenerateProof(target []byte) ([][][]byte, error) {
	var proof [][][]byte
	var path []*MerkleNode

	var findTarget func(n *MerkleNode) bool
	findTarget = func(n *MerkleNode) bool {
		if n == nil {
			return false
		}

		path = append(path, n)
		defer func() { path = path[:len(path)-1] }()

		if bytes.Equal(n.Hash, target) {
			return true
		}

		return findTarget(n.Left) || findTarget(n.Right)
	}

	if !findTarget(cmn.node) {
		return nil, fmt.Errorf("target hash not found in the tree")
	}

	for i := len(path) - 1; i > 0; i-- {
		current := path[i]
		parent := path[i-1]

		layer := [][]byte{nil, nil} // [left sibling, right sibling]

		if parent.Left == current {
			if parent.Right != nil {
				layer[1] = parent.Right.Hash
			}
		} else {
			if parent.Left != nil {
				layer[0] = parent.Left.Hash
			}
		}

		proof = append(proof, layer)
	}

	return proof, nil
}

// VerifyProof 验证给定的 Merkle 证明是否有效。
func (cmn *ChameleonMerkleNode) VerifyProof(proof [][][]byte, targetHash []byte) bool {
	currentHash := targetHash

	for _, layer := range proof {
		left, right := layer[0], layer[1]

		var combined []byte
		if left == nil {
			// 当前节点在左边
			combined = append(currentHash, right...)
		} else {
			// 当前节点在右边
			combined = append(left, currentHash...)
		}

		// 重新计算当前节点的hash
		currentHash, _ = sha256Hash(combined)
	}

	// 最后验证是不是等于根节点
	return bytes.Equal(currentHash, cmn.node.Hash)
}

// 序列化：只保存叶子节点
func (cmn *ChameleonMerkleNode) Serialize() ([]byte, error) {
	leaves := cmn.GetAllLeavesHashes()

	var leavesEncoded []string
	for _, leaf := range leaves {
		leavesEncoded = append(leavesEncoded, base64.StdEncoding.EncodeToString(leaf))
	}

	ser := chameleonMerkleNodeLite{
		Hash: base64.StdEncoding.EncodeToString(cmn.hash),
		PK: &chameleonPubKeySerializable{
			PubX: cmn.pk.pubX.String(),
			PubY: cmn.pk.pubY.String(),
		},
		RN: &chameleonRandomNumSerializable{
			RX: cmn.rn.rX.String(),
			RY: cmn.rn.rY.String(),
			S:  cmn.rn.s.String(),
		},
		Leaves: leavesEncoded,
	}

	return json.Marshal(ser)
}

func DeserializeChameleonMerkleTree(data []byte) (*ChameleonMerkleNode, error) {
	// 先反序列化到辅助结构体
	var lite chameleonMerkleNodeLite
	if err := json.Unmarshal(data, &lite); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ChameleonMerkleNode: %w", err)
	}

	// 还原hash
	rootHash, err := base64.StdEncoding.DecodeString(lite.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode root hash: %w", err)
	}

	// 还原公钥
	pubX := new(big.Int)
	pubX.SetString(lite.PK.PubX, 10)
	pubY := new(big.Int)
	pubY.SetString(lite.PK.PubY, 10)
	pk := &ChameleonPubKey{
		pubX: pubX,
		pubY: pubY,
	}

	// 还原随机数
	rx := new(big.Int)
	rx.SetString(lite.RN.RX, 10)
	ry := new(big.Int)
	ry.SetString(lite.RN.RY, 10)
	s := new(big.Int)
	s.SetString(lite.RN.S, 10)
	rn := &ChameleonRandomNum{
		rX: rx,
		rY: ry,
		s:  s,
	}

	// 还原所有叶子节点
	var leafHashes [][]byte
	for _, encodedLeaf := range lite.Leaves {
		leafHash, err := base64.StdEncoding.DecodeString(encodedLeaf)
		if err != nil {
			return nil, fmt.Errorf("failed to decode leaf hash: %w", err)
		}
		leafHashes = append(leafHashes, leafHash)
	}

	if len(leafHashes) == 0 {
		return nil, fmt.Errorf("no leaf nodes found")
	}

	// 根据叶子节点重建Merkle树
	root, err := buildMerkleTreeFromLeafHashes(leafHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild Merkle tree: %w", err)
	}

	return &ChameleonMerkleNode{
		node: root,
		hash: rootHash,
		pk:   pk,
		rn:   rn,
	}, nil
}

// GetPublicKey returns the Chameleon public key
func (cmn *ChameleonMerkleNode) GetPublicKey() *ChameleonPubKey {
	return cmn.pk
}

// GetRandomNumber returns the random number used in Chameleon hash
func (cmn *ChameleonMerkleNode) GetRandomNumber() *ChameleonRandomNum {
	return cmn.rn
}
