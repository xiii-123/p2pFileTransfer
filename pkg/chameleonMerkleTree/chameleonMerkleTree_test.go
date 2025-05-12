package chameleonMerkleTree

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

const DefaultFilePath = "C:\\Users\\admin\\Desktop\\hello.txt"

func TestSerializeAndDeserialize(t *testing.T) {
	// 生成公私钥对
	_, pubKey := NewChameleonKeyPair()

	// 序列化公钥
	serializedPubKey := pubKey.Serialize()
	fmt.Printf("Serialized public key: %x\n", serializedPubKey)

	// 反序列化公钥
	deserializedPubKey, err := DeserializeChameleonPubKey(serializedPubKey)
	if err != nil {
		fmt.Printf("Error deserializing public key: %v\n", err)
		return
	}
	fmt.Printf("Deserialized public key: %x\n", deserializedPubKey.Serialize())

	// 验证反序列化是否成功
	if !bytes.Equal(pubKey.Serialize(), deserializedPubKey.Serialize()) {
		t.Errorf("Deserialized public key does not match original")
	}
}

func getDefaultTree(pubKey *ChameleonPubKey) *ChameleonMerkleNode {
	config := NewMerkleConfig()
	config.BlockSize = 4 * 1024
	file, err := os.Open(DefaultFilePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	tree, err := NewChameleonMerkleTree(file, config, pubKey)
	if err != nil {
		fmt.Println("Error building Merkle tree:", err)
		return nil
	}
	return tree
}

func TestBuildMerkleTreeFromFile(t *testing.T) {
	_, pubKey := NewChameleonKeyPair()
	tree := getDefaultTree(pubKey)
	fmt.Println("Root hash: ", tree.GetRootHash())
	if !tree.VerifyChameleonHash() {
		t.Errorf("Chameleon hash verification failed")
	}
}

func TestSerializeAndDeserializeTree(t *testing.T) {
	_, pubKey := NewChameleonKeyPair()
	tree := getDefaultTree(pubKey)
	// 序列化树
	serializedTree, err := tree.Serialize()
	if err != nil {
		t.Fatalf("Error serializing tree: %v", err)
	}
	// 反序列化树
	deserializedTree, err := DeserializeChameleonMerkleTree(serializedTree)
	if err != nil {
		t.Fatalf("Error deserializing tree: %v", err)
	}
	// 验证反序列化是否成功
	if !bytes.Equal(tree.GetRootHash(), deserializedTree.GetRootHash()) &&
		!bytes.Equal(tree.GetPublicKey().Serialize(), deserializedTree.GetPublicKey().Serialize()) &&
		!bytes.Equal(tree.GetRandomNumber().Serialize(), deserializedTree.GetRandomNumber().Serialize()) &&
		!bytes.Equal(tree.GetChameleonHash(), deserializedTree.GetChameleonHash()) {
		t.Errorf("Deserialized tree does not match original")
	}
}

func TestRebuildChameleonMerkleTree(t *testing.T) {
	secKey, pubKey := NewChameleonKeyPair()
	config := NewDefaultMerkleConfig()
	tree := getDefaultTree(pubKey)

	newFile, err := os.Open("C:\\Users\\admin\\Desktop\\hello.txt")
	if err != nil {
		t.Error("Error opening file:", err)
		return
	}
	newTree, err := UpdateChameleonMerkleTree(newFile, config, secKey, tree.GetRootHash(), tree.GetChameleonHash(), tree.GetRandomNumber(), tree.GetPublicKey())
	if err != nil {
		t.Error("Error updating Merkle tree:", err)
		return
	}
	fmt.Printf("New root hash: %x\n", newTree.GetRootHash())
	if !newTree.VerifyChameleonHash() {
		t.Errorf("Chameleon hash verification failed")
	}
}

func TestRebuildChameleonPubKey(t *testing.T) {
	secKey, pubKey := NewChameleonKeyPair()
	rePublicKey, err := RebuildChameleonPubKey(secKey)
	if err != nil {
		t.Error("Error rebuilding public key:", err)
		return
	}
	if !bytes.Equal(pubKey.Serialize(), rePublicKey.Serialize()) {
		t.Errorf("Rebuilt public key does not match original")
	}
}
