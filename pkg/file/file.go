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
