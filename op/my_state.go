package op

import (
	"encoding/json"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
)

// 我的状态
type MyState struct {
	NodeCount int `json:"nodeCount"` // 节点数量
	ConnCount int `json:"connCount"` // 连接数量
}

func initState(h host.Host, stopChan chan int, cb Callback) {
	log.Println("启动状态")
	ticker := time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-stopChan:
				log.Println("停止状态")
				_ = ticker.Stop
				return
			case <-ticker.C:
				nodeCount := h.Peerstore().Peers().Len()
				connCount := len(h.Network().Conns())
				jsonBytes, _ := json.Marshal(MyState{NodeCount: nodeCount, ConnCount: connCount})
				cb.OnOpState(string(jsonBytes))
			}
		}
	}()
}
