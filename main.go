package main

import (
	"flag"
	"go-open-p2p/op"
	"log"
	"os"
	"os/signal"
	"syscall"
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
		e := op.Start(*privateFlag, *publicFlag, *nameFlag, CallbackImpl{})
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
}

func (impl CallbackImpl) OnOpStop() {
	log.Println("回调停止")
	stopChan <- 1
}
