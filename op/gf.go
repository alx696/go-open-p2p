package op

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	libp2p_swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/multiformats/go-multiaddr"
	"io/ioutil"
	"log"
	"os"
	"strings"
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

// 清除节点网络缓存
//
// 防止拨号器使用无法连接地址快速重拨导致一直连不上.
//
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

// 连接节点
func connectPeer(gc context.Context, h host.Host, addr peer.AddrInfo, timeout time.Duration) error {
	ctx, ctxCancel := context.WithTimeout(gc, timeout)
	defer ctxCancel()
	e := h.Connect(ctx, addr)
	if e != nil {
		//log.Println("连接失败", addr.ID.Pretty(), e)
		clearPeerNetworkCache(h, addr.ID)
		return e
	}
	//log.Println("连接成功", addr.ID.Pretty())
	protectPeerConn(h, addr.ID)

	return nil
}

// 连接引导(帮助节点之间进行发现)
func connectBootstrap(gc context.Context, h host.Host, multiaddrText string) {
	addrInfo, e := multiaddrToAddrInfo(multiaddrText)
	if e != nil {
		log.Println("引导地址错误", multiaddrText, e)
		return
	}

	e = connectPeer(gc, h, *addrInfo, time.Second*3)
	if e != nil {
		log.Println("连接引导失败", multiaddrText, e)
		return
	}

	log.Println("连接引导成功", multiaddrText)
}

// 从DHT中查找节点地址信息
func findAddrInfoFromDHT(gc context.Context, id peer.ID) (*peer.AddrInfo, error) {
	localContext, localContextCancel := context.WithTimeout(gc, time.Second)
	defer localContextCancel()
	addrInfo, e := globalDHT.FindPeer(localContext, id)
	if e != nil {
		return nil, e
	}

	// 地址信息中添加中继地址, 支持没有公网IP的用户
	//参考 https://github.com/libp2p/go-libp2p-examples/blob/master/relay/main.go
	relayMultiAddr, e := multiaddr.NewMultiaddr(fmt.Sprint("/p2p-circuit/ipfs/", id.Pretty()))
	if e != nil {
		return nil, e
	}
	addrInfo.Addrs = append(addrInfo.Addrs, relayMultiAddr)

	return &addrInfo, nil
}

// 连接数量
func connectCount(h host.Host, id peer.ID) int {
	return len(h.Network().ConnsToPeer(id))
}

// 创建节点的流
//
// 注意: defer s.Close()
func createStream(gc context.Context, h host.Host, id string, protocolID protocol.ID) (network.Stream, error) {
	peerID, _ := peer.Decode(id)
	lc, lcCancel := context.WithTimeout(gc, time.Second*3)
	defer lcCancel()
	return h.NewStream(lc, peerID, protocolID)
}

// 从读写器中获取文本
func readTextFromReadWriter(rw *bufio.ReadWriter) (*[]byte, error) {
	//读取
	txt, e := rw.ReadString('\n')
	if e != nil {
		return nil, e
	}
	//移除delim
	txt = strings.TrimSuffix(txt, "\n")

	if txt == "" {
		var empty []byte
		return &empty, nil
	}

	// //读取
	// encodeData, e := rw.ReadBytes('\n')
	// if e != nil {
	// 	return nil, e
	// }
	// //移除delim
	// encodeData = encodeData[0 : len(encodeData)-1]

	//解码
	data, e := base64.StdEncoding.DecodeString(txt)
	if e != nil {
		return nil, e
	}

	return &data, nil
}

// 往读写器中写入文本
func writeTextToReadWriter(rw *bufio.ReadWriter, data *[]byte) error {
	//编码
	encodeData := []byte(base64.StdEncoding.EncodeToString(*data))

	//添加delim
	encodeData = append(encodeData, '\n')

	//写入
	_, e := rw.Write(encodeData)
	if e != nil {
		return e
	}
	e = rw.Flush()
	if e != nil {
		return e
	}
	return nil
}
