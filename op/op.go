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
	"go-open-p2p/dns"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Callback interface {
	OnOpStart(id string, addrArray string)
	OnOpStop()
	// OnOpState 节点状态变化, my_state.MyState
	OnOpState(jt string)
	// OnOpMDNSPeer MDNS发现节点
	OnOpMDNSPeer(id string)
	// OnOpConnState 节点连接状态变化
	OnOpConnState(id string, isConn bool)
	// OnOpText 收到对方发来文本
	OnOpText(jt string)
	// OnOpFileSendProgress 文件发送进度
	OnOpFileSendProgress(id, filePath string, percentage float64)
	// OnOpFileReceiveError 文件接收错误
	OnOpFileReceiveError(id, filePath, et string)
	// OnOpFileReceiveProgress 文件接收进度
	OnOpFileReceiveProgress(id, filePath string, percentage float64)
	// OnOpFileReceiveDone 文件接收完毕
	OnOpFileReceiveDone(id, filePath string)
}

const (
	// 连接保护标记:保持,权重100.
	connProtectTag = "keep-conn"
	// 协议：文本
	protocolText = "/lilu.red/op/1/text"
	// 协议：文件
	protocolFile = "/lilu.red/op/1/file"
)

var globalCallback Callback
var globalContext context.Context
var globalContextCancel context.CancelFunc
var globalHost host.Host
var globalDHT *libp2p_dht.IpfsDHT
var globalPublicDirectory string
var mdnsStopChan = make(chan int, 1)
var stateStopChan = make(chan int, 1)
var connStateStopChan = make(chan int, 1)

// Start 启动
//
// privateDirArg 私有文件夹绝对路径, 用于存放密钥等私密内容
//
// publicDirArg 公共文件夹绝对路径, 用于存放接收文件等公开内容
//
// nameArg 我的名称, 用于对方分辨自己
//
// callbackArg 回调, 用于传递异步状态数据
func Start(privateDirArg string, publicDirArg string, callbackArg Callback) error {
	log.Println("启动开放点对点")
	log.Println("私有文件夹", privateDirArg)
	log.Println("公共文件夹", publicDirArg)
	globalPublicDirectory = publicDirArg
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
	mdnsInit(globalContext, globalHost, mdnsStopChan, globalCallback)

	// 连接引导
	var dnsTxtArray []string
	dnsTxtArray, e = dns.MaDNS("/dnsaddr/bootstrap.libp2p.io")
	if e == nil {
		for _, v := range dnsTxtArray {
			go connectBootstrap(globalContext, globalHost, v)
		}
	}
	dnsTxtArray, e = dns.Txt("bootstrap.ipfs.lilu.red")
	if e == nil {
		for _, v := range dnsTxtArray {
			go connectBootstrap(globalContext, globalHost, v)
		}
	}

	// 初始化交换
	initExchange(globalHost)

	// 告知节点启动
	myAddrBytes, e := json.Marshal(globalHost.Addrs())
	if e != nil {
		return fmt.Errorf("我的地址转换出错: %w", e)
	}
	log.Println("开放点对点已经启动", globalHost.ID().Pretty(), string(myAddrBytes))
	globalCallback.OnOpStart(globalHost.ID().Pretty(), string(myAddrBytes))

	// 初始化状态
	initState(globalHost, stateStopChan, globalCallback)

	// 初始化连接状态
	connStateInit(globalContext, globalHost, connStateStopChan, globalCallback)

	// 保持运行
	<-globalContext.Done()

	// 告知节点停止
	globalCallback.OnOpStop()

	log.Println("开放点对点已经停止")

	return nil
}

// Stop 停止
func Stop() {
	log.Println("停止开放点对点")

	mdnsStopChan <- 1
	stateStopChan <- 1
	connStateStopChan <- 1

	globalContextCancel()
}
