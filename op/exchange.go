package op

import (
	"bufio"
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"log"
)

func initExchange(gc context.Context, h host.Host) {
	h.SetStreamHandler(protocolInfo, infoStreamHandler)
}

// 处理信息请求
func infoStreamHandler(s network.Stream) {
	remotePeerID := s.Conn().RemotePeer()
	log.Println("处理信息, 对方ID:", remotePeerID)
	defer s.Close()

	// 创建读写器
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	// 读取
	requestBytes, e := readTextFromReadWriter(rw)
	if e != nil {
		log.Println("处理信息, 读取对方发来内容出错:", e)
		return
	}
	requestText := string(*requestBytes)
	log.Println("处理信息, 对方发来内容:", requestText)

	// 通知收到信息
	globalCallback.OnOpInfo(requestText)

	// 回复1
	responseBytes := []byte("1")
	e = writeTextToReadWriter(rw, &responseBytes)
	if e != nil {
		log.Println("处理信息, 回复对方1时出错:", e)
	}
}
