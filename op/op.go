package op

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/libp2p/go-libp2p"
	libp2p_conn "github.com/libp2p/go-libp2p-connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	libp2p_dht "github.com/libp2p/go-libp2p-kad-dht"
	libp2p_noise "github.com/libp2p/go-libp2p-noise"
	libp2p_quic "github.com/libp2p/go-libp2p-quic-transport"
	libp2p_tls "github.com/libp2p/go-libp2p-tls"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Callback interface {
	OnOpStart(id string, addrArray string)
	OnOpStop()
}

var globalCallback Callback
var globalName string
var globalContext context.Context
var globalContextCancel context.CancelFunc
var globalHost host.Host
var globalDHT *libp2p_dht.IpfsDHT
var mdnsStopChan = make(chan int, 1)

func Start(privateDirArg string, publicDirArg string, nameArg string, callbackArg Callback) error {
	log.Println("启动开放点对点")
	log.Println("私有文件夹", privateDirArg)
	log.Println("公共文件夹", publicDirArg)
	log.Println("我的名字", nameArg)
	globalName = nameArg
	globalCallback = callbackArg

	e := os.MkdirAll(privateDirArg, os.ModePerm)
	if e != nil {
		return fmt.Errorf("%w\n创建私有文件夹出错", e)
	}
	e = os.MkdirAll(publicDirArg, os.ModePerm)
	if e != nil {
		return fmt.Errorf("%w\n创建公共文件夹出错", e)
	}

	// 获取密钥
	myKey, e := getPrivateKey(filepath.Join(privateDirArg, "my.key"))
	if e != nil {
		return fmt.Errorf("%w\n获取密钥出错", e)
	}

	// 创建全局上下文
	globalContext, globalContextCancel = context.WithCancel(context.Background())

	// 创建主机
	port := 0
	globalHost, e = libp2p.New(
		globalContext,
		// Use the keypair we generated
		libp2p.Identity(*myKey),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			fmt.Sprint("/ip4/0.0.0.0/tcp/", port),          // regular tcp connections
			fmt.Sprint("/ip4/0.0.0.0/udp/", port, "/quic"), // a UDP endpoint for the QUIC transport
			fmt.Sprint("/ip6/::/udp/", port, "/quic"),      // a UDP endpoint for the QUIC transport
		),
		// support TLS connections
		libp2p.Security(libp2p_tls.ID, libp2p_tls.New),
		// support noise connections
		libp2p.Security(libp2p_noise.ID, libp2p_noise.New),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// support QUIC - experimental
		libp2p.Transport(libp2p_quic.NewTransport),
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(libp2p_conn.NewConnManager(
			100,         // Lowwater
			200,         // HighWater,
			time.Minute, // GracePeriod
		)),
		// Attempt to open ports using uPNP for NATed hosts.
		libp2p.NATPortMap(),
		// Let this host use the DHT to find other hosts
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			var e error
			globalDHT, e = libp2p_dht.New(globalContext, h)
			return globalDHT, e
		}),
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		libp2p.EnableAutoRelay(),
		// If you want to help other peers to figure out if they are behind
		// NATs, you can launch the server-side of AutoNAT too (AutoRelay
		// already runs the client)
		//
		// This service is highly rate-limited and should not cause any
		// performance issues.
		libp2p.EnableNATService(),
	)
	if e != nil {
		return fmt.Errorf("创建主机出错: %w", e)
	}
	defer globalHost.Close()

	// 初始化MDNS
	initMDNS(globalHost, mdnsStopChan)

	// 告知节点启动
	myAddrBytes, e := json.Marshal(globalHost.Addrs())
	if e != nil {
		return fmt.Errorf("我的地址转换出错: %w", e)
	}
	globalCallback.OnOpStart(globalHost.ID().Pretty(), string(myAddrBytes))
	log.Println("开放点对点已经启动", globalHost.ID().Pretty(), string(myAddrBytes))

	// 保持运行
	<-globalContext.Done()

	// 告知节点停止
	globalCallback.OnOpStop()

	log.Println("开放点对点已经停止")

	return nil
}

func Stop() {
	log.Println("停止开放点对点")

	mdnsStopChan <- 1

	globalContextCancel()
}
