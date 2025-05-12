package chameleonMerkleTree

import (
	"crypto/sha256"
	"fmt"
	"io"
)

func CheckBytes(bytes []byte) bool {
	if bytes == nil || len(bytes) == 0 {
		return false
	}
	return true
}

func sha256Hash(data []byte) ([]byte, error) {
	h := sha256.New()
	if _, err := h.Write(data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
func ReadFileToBuffers(file io.ReadWriter, blockSize uint) ([][]byte, error) {
	if file == nil {
		return nil, fmt.Errorf("file is nil")
	}

	var buffers [][]byte
	buffer := make([]byte, blockSize)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		// 拷贝有效数据，避免复用导致的问题
		data := make([]byte, n)
		copy(data, buffer[:n])
		buffers = append(buffers, data)

		if err == io.EOF {
			break
		}
	}

	if len(buffers) == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	return buffers, nil
}

// buildMerkleTreeFromLeafHashes 从叶子哈希构建Merkle树（直接使用已有哈希）
// leaves 是一组叶子节点的哈希值
func buildMerkleTreeFromLeafHashes(leaves [][]byte) (*MerkleNode, error) {
	if len(leaves) == 0 {
		return nil, fmt.Errorf("no leaves provided")
	}

	// 将每个hash包装成一个叶子节点
	var nodes []*MerkleNode
	for _, hash := range leaves {
		nodes = append(nodes, &MerkleNode{Hash: hash})
	}

	// 递归构建Merkle树
	for len(nodes) > 1 {
		var newLevel []*MerkleNode
		for i := 0; i < len(nodes); i += 2 {
			left := nodes[i]
			var right *MerkleNode
			if i+1 < len(nodes) {
				right = nodes[i+1]
			} else {
				right = left // 处理奇数节点（复制自己）
			}

			combined := append(left.Hash, right.Hash...)
			parentHash := sha256.Sum256(combined)

			parent := &MerkleNode{
				Hash:  parentHash[:],
				Left:  left,
				Right: right,
			}
			left.Parent = parent
			right.Parent = parent
			newLevel = append(newLevel, parent)
		}
		nodes = newLevel
	}

	return nodes[0], nil
}

func BuildMerkleTreeFromFileRW(file io.ReadWriter, config *MerkleConfig) (*MerkleNode, error) {
	if file == nil {
		return nil, fmt.Errorf("file is nil")
	}
	if config.BlockSize <= 0 {
		return nil, fmt.Errorf("invalid block size")
	}

	leaves, err := ReadFileToBuffers(file, config.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create buffer channel: %w", err)
	}

	root, err := buildMerkleTreeFromLeafHashes(leaves)
	if err != nil {
		return nil, fmt.Errorf("failed to build Merkle tree: %w", err)
	}

	return root, nil
}
