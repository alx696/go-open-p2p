package main

import (
	"flag"
	"go-open-p2p/op"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type CallbackImpl struct {
}

// 用以捕获启动错误
var startErrorChan = make(chan error, 1)

// 用以等待彻底关闭
var stopChan = make(chan int, 1)

func main() {
	privateFlag := flag.String("private", "", "private dir")
	publicFlag := flag.String("public", "", "public dir")
	nameFlag := flag.String("name", "", "my name")
	flag.Parse()

	if *privateFlag == "" || *publicFlag == "" || *nameFlag == "" {
		log.Fatalln("没有设置参数")
	}

	go func() {
		e := op.Start(*privateFlag, *publicFlag, CallbackImpl{})
		if e != nil {
			startErrorChan <- e
		}
	}()

	// 关注系统信号
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// 等待
	select {
	case s := <-signalChan:
		log.Println("收到系统信号", s.String())
		op.Stop()
	case e := <-startErrorChan:
		log.Println("启动出错", e)
		stopChan <- 1
	}

	<-stopChan
	log.Println("程序已经关闭")
}

func (impl CallbackImpl) OnOpStart(id string, addrArray string) {
	log.Println("回调启动", id, addrArray)

	go func() {
		<-time.After(time.Second * 6)

		//	log.Println("测试发送文本")
		//	op.TextSend("12D3KooWMVboev1gH614CnVwoG7mpN4te4QAJuk8c6Ykn7cSLh3o", "你好")
		//
		//	log.Println("测试发送文件")
		//	op.FileSend("12D3KooWMVboev1gH614CnVwoG7mpN4te4QAJuk8c6Ykn7cSLh3o", "/home/k/下载/libwebp-1.2.1-linux-x86-64.tar.gz")

		//var connStateIdArray []string
		//connStateIdArray = append(connStateIdArray, "12D3KooWS7mTsSGrngHP1YhNsiFZWYHRGpxgmfXoFcUnaU8KHbJt")
		//jsonBytes, _ := json.Marshal(connStateIdArray)
		//e := op.ConnStateCheckSet(string(jsonBytes))
		//if e != nil {
		//	log.Println("设置状态检查ID数组出错", e)
		//} else {
		//	log.Println("设置状态检查ID数组成功")
		//}
	}()
}

func (impl CallbackImpl) OnOpStop() {
	log.Println("回调停止")
	stopChan <- 1
}

func (impl CallbackImpl) OnOpState(jt string) {
	//log.Println("回调状态", jt)
}

func (impl CallbackImpl) OnOpMDNSPeer(id string) {
	log.Println("回调MDNS发现节点", id)
}

func (impl CallbackImpl) OnOpConnState(id string, isConn bool) {
	log.Println("回调节点连接状态变化", id, isConn)
}

func (impl CallbackImpl) OnOpText(jt string) {
	log.Println("回调收到对方发来文本", jt)
}

func (impl CallbackImpl) OnOpFileSendProgress(id, filePath string, percentage float64) {
	log.Println("回调文件发送进度", id, filePath, percentage)
}

func (impl CallbackImpl) OnOpFileReceiveError(id, filePath, et string) {
	log.Println("回调文件接收错误", id, filePath, et)
}

func (impl CallbackImpl) OnOpFileReceiveProgress(id, filePath string, percentage float64) {
	log.Println("回调文件接收进度", id, filePath, percentage)
}

func (impl CallbackImpl) OnOpFileReceiveDone(id, filePath string) {
	log.Println("回调文件接收完毕", id, filePath)
}
