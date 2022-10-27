package op

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

var connStateMutex sync.RWMutex

// 不要使用! 通过connStateIdArraySet()进行设置
var connStateIdArray []string

func connStateInit(gc context.Context, h host.Host, stopChan chan int, cb Callback) {
	log.Println("启动连接状态")
	ticker := time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-stopChan:
				log.Println("停止连接状态")
				_ = ticker.Stop
				return
			case <-ticker.C:
				connStateMutex.RLock()
				for _, id := range connStateIdArray {
					go connStateCheck(gc, h, id, cb)
				}
				connStateMutex.RUnlock()
			}
		}
	}()
}

func connStateCheck(gc context.Context, h host.Host, id string, cb Callback) {
	peerID, e := peer.Decode(id)
	if e != nil {
		log.Println("连接状态检查时解析节点标识出错", e)
		return
	}

	connCount := connectCount(h, peerID)
	if connCount > 0 {
		// 通知正在连接
		cb.OnOpConnState(id, true)
		return
	}

	// 尝试连接
	addr, e := findAddrInfoFromDHT(gc, peerID)
	if e != nil {
		//log.Println("连接状态检查时获取连接地址出错", e)
		// 通知连接断开
		cb.OnOpConnState(id, false)
		return
	}
	e = connectPeer(gc, h, *addr, time.Second)
	if e != nil {
		//log.Println("连接状态检查时尝试进行连接失败", e)
		// 通知连接断开
		cb.OnOpConnState(id, false)
		return
	}
	// 通知正在连接
	cb.OnOpConnState(id, true)
}

func connStateIdArraySet(array []string) {
	log.Println("设置状态检查标识数组", array)
	connStateMutex.Lock()
	connStateIdArray = array
	connStateMutex.Unlock()
}
