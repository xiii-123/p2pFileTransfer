package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p/core/peer"
)

type AntiLeecher interface {
	Refuse(ctx context.Context, peerID peer.ID) bool
}

type DefaultAntiLeecher struct{}

func (d *DefaultAntiLeecher) Refuse(ctx context.Context, peerID peer.ID) bool {
	return false
}
