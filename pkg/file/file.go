// Package file 提供文件元数据结构和类型定义
//
// 主要类型:
//   - MetaData: 文件元数据，包含根哈希、随机数、公钥等信息
//   - ChunkData: Chunk 数据，包含大小和哈希
//
// 使用场景:
//   - 文件标识: 使用根哈希和随机数唯一标识文件
//   - 完整性验证: 使用公钥和哈希验证文件完整性
//   - 分块信息: 存储文件的所有 Chunk 哈希列表
//
// 注意事项:
//   - 此包仅定义数据结构，不包含处理逻辑
//   - MetaData 应与 Chameleon Merkle Tree 配合使用
package file

type MetaData struct {
	RootHash        []byte      `json:"rootHash"`                   // 变色龙哈希（CID）或常规Merkle根哈希
	RegularRootHash []byte      `json:"regularRootHash,omitempty"`  // 常规Merkle根哈希（仅chameleon模式需要）
	RandomNum       []byte      `json:"randomNum,omitempty"`        // 随机数（仅chameleon模式）
	PublicKey       []byte      `json:"publicKey,omitempty"`        // 公钥（仅chameleon模式）
	Description     string      `json:"description,omitempty"`      // 文件描述
	FileSize        uint64      `json:"fileSize"`                   // 文件大小（字节）
	FileName        string      `json:"fileName"`                   // 文件名
	Encryption      string      `json:"encryption,omitempty"`       // 加密方式
	TreeType        string      `json:"treeType"`                   // Merkle树类型: "chameleon" | "regular"
	Leaves          []ChunkData `json:"leaves"`                     // 所有chunk的哈希列表
}

type ChunkData struct {
	ChunkSize int    `json:"chunkSize"`
	ChunkHash []byte `json:"chunkHash"`
}
