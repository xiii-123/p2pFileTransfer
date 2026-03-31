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

import (
	"encoding/hex"
	"encoding/json"
)

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

// MarshalJSON 自定义 JSON 序列化，使用 hex 编码而不是 base64
func (m MetaData) MarshalJSON() ([]byte, error) {
	type Alias MetaData
	return json.Marshal(&struct {
		RootHash        string `json:"rootHash"`
		RegularRootHash string `json:"regularRootHash,omitempty"`
		RandomNum       string `json:"randomNum,omitempty"`
		PublicKey       string `json:"publicKey,omitempty"`
		*Alias
	}{
		RootHash:        hex.EncodeToString(m.RootHash),
		RegularRootHash: hex.EncodeToString(m.RegularRootHash),
		RandomNum:       hex.EncodeToString(m.RandomNum),
		PublicKey:       hex.EncodeToString(m.PublicKey),
		Alias:           (*Alias)(&m),
	})
}

// UnmarshalJSON 自定义 JSON 反序列化，从 hex 解码
func (m *MetaData) UnmarshalJSON(data []byte) error {
	type Alias MetaData
	aux := &struct {
		RootHash        string `json:"rootHash"`
		RegularRootHash string `json:"regularRootHash,omitempty"`
		RandomNum       string `json:"randomNum,omitempty"`
		PublicKey       string `json:"publicKey,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	// 解码 hex 字符串
	if aux.RootHash != "" {
		hash, err := hex.DecodeString(aux.RootHash)
		if err != nil {
			return err
		}
		m.RootHash = hash
	}
	if aux.RegularRootHash != "" {
		hash, err := hex.DecodeString(aux.RegularRootHash)
		if err != nil {
			return err
		}
		m.RegularRootHash = hash
	}
	if aux.RandomNum != "" {
		num, err := hex.DecodeString(aux.RandomNum)
		if err != nil {
			return err
		}
		m.RandomNum = num
	}
	if aux.PublicKey != "" {
		key, err := hex.DecodeString(aux.PublicKey)
		if err != nil {
			return err
		}
		m.PublicKey = key
	}
	return nil
}

type ChunkData struct {
	Index     int    `json:"index"`      // Chunk 索引（确保顺序）
	ChunkSize int    `json:"chunkSize"`
	ChunkHash []byte `json:"chunkHash"`
}

// MarshalJSON 自定义 JSON 序列化，使用 hex 编码而不是 base64
func (c ChunkData) MarshalJSON() ([]byte, error) {
	type Alias ChunkData
	return json.Marshal(&struct {
		ChunkHash string `json:"chunkHash"`
		*Alias
	}{
		ChunkHash: hex.EncodeToString(c.ChunkHash),
		Alias:     (*Alias)(&c),
	})
}

// UnmarshalJSON 自定义 JSON 反序列化，从 hex 解码
func (c *ChunkData) UnmarshalJSON(data []byte) error {
	type Alias ChunkData
	aux := &struct {
		ChunkHash string `json:"chunkHash"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	hash, err := hex.DecodeString(aux.ChunkHash)
	if err != nil {
		return err
	}
	c.ChunkHash = hash
	return nil
}
