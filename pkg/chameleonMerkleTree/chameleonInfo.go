package chameleonMerkleTree

import (
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateChameleonKeyPair 生成Chameleon哈希的公私钥对
func NewChameleonKeyPair() ([]byte, *ChameleonPubKey) {
	secKey, pubX, pubY, _ := elliptic.GenerateKey(GetCurve(), rand.Reader)
	return secKey, &ChameleonPubKey{
		pubX: pubX,
		pubY: pubY,
	}
}

func RebuildChameleonPubKey(secKey []byte) (*ChameleonPubKey, error) {
	if len(secKey) != 32 {
		return nil, fmt.Errorf("invalid private key length")
	}
	x, y := GetCurve().ScalarBaseMult(secKey)
	return &ChameleonPubKey{
		pubX: x,
		pubY: y,
	}, nil
}

func (pubKey *ChameleonPubKey) Serialize() []byte {
	return append(pubKey.pubX.Bytes(), pubKey.pubY.Bytes()...)
}

func DeserializeChameleonPubKey(data []byte) (*ChameleonPubKey, error) {
	if len(data) != 64 {
		return nil, fmt.Errorf("invalid data length: expected 64 bytes, got %d", len(data))
	}
	pubXBytes := data[:32]
	pubYBytes := data[32:]
	return &ChameleonPubKey{
		pubX: new(big.Int).SetBytes(pubXBytes),
		pubY: new(big.Int).SetBytes(pubYBytes),
	}, nil
}

func (randomNum *ChameleonRandomNum) Serialize() []byte {
	return append(append(randomNum.rX.Bytes(), randomNum.rY.Bytes()...), randomNum.s.Bytes()...)
}

func DeserializeChameleonRandomNum(data []byte) (*ChameleonRandomNum, error) {
	if len(data) != 96 {
		return nil, fmt.Errorf("invalid data length: expected 96 bytes, got %d", len(data))
	}
	rXBytes := data[:32]
	rYBytes := data[32:64]
	sBytes := data[64:]
	return &ChameleonRandomNum{
		rX: new(big.Int).SetBytes(rXBytes),
		rY: new(big.Int).SetBytes(rYBytes),
		s:  new(big.Int).SetBytes(sBytes),
	}, nil
}
