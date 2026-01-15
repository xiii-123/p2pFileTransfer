// Package p2p provides Regular Merkle Tree implementation for standard file hashing
package p2p

import (
	"crypto/sha256"
	"io"
	"os"
)

// Chunk represents a file chunk with its hash and data
type Chunk struct {
	Hash []byte
	Data []byte
}

// CalculateChunkHashes reads a file and calculates SHA256 hash for each chunk
func CalculateChunkHashes(file *os.File, chunkSize uint) ([]Chunk, error) {
	var chunks []Chunk
	buffer := make([]byte, chunkSize)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		data := make([]byte, n)
		copy(data, buffer[:n])

		hash := sha256.Sum256(data)
		chunks = append(chunks, Chunk{
			Hash: hash[:],
			Data: data,
		})
	}

	// Reset file pointer to beginning
	_, err := file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	return chunks, nil
}

// BuildMerkleRoot builds a Merkle tree from chunk hashes and returns the root hash
func BuildMerkleRoot(chunks []Chunk) []byte {
	if len(chunks) == 0 {
		return nil
	}

	// Extract all hashes
	hashes := make([][]byte, len(chunks))
	for i, chunk := range chunks {
		hashes[i] = chunk.Hash
	}

	// Build Merkle tree bottom-up
	for len(hashes) > 1 {
		var newLevel [][]byte

		// Pair up hashes and calculate parent hash
		for i := 0; i < len(hashes); i += 2 {
			if i+1 == len(hashes) {
				// Odd number of nodes, duplicate the last one
				newLevel = append(newLevel, hashes[i])
			} else {
				// Calculate parent hash
				h := sha256.New()
				h.Write(hashes[i])
				h.Write(hashes[i+1])
				newLevel = append(newLevel, h.Sum(nil))
			}
		}

		hashes = newLevel
	}

	return hashes[0]
}
