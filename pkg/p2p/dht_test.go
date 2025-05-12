package p2p

import (
	"context"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPut(t *testing.T) {
	config1 := NewP2PConfig()
	config1.Port = 0
	p1, err := NewP2PService(context.Background(), config1)
	if err != nil {
		t.Fatalf("Failed to create P2P service: %v", err)
	}

	p1.Put(context.Background(), "hello", []byte("world"))
	<-context.Background().Done()
}

func TestGet(t *testing.T) {
	config2 := NewP2PConfig()
	maddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/62305/p2p/QmYXt4nUjUEV2qihnVsN38iuMHnKvAKKrXhVCN1KLz7beH")
	if err != nil {
		t.Fatalf("Failed to create multiaddr: %v", err)
	}

	config2.BootstrapPeers = append(config2.BootstrapPeers, maddr)
	config2.Port = 10001

	p2, err := NewP2PService(context.Background(), config2)

	s1, err := p2.Get(context.Background(), "hello")
	require.NoError(t, err)
	if string(s1) != "world" {
		t.Fatalf("Expected 'world', got '%s'", string(s1))
	}
}

func TestAnnounce(t *testing.T) {
	config1 := NewP2PConfig()
	p1, err := NewP2PService(context.Background(), config1)
	require.NoError(t, err)
	p1.Announce(context.Background(), "123456")

	<-context.Background().Done()
}

func TestLookUp(t *testing.T) {
	config2 := NewP2PConfig()
	maddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/51871/p2p/QmesqkFZtpDAN61Doixfk4whEaxHTGnCp2dE397wBfHf1n")
	require.NoError(t, err)
	config2.BootstrapPeers = append(config2.BootstrapPeers, maddr)

	p2, err := NewP2PService(context.Background(), config2)

	s1, err := p2.Lookup(context.Background(), "123456")
	require.NoError(t, err)
	for _, v := range s1 {
		t.Logf("Lookup result: %s", v)
	}
}
