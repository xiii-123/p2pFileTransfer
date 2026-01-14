// Package p2p 提供工具函数
//
// Utils 功能:
//   - 主机创建: 创建 libp2p 主机实例
//   - 地址生成: 生成监听地址
//   - 密钥对生成: RSA 2048 位密钥对
//
// 主要函数:
//   - newBasicHost: 创建基本 libp2p 主机
//   - GetHostAddress: 获取主机地址字符串
//
// 安全选项:
//   - 加密连接: 默认使用加密连接
//   - 不安全模式: 可选禁用加密（仅用于测试）
//
// 使用示例:
//
//	host, err := newBasicHost(0, false, 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer host.Close()
//
//	addr := GetHostAddress(host)
//	fmt.Printf("Listening on: %s\n", addr)
//
// 注意事项:
//   - 生产环境应始终使用加密连接
//   - 不安全模式仅用于开发和测试
//   - RSA 2048 提供足够的安全性
package p2p

import (
	"crypto/rand"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
	"io"
	mrand "math/rand"
)

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given multiaddress. It won't encrypt the connection if insecure is true.
func newBasicHost(listenPort int, insecure bool, randseed int64) (host.Host, error) {
	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}

	// Generate a key pair for this host. We will use it at least
	// to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.Identity(priv),
		//libp2p.DisableRelay(),
	}

	//if insecure {
	//	opts = append(opts, libp2p.NoSecurity)
	//} else {
	//	opts = append(opts, libp2p.Security(noise.ID, noise.New))
	//}

	return libp2p.New(opts...)
}

func GetHostAddress(host host.Host) string {
	// Build host multiaddress
	hostAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", host.ID()))
	if err != nil {
		logrus.Errorf("Failed to create host multiaddress: %v", err)
		return ""
	}

	// Now we can build a full multiaddress to reach this host
	// by encapsulating both addresses:
	addrs := host.Addrs()
	if len(addrs) == 0 {
		logrus.Error("Host has no addresses")
		return ""
	}

	addr := addrs[0]
	return addr.Encapsulate(hostAddr).String()
}
