package op

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	libp2p_swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// 获取密钥(没有时生成, 存在时加载)
func getPrivateKey(privateKeyPath string) (*crypto.PrivKey, error) {
	var privateKey crypto.PrivKey
	var privateKeyBytes []byte
	_, e := os.Stat(privateKeyPath)
	if os.IsNotExist(e) {
		privateKey, _, e = crypto.GenerateKeyPair(
			crypto.Ed25519, // Select your key type. Ed25519 are nice short
			-1,             // Select key length when possible (i.e. RSA).
		)
		if e != nil {
			return nil, e
		}
		privateKeyBytes, e = crypto.MarshalPrivateKey(privateKey)
		if e != nil {
			return nil, e
		}
		e = ioutil.WriteFile(privateKeyPath, privateKeyBytes, os.ModePerm)
		if e != nil {
			return nil, e
		}
	} else {
		privateKeyBytes, e = ioutil.ReadFile(privateKeyPath)
		if e != nil {
			return nil, e
		}
		privateKey, e = crypto.UnmarshalPrivateKey(privateKeyBytes)
		if e != nil {
			return nil, e
		}
	}
	return &privateKey, nil
}

// 清除节点网络缓存
// 防止拨号器使用无法连接地址快速重拨导致一直连不上.
// https://github.com/prysmaticlabs/prysm/issues/2674#issuecomment-529229685
func clearPeerNetworkCache(h host.Host, id peer.ID) {
	h.Peerstore().ClearAddrs(id)
	h.Network().(*libp2p_swarm.Swarm).Backoff().Clear(id)
}

// 保护节点连接防止被清理
func protectPeerConn(h host.Host, id peer.ID) {
	h.ConnManager().TagPeer(id, connProtectTag, 100)
	h.ConnManager().Protect(id, connProtectTag)
}

// 多址字符转节点地址
func multiaddrToAddrInfo(multiaddrText string) (*peer.AddrInfo, error) {
	multiAddr, e := multiaddr.NewMultiaddr(multiaddrText)
	if e != nil {
		return nil, fmt.Errorf("多址字符错误: %w", e)
	}

	addrInfo, e := peer.AddrInfoFromP2pAddr(multiAddr)
	if e != nil {
		return nil, fmt.Errorf("多址字符转节点地址出错: %w", e)
	}

	// 在节点地址中添加中继多址
	relayMultiAddr, e := multiaddr.NewMultiaddr(fmt.Sprint("/p2p-circuit/ipfs/", addrInfo.ID.Pretty()))
	if e != nil {
		return nil, fmt.Errorf("创建中继多址出错: %w", e)
	}
	addrInfo.Addrs = append(addrInfo.Addrs, relayMultiAddr)

	return addrInfo, nil
}

// 连接引导(帮助节点之间进行发现)
func connectBootstrap(gc context.Context, h host.Host, multiaddrText string) {
	log.Println("连接引导地址", multiaddrText)
	addrInfo, e := multiaddrToAddrInfo(multiaddrText)
	if e != nil {
		log.Println("连接引导失败", multiaddrText, e)
		return
	}

	localContext, localContextCancel := context.WithTimeout(gc, time.Second*3)
	defer localContextCancel()
	e = h.Connect(localContext, *addrInfo)
	if e != nil {
		log.Println("连接引导失败", multiaddrText, e)
		clearPeerNetworkCache(h, addrInfo.ID)
		return
	}
	log.Println("连接引导成功", multiaddrText)
	protectPeerConn(h, addrInfo.ID)
}
