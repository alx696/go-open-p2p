// 参考 https://github.com/libp2p/go-libp2p/blob/master/examples/chat-with-mdns/mdns.go
package op

import (
	"context"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

type discoveryNotifee struct {
	PeerChan chan peer.AddrInfo
}

// interface to be called when new  peer is found
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.PeerChan <- pi
}

func mdnsInit(gc context.Context, h host.Host, stopChan chan int, cb Callback) {
	log.Println("启动MDNS")
	n := &discoveryNotifee{PeerChan: make(chan peer.AddrInfo)}
	s := mdns.NewMdnsService(h, "lilu-open-p2p", n)
	e := s.Start()
	if e != nil {
		log.Panicln(e)
	}

	go func() {
		for {
			select {
			case <-stopChan:
				log.Println("停止MDNS")
				_ = s.Close()
				return
			case addr := <-n.PeerChan:
				// 忽略自己
				if addr.ID.Pretty() == h.ID().Pretty() {
					break
				}

				log.Println("MDNS发现节点", addr.ID.Pretty())
				go func() {
					e := connectPeer(gc, h, addr, time.Second)
					if e != nil {
						//log.Println("MDNS节点连接失败", addr.ID.Pretty(), e)
						return
					}
					//log.Println("MDNS节点连接成功", addr.ID.Pretty())

					cb.OnOpMDNSPeer(addr.ID.Pretty())
				}()
			}
		}
	}()
}
