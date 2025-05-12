package p2p

import (
	"context"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChunk(t *testing.T) {
	config1 := NewP2PConfig()
	_, err := NewP2PService(context.Background(), config1)
	require.NoError(t, err)

	<-context.Background().Done()
}

func TestChunkExit(t *testing.T) {
	config2 := NewP2PConfig()
	maddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/55816/p2p/QmaZ4tf3R7aHJtsfgdTSQhBmhCyuBuvAbRoxWaD9HsQhfi")
	require.NoError(t, err)
	config2.BootstrapPeers = append(config2.BootstrapPeers, maddr)

	p2, err := NewP2PService(context.Background(), config2)

	peers, _ := p2.DHT.GetClosestPeers(context.Background(), "hello.txt")
	res, err := p2.CheckChunkExists(context.Background(), peers[0], "hello.txt")
	require.NoError(t, err)
	if res {
		t.Log("File exists")
	}
}

func TestChunkGet(t *testing.T) {
	config2 := NewP2PConfig()
	maddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/55816/p2p/QmaZ4tf3R7aHJtsfgdTSQhBmhCyuBuvAbRoxWaD9HsQhfi")
	require.NoError(t, err)
	config2.BootstrapPeers = append(config2.BootstrapPeers, maddr)

	p2, err := NewP2PService(context.Background(), config2)

	peers, _ := p2.DHT.GetClosestPeers(context.Background(), "hello.txt")
	res, err := p2.DownloadChunk(context.Background(), peers[0], "hello.txt")
	require.NoError(t, err)
	t.Logf("File content:\n %s", string(res))
}
