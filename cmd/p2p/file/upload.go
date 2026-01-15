// Package file provides file upload command with dual Merkle tree support
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/sirupsen/logrus"
	"p2pFileTransfer/pkg/chameleonMerkleTree"
	"p2pFileTransfer/pkg/file"
	"p2pFileTransfer/pkg/p2p"
)

var (
	treeType     string // chameleon | regular
	description  string
	metadataPath string
	chunkSize    uint
	showProgress bool
)

// uploadCmd represents the file upload command
var uploadCmd = &cobra.Command{
	Use:   "upload <file>",
	Short: "Upload a file to the P2P network",
	Long: `Upload a file to the P2P network.

Supports two Merkle tree types:
  - chameleon (default): Editable tree using elliptic curve P256.
    Requires key generation and management. Suitable for files
    that may need to be modified later.

  - regular: Standard immutable SHA256 Merkle tree.
    Simpler and faster. Suitable for one-time uploads.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		return uploadFile(context.Background(), filePath)
	},
}

func init() {
	FileCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringVarP(&treeType, "tree-type", "t", "chameleon",
		"Merkle tree type: chameleon (editable) | regular (standard)")
	uploadCmd.Flags().StringVarP(&description, "description", "d", "", "File description")
	uploadCmd.Flags().StringVarP(&metadataPath, "output", "o", "", "Metadata output path")
	uploadCmd.Flags().UintVar(&chunkSize, "chunk-size", 262144, "Chunk size in bytes")
	uploadCmd.Flags().BoolVarP(&showProgress, "progress", "p", false, "Show progress bar")
}

func uploadFile(ctx context.Context, filePath string) error {
	// Validate treeType parameter
	if treeType != "chameleon" && treeType != "regular" {
		return fmt.Errorf("invalid tree-type: %s (must be 'chameleon' or 'regular')", treeType)
	}

	// Route to appropriate upload function based on tree type
	if treeType == "chameleon" {
		return uploadFileChameleon(ctx, filePath)
	}
	return uploadFileRegular(ctx, filePath)
}

// uploadFileChameleon uploads a file using Chameleon Merkle Tree
func uploadFileChameleon(ctx context.Context, filePath string) error {
	fmt.Printf("Uploading with Chameleon Merkle Tree (editable)...\n")

	// 1. Open file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// 2. Get file info
	fileInfo, _ := f.Stat()
	fileName := filepath.Base(filePath)
	fileSize := fileInfo.Size()

	// 3. Generate key pair using the proper API
	privKey, pubKey := chameleonMerkleTree.NewChameleonKeyPair()

	// 4. Read file and compute SHA256 hashes for each chunk
	chunks, err := p2p.CalculateChunkHashes(f, chunkSize)
	if err != nil {
		return fmt.Errorf("failed to calculate chunk hashes: %w", err)
	}

	// 5. Build Chameleon Merkle Tree from pre-computed hashes
	// Extract hash arrays
	hashData := make([][]byte, len(chunks))
	for i, chunk := range chunks {
		hashData[i] = chunk.Hash
	}

	// Use the new package function to build Chameleon Merkle Tree from hashes
	cmt, err := chameleonMerkleTree.NewChameleonMerkleTreeFromHashes(hashData, pubKey)
	if err != nil {
		return fmt.Errorf("failed to build Chameleon Merkle tree: %w", err)
	}

	// 5. Get root hash (CID) using Chameleon hash
	cid := cmt.GetChameleonHash()
	fmt.Printf("File CID: %x\n", cid)

	// 6. Create P2P service
	logrus.Info("Creating P2P service...")
	p2pConfig := p2p.NewP2PConfig()
	service, err := p2p.NewP2PService(ctx, p2pConfig)
	if err != nil {
		return fmt.Errorf("failed to create P2P service: %w", err)
	}
	defer service.Shutdown()

	// 7. Upload chunks to storage directory and Announce
	chunkPath := p2pConfig.ChunkStoragePath
	logrus.Infof("Uploading %d chunks to %s...", len(chunks), chunkPath)

	// Ensure chunk storage directory exists
	if err := os.MkdirAll(chunkPath, 0755); err != nil {
		return fmt.Errorf("failed to create chunk storage directory: %w", err)
	}

	for i, chunk := range chunks {
		logrus.Debugf("Chunk %d hash length: %d bytes", i, len(chunk.Hash))

		// Write chunk to storage
		chunkFile := filepath.Join(chunkPath, fmt.Sprintf("%x", chunk.Hash))
		logrus.Debugf("Writing chunk %d to %s (path length: %d)", i, chunkFile, len(chunkFile))
		if err := os.WriteFile(chunkFile, chunk.Data, 0644); err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", i, err)
		}

		if showProgress {
			fmt.Printf("[%d/%d] Uploaded chunk\n", i+1, len(chunks))
		}

		// Announce chunk to DHT
		chunkHashStr := fmt.Sprintf("%x", chunk.Hash)
		if err := service.Announce(ctx, chunkHashStr); err != nil {
			logrus.Warnf("Failed to announce chunk %d: %v", i, err)
		}
	}

	// 9. Get random number from Chameleon Merkle Tree
	randomNum := cmt.GetRandomNumber()

	// 10. Serialize public key and random number for MetaData
	publicKeySerialized := pubKey.Serialize()
	randomNumSerialized := randomNum.Serialize()

	// Build leaves metadata from the chunks we already have
	leaves := convertChunksToChunkData(chunks)

	metadata := &file.MetaData{
		RootHash:    cid,
		RandomNum:   randomNumSerialized,
		PublicKey:   publicKeySerialized,
		Description: description,
		FileSize:    uint64(fileSize),
		FileName:    fileName,
		Encryption:  "none",
		TreeType:    "chameleon",
		Leaves:      leaves,
	}

	// 11. Save MetaData and private key
	if err := saveMetadataAndKey(metadata, privKey, cid); err != nil {
		return err
	}

	printUploadSummary(fileName, fileSize, len(chunks), cid, "chameleon")
	return nil
}

// uploadFileRegular uploads a file using Regular Merkle Tree
func uploadFileRegular(ctx context.Context, filePath string) error {
	fmt.Printf("Uploading with Regular Merkle Tree (standard)...\n")

	// 1. Open file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// 2. Get file info
	fileInfo, _ := f.Stat()
	fileName := filepath.Base(filePath)
	fileSize := fileInfo.Size()

	// 3. Read file and calculate all chunk hashes
	chunks, err := p2p.CalculateChunkHashes(f, chunkSize)
	if err != nil {
		return fmt.Errorf("failed to calculate chunk hashes: %w", err)
	}

	// 4. Build Merkle Tree and calculate root hash
	cid := p2p.BuildMerkleRoot(chunks)
	fmt.Printf("File CID: %x\n", cid)

	// 5. Create P2P service
	logrus.Info("Creating P2P service...")
	p2pConfig := p2p.NewP2PConfig()
	service, err := p2p.NewP2PService(ctx, p2pConfig)
	if err != nil {
		return fmt.Errorf("failed to create P2P service: %w", err)
	}
	defer service.Shutdown()

	// 6. Upload chunks to storage directory and Announce
	chunkPath := p2pConfig.ChunkStoragePath
	logrus.Infof("Uploading %d chunks to %s...", len(chunks), chunkPath)

	for i, chunk := range chunks {
		chunkFile := filepath.Join(chunkPath, fmt.Sprintf("%x", chunk.Hash))
		if err := os.WriteFile(chunkFile, chunk.Data, 0644); err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", i, err)
		}

		if showProgress {
			fmt.Printf("[%d/%d] Uploaded chunk\n", i+1, len(chunks))
		}

		chunkHashStr := fmt.Sprintf("%x", chunk.Hash)
		if err := service.Announce(ctx, chunkHashStr); err != nil {
			logrus.Warnf("Failed to announce chunk %d: %v", i, err)
		}
	}

	// 7. Generate MetaData (no key info needed)
	metadata := &file.MetaData{
		RootHash:    cid,
		PublicKey:   nil, // Regular mode doesn't need public key
		RandomNum:   nil, // Regular mode doesn't need random number
		Description: description,
		FileSize:    uint64(fileSize),
		FileName:    fileName,
		Encryption:  "none",
		TreeType:    "regular",
		Leaves:      convertChunksToChunkData(chunks),
	}

	// 8. Save MetaData (no private key needed)
	if err := saveMetadata(metadata, cid); err != nil {
		return err
	}

	printUploadSummary(fileName, fileSize, len(chunks), cid, "regular")
	return nil
}

// Helper functions

func convertChunksToChunkData(chunks []p2p.Chunk) []file.ChunkData {
	result := make([]file.ChunkData, len(chunks))
	for i, chunk := range chunks {
		result[i] = file.ChunkData{
			ChunkSize: len(chunk.Data),
			ChunkHash: chunk.Hash,
		}
	}
	return result
}

func saveMetadataAndKey(metadata *file.MetaData, privKey []byte, cid []byte) error {
	outputDir := metadataPath
	if outputDir == "" {
		outputDir = "./metadata"
	}

	// Ensure directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Save MetaData
	metadataFile := filepath.Join(outputDir, fmt.Sprintf("%x.json", cid))
	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(metadataFile, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Save private key (only for Chameleon mode)
	keyFile := filepath.Join(outputDir, fmt.Sprintf("%x.key", cid))
	keyData, _ := json.MarshalIndent(map[string]interface{}{"privateKey": privKey}, "", "  ")
	if err := os.WriteFile(keyFile, keyData, 0600); err != nil {
		return fmt.Errorf("failed to save private key: %w", err)
	}

	fmt.Printf("\nMetadata saved to: %s\n", metadataFile)
	fmt.Printf("Private key saved to: %s\n", keyFile)
	fmt.Printf("\n⚠️  Important: Keep the private key file safe!\n")
	fmt.Printf("    It's required to edit this file later.\n")
	return nil
}

func saveMetadata(metadata *file.MetaData, cid []byte) error {
	outputDir := metadataPath
	if outputDir == "" {
		outputDir = "./metadata"
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	metadataFile := filepath.Join(outputDir, fmt.Sprintf("%x.json", cid))
	metadataJSON, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(metadataFile, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	fmt.Printf("\nMetadata saved to: %s\n", metadataFile)
	return nil
}

func printUploadSummary(fileName string, fileSize int64, chunkCount int, cid []byte, treeType string) {
	fmt.Printf("\n✓ Upload complete!\n")
	fmt.Printf("  File: %s\n", fileName)
	fmt.Printf("  Size: %d bytes\n", fileSize)
	fmt.Printf("  Chunks: %d\n", chunkCount)
	fmt.Printf("  Tree Type: %s\n", treeType)
	fmt.Printf("  CID: %x\n", cid)
}
