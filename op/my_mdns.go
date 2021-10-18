package op

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"log"
	"time"
)

type DnsNotifee struct {
	PeerChan chan peer.AddrInfo
}

// 实现mdns.Notifee
func (n *DnsNotifee) HandlePeerFound(pa peer.AddrInfo) {
	n.PeerChan <- pa
}

func connMDNS(gc context.Context, h host.Host, addr peer.AddrInfo) {
	ctx, can := context.WithTimeout(gc, time.Second)
	defer can()
	e := h.Connect(ctx, addr)
	if e != nil {
		log.Println("MDNS节点连接失败", addr.ID.Pretty(), e)
		return
	}
	log.Println("MDNS节点连接成功", addr.ID.Pretty())
}

func initMDNS(gc context.Context, h host.Host, stopChan chan int) {
	dsnNotifee := &DnsNotifee{PeerChan: make(chan peer.AddrInfo)}
	s := mdns.NewMdnsService(h, "")
	s.RegisterNotifee(dsnNotifee)

	go func() {
		for {
			select {
			case <-stopChan:
				log.Println("停止MDNS")
				_ = s.Close()
				return
			case addr := <-dsnNotifee.PeerChan:
				// 忽略自己
				if addr.ID.Pretty() == h.ID().Pretty() {
					break
				}

				log.Println("MDNS发现节点", addr.ID.Pretty())
				go connMDNS(gc, h, addr)
			}
		}
	}()
}
