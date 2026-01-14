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
	RootHash    []byte      `json:"rootHash"`
	RandomNum   []byte      `json:"randomNum"`
	PublicKey   []byte      `json:"publicKey"`
	Description string      `json:"description"`
	FileSize    uint64      `json:"fileSize"`
	FileName    string      `json:"fileName"`
	Encryption  string      `json:"encryption"`
	Leaves      []ChunkData `json:"leaves"`
}

type ChunkData struct {
	ChunkSize int    `json:"chunkSize"`
	ChunkHash []byte `json:"chunkHash"`
}
